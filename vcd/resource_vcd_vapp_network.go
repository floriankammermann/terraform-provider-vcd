package vcd

import (
	"fmt"
	"log"

	"github.com/vmware/go-vcloud-director/v2/govcd"
	"github.com/vmware/go-vcloud-director/v2/types/v56"

	"github.com/hashicorp/terraform/helper/schema"
)

func resourceVcdVappNetwork() *schema.Resource {
	return &schema.Resource{
		Create: resourceVcdVappNetworkCreate,
		Read:   resourceVappNetworkRead,
		Delete: resourceVappNetworkDelete,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"vapp_name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"parent_network": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"org": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"vdc": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"netmask": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "255.255.255.0",
			},
			"gateway": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"dns1": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "8.8.8.8",
			},

			"dns2": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "8.8.4.4",
			},

			"dns_suffix": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"guest_vlan_allowed": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
				Default:  false,
			},

			"dhcp_pool": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"start_address": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},

						"end_address": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
						},

						"default_lease_time": &schema.Schema{
							Type:     schema.TypeInt,
							Default:  3600,
							Optional: true,
						},

						"max_lease_time": &schema.Schema{
							Type:     schema.TypeInt,
							Default:  7200,
							Optional: true,
						},

						"enabled": &schema.Schema{
							Type:     schema.TypeBool,
							Default:  true,
							Optional: true,
						},
					},
				},
				Set: resourceVcdNetworkIPAddressHash,
			},
			"static_ip_pool": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"start_address": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},

						"end_address": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
				Set: resourceVcdNetworkIPAddressHash,
			},
			"fence_mode": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "isolated",
			},
			"firewall_rule": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"description": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},

						"policy": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},

						"protocol": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},

						"destination_port": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},

						"destination_ip": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},

						"source_port": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},

						"source_ip": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func resourceVcdVappNetworkCreate(d *schema.ResourceData, meta interface{}) error {
	vcdClient := meta.(*VCDClient)
	vcdClient.Mutex.Lock()
	defer vcdClient.Mutex.Unlock()

	_, vdc, err := vcdClient.GetOrgAndVdcFromResource(d)
	if err != nil {
		return fmt.Errorf(errorRetrievingOrgAndVdc, err)
	}

	vapp, err := vdc.FindVAppByName(d.Get("vapp_name").(string))
	if err != nil {
		return fmt.Errorf("error finding vApp. %#v", err)
	}

	vdcOrgNetwork, err := vdc.FindVDCNetwork(d.Get("parent_network").(string))
	if err != nil {
		return fmt.Errorf("error finding orgNetwork. %#v", err)
	}

	staticIpRanges := expandIPRange(d.Get("static_ip_pool").(*schema.Set).List())

	parentNetwork := &types.Reference{
		HREF: vdcOrgNetwork.OrgVDCNetwork.HREF,
		ID:   vdcOrgNetwork.OrgVDCNetwork.ID,
		Type: vdcOrgNetwork.OrgVDCNetwork.Type,
		Name: vdcOrgNetwork.OrgVDCNetwork.Name,
	}

	vappNetworkSettings := &govcd.VappNetworkSettings{
		Name:             d.Get("name").(string),
		Gateway:          d.Get("gateway").(string),
		NetMask:          d.Get("netmask").(string),
		DNS1:             d.Get("dns1").(string),
		DNS2:             d.Get("dns2").(string),
		DNSSuffix:        d.Get("dns_suffix").(string),
		ParentNetwork:    parentNetwork,
		FenceMode:        d.Get("fence_mode").(string),
		StaticIPRanges:   staticIpRanges.IPRange,
		GuestVLANAllowed: d.Get("guest_vlan_allowed").(bool),
	}

	if dhcp, ok := d.GetOk("dhcp_pool"); ok && len(dhcp.(*schema.Set).List()) > 0 {
		for _, item := range dhcp.(*schema.Set).List() {
			data := item.(map[string]interface{})
			vappNetworkSettings.DhcpSettings = &govcd.DhcpSettings{
				IsEnabled:        data["enabled"].(bool),
				DefaultLeaseTime: data["default_lease_time"].(int),
				MaxLeaseTime:     data["max_lease_time"].(int),
				IPRange: &types.IPRange{StartAddress: data["start_address"].(string),
					EndAddress: data["end_address"].(string)}}
		}
	}

	if firewallrules, ok := d.GetOk("firewall_rule"); ok && len(firewallrules.([]interface{})) > 0 {
		vappNetworkSettings.FirewallRules = []*types.FirewallRule{}
		for _, item := range firewallrules.([]interface{}) {
			data := item.(map[string]interface{})
			firewallrule := &types.FirewallRule{
				Description: data["description"].(string),
				Policy:      data["policy"].(string),
				Protocols: &types.FirewallRuleProtocols{
					Any: true,
				},
				DestinationPortRange: data["destination_port"].(string),
				DestinationIP:        data["destination_ip"].(string),
				SourcePortRange:      data["source_port"].(string),
				SourceIP:             data["source_ip"].(string),
			}
			vappNetworkSettings.FirewallRules = append(vappNetworkSettings.FirewallRules, firewallrule)
		}
	}

	task, err := vapp.AddVAppNetwork(vappNetworkSettings)

	if err != nil {
		return fmt.Errorf("error creating vApp network. %#v", err)
	}

	err = task.WaitTaskCompletion()
	if err != nil {
		return fmt.Errorf("error waiting for task to complete: %+v", err)
	}

	d.SetId(d.Get("name").(string))

	return resourceVappNetworkRead(d, meta)
}

func resourceVappNetworkRead(d *schema.ResourceData, meta interface{}) error {
	vcdClient := meta.(*VCDClient)

	_, vdc, err := vcdClient.GetOrgAndVdcFromResource(d)
	if err != nil {
		return fmt.Errorf(errorRetrievingOrgAndVdc, err)
	}

	vapp, err := vdc.FindVAppByName(d.Get("vapp_name").(string))
	if err != nil {
		return fmt.Errorf("error finding Vapp: %#v", err)
	}

	vAppNetworkConfig, err := vapp.GetNetworkConfig()
	if err != nil {
		return fmt.Errorf("error getting vApp networks: %#v", err)
	}

	vAppNetwork := types.VAppNetworkConfiguration{}
	for _, networkConfig := range vAppNetworkConfig.NetworkConfig {
		if networkConfig.NetworkName == d.Get("name").(string) {
			vAppNetwork = networkConfig
		}
	}

	if vAppNetwork == (types.VAppNetworkConfiguration{}) {
		log.Printf("[DEBUG] Network no longer exists. Removing from tfstate")
		d.SetId("")
		return nil
	}

	d.Set("name", vAppNetwork.NetworkName)
	d.Set("href", vAppNetwork.HREF)
	if c := vAppNetwork.Configuration; c != nil {
		d.Set("fence_mode", c.FenceMode)
		if c.IPScopes != nil {
			d.Set("gateway", c.IPScopes.IPScope.Gateway)
			d.Set("netmask", c.IPScopes.IPScope.Netmask)
			d.Set("dns1", c.IPScopes.IPScope.DNS1)
			d.Set("dns2", c.IPScopes.IPScope.DNS2)
			d.Set("dnsSuffix", c.IPScopes.IPScope.DNSSuffix)
		}
		d.Set("guestVlanAllowed", c.GuestVlanAllowed)
	}

	return nil
}

func resourceVappNetworkDelete(d *schema.ResourceData, meta interface{}) error {
	vcdClient := meta.(*VCDClient)
	vcdClient.Mutex.Lock()
	defer vcdClient.Mutex.Unlock()

	_, vdc, err := vcdClient.GetOrgAndVdcFromResource(d)
	if err != nil {
		return fmt.Errorf(errorRetrievingOrgAndVdc, err)
	}

	vapp, err := vdc.FindVAppByName(d.Get("vapp_name").(string))
	if err != nil {
		return fmt.Errorf("error finding vApp: %#v", err)
	}

	task, err := vapp.RemoveIsolatedNetwork(d.Get("name").(string))
	if err != nil {
		return fmt.Errorf("error removing vApp network: %#v", err)
	}

	err = task.WaitTaskCompletion()
	if err != nil {
		return fmt.Errorf("error waiting for task to complete: %+v", err)
	}

	d.SetId("")

	return nil
}
