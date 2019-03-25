package vcd

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/vmware/go-vcloud-director/v2/govcd"
	"github.com/vmware/go-vcloud-director/v2/types/v56"
)

func resourceVcdVappNetworkNat() *schema.Resource {
	return &schema.Resource{
		Create: resourceVcdVappNetworkNATCreate,
		Delete: resourceVcdVappNetworkNATDelete,
		Read:   resourceVcdVappNetworkNATRead,

		Schema: map[string]*schema.Schema{
			"vapp_name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"vapp_network_name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"org": {
				Type:     schema.TypeString,
				Required: false,
				Optional: true,
				ForceNew: true,
			},
			"vdc": {
				Type:     schema.TypeString,
				Required: false,
				Optional: true,
				ForceNew: true,
			},
			"nat_type": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"rule": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"external_ip_address": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"mapping_mode": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"vm_name": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"vm_nic_id": &schema.Schema{
							Type:     schema.TypeInt,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func resourceVcdVappNetworkNATCreate(d *schema.ResourceData, meta interface{}) error {
	vcdClient := meta.(*VCDClient)
	vcdClient.Mutex.Lock()
	defer vcdClient.Mutex.Unlock()
	_, vdc, err := vcdClient.GetOrgAndVdcFromResource(d)
	if err != nil {
		return fmt.Errorf(errorRetrievingOrgAndVdc, err)
	}

	var vapp govcd.VApp

	for i := 0; i <= 60; i += 1 {
		vapp, err = vdc.FindVAppByName(d.Get("vapp_name").(string))
		if err != nil {
			return fmt.Errorf("error finding vApp. %#v", err)
		}

		fmt.Println(vapp.VApp.Name)

		networkConfig, err := vapp.GetNetworkConfig()
		if err != nil {
			return fmt.Errorf("error getting network config section: %#v", err)
		}

		if len(networkConfig.NetworkConfig) >= 1 {
			break
		}

		time.Sleep(3000 * time.Millisecond)
	}

	networkConfig, err := vapp.GetNetworkConfig()
	if err != nil {
		return fmt.Errorf("error getting network config section: %#v", err)
	}
	if len(networkConfig.NetworkConfig) < 1 {
		return fmt.Errorf("networkConfig is not set on create")
	}
	if len(networkConfig.NetworkConfig) > 1 {
		return fmt.Errorf("multipe networkConfigs are set")
	}

	if networkConfig.NetworkConfig[0].Configuration == nil {
		return fmt.Errorf("networkConfig Configuration is not set")
	}

	if networkConfig.NetworkConfig[0].Configuration.Features == nil {
		return fmt.Errorf("networkConfig Configuration Features is not set")
	}

	if networkConfig.NetworkConfig[0].Configuration.Features.NatService == nil {
		return fmt.Errorf("networkConfig Configuration Features NatService is not set")
	}

	natService := networkConfig.NetworkConfig[0].Configuration.Features.NatService

	natRules := make([]*types.NatRule, 0, 1)
	rulesCount := d.Get("rule.#").(int)
	for i := 0; i < rulesCount; i++ {
		prefix := fmt.Sprintf("rule.%d", i)

		vmName := d.Get(prefix + ".vm_name").(string)
		targetVm, err := vdc.FindVMByName(vapp, vmName)
		if err != nil {
			return fmt.Errorf("could not find vm by name %#v", err)
		}

		rule := &types.NatOneToOneVMRule{
			MappingMode:       d.Get(prefix + ".mapping_mode").(string),
			ExternalIPAddress: d.Get(prefix + ".external_ip_address").(string),
			VAppScopedVMID:    targetVm.VM.VAppScopedLocalID,
			VMNicID:           d.Get(prefix + ".vm_nic_id").(int),
		}

		natRule := &types.NatRule{
			OneToOneVMRule: rule,
		}
		natRules = append(natRules, natRule)
	}

	fmt.Println("created the natrules array")
	natService.NatRule = natRules
	vapp.AddVAppNetworkNatRules(d.Get("vapp_network_name").(string), natService)

	return nil
}

func resourceVcdVappNetworkNATRead(d *schema.ResourceData, meta interface{}) error {
	vcdClient := meta.(*VCDClient)

	_, vdc, err := vcdClient.GetOrgAndVdcFromResource(d)
	if err != nil {
		return fmt.Errorf(errorRetrievingOrgAndVdc, err)
	}

	vapp, err := vdc.FindVAppByName(d.Get("vapp_name").(string))
	if err != nil {
		return fmt.Errorf("error finding Vapp: %#v", err)
	}
	fmt.Println(vapp.VApp.Name)

	networkConfig, err := vapp.GetNetworkConfig()
	if err != nil {
		return fmt.Errorf("error getting network config section: %#v", err)
	}

	if len(networkConfig.NetworkConfig) < 1 {
		return nil
	}

	if len(networkConfig.NetworkConfig) > 1 {
		return fmt.Errorf("multipe networkConfigs are set")
	}

	if networkConfig.NetworkConfig[0].Configuration == nil {
		return fmt.Errorf("networkConfig Configuration is not set")
	}

	if networkConfig.NetworkConfig[0].Configuration.Features == nil {
		return fmt.Errorf("networkConfig Configuration Features is not set")
	}

	if networkConfig.NetworkConfig[0].Configuration.Features.NatService == nil {
		return fmt.Errorf("networkConfig Configuration Features NatService is not set")
	}

	natService := networkConfig.NetworkConfig[0].Configuration.Features.NatService
	d.Set("nat_type", natService.NatType)

	ruleList := d.Get("rule").([]interface{})
	rulesCount := d.Get("rule.#").(int)
	for i := 0; i < rulesCount; i++ {
		prefix := fmt.Sprintf("rule.%d", i)
		if d.Get(prefix+".id").(string) == "" {
			log.Printf("[INFO] Rule %d has no id. Searching...", i)
			ruleid, err := matchNatRule(d, prefix, natService.NatRule)
			if err == nil {
				currentRule := ruleList[i].(map[string]interface{})
				currentRule["id"] = ruleid
				ruleList[i] = currentRule
			}
		}
	}
	return nil
}

func matchNatRule(d *schema.ResourceData, prefix string, rules []*types.NatRule) (string, error) {

	for _, r := range rules {
		or := r.OneToOneVMRule
		if strings.ToLower(d.Get(prefix+".external_ip_address").(string)) == or.ExternalIPAddress &&
			strings.ToLower(d.Get(prefix+".external_ip_address").(string)) == or.ExternalIPAddress &&
			strings.ToLower(d.Get(prefix+".external_ip_address").(string)) == or.ExternalIPAddress {
			return r.ID, nil
		}
	}
	return "", fmt.Errorf("Unable to find rule")
}

func resourceVcdVappNetworkNATDelete(d *schema.ResourceData, meta interface{}) error {
	vcdClient := meta.(*VCDClient)

	// Multiple VCD components need to run operations on the Edge Gateway, as
	// the edge gatway will throw back an error if it is already performing an
	// operation we must wait until we can aquire a lock on the client
	vcdClient.Mutex.Lock()
	defer vcdClient.Mutex.Unlock()
	return nil
}
