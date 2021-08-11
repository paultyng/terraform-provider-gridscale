package relationmanager

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	fwu "github.com/terraform-providers/terraform-provider-gridscale/gridscale/firewall-utils"

	"github.com/gridscale/gsclient-go/v3"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	errHandler "github.com/terraform-providers/terraform-provider-gridscale/gridscale/error-handler"
)

//firewallRuleTypes defines all types of firewall rules
var firewallRuleTypes = []string{"rules_v4_in", "rules_v4_out", "rules_v6_in", "rules_v6_out"}

//ServerRelationManger is an wrapper of gsclient which is used for
//managing relations of a server in gridscale terraform provider
type ServerRelationManger struct {
	gsc  *gsclient.Client
	data *schema.ResourceData
}

//NewServerRelationManger creates a new instance ServerRelationManger
func NewServerRelationManger(gsc *gsclient.Client, d *schema.ResourceData) *ServerRelationManger {
	return &ServerRelationManger{gsc, d}
}

//getGSClient returns gsclient from server relation manager
func (c ServerRelationManger) getGSClient() *gsclient.Client {
	return c.gsc
}

//getData returns resource data from server relation manager
func (c ServerRelationManger) getData() *schema.ResourceData {
	return c.data
}

//LinkStorages links storages to a server
//**Note: The first storage in the list will be automatically set as the boot device
func (c *ServerRelationManger) LinkStorages(ctx context.Context) error {
	d := c.getData()
	client := c.getGSClient()
	if attr, ok := d.GetOk("storage"); ok {
		for _, value := range attr.([]interface{}) {
			storage := value.(map[string]interface{})
			err := client.LinkStorage(ctx, d.Id(), storage["object_uuid"].(string), false)
			if err != nil {
				return fmt.Errorf(
					"Error waiting for storage (%s) to be attached to server (%s): %s",
					storage["object_uuid"].(string),
					d.Id(),
					err,
				)
			}
		}
	}
	return nil
}

//LinkIPv4 links IPv4 address to a server
func (c *ServerRelationManger) LinkIPv4(ctx context.Context) error {
	d := c.getData()
	client := c.getGSClient()
	if attr, ok := d.GetOk("ipv4"); ok {
		//Check IP version
		if client.GetIPVersion(ctx, attr.(string)) != 4 {
			return fmt.Errorf("The IP address with UUID %v is not version 4", attr.(string))
		}
		err := client.LinkIP(ctx, d.Id(), attr.(string))
		if err != nil {
			return fmt.Errorf(
				"Error waiting for IP address (%s) to be attached to server (%s): %s",
				attr.(string),
				d.Id(),
				err,
			)
		}
	}
	return nil
}

//LinkIPv6 link an IPv6 address to a server
func (c *ServerRelationManger) LinkIPv6(ctx context.Context) error {
	d := c.getData()
	client := c.getGSClient()
	if attr, ok := d.GetOk("ipv6"); ok {
		//Check IP version
		if client.GetIPVersion(ctx, attr.(string)) != 6 {
			return fmt.Errorf("The IP address with UUID %v is not version 6", attr.(string))
		}
		err := client.LinkIP(ctx, d.Id(), attr.(string))
		if err != nil {
			return fmt.Errorf(
				"Error waiting for IP address (%s) to be attached to server (%s): %s",
				attr.(string),
				d.Id(),
				err,
			)
		}
	}
	return nil
}

//LinkISOImage links an ISO image to a server
func (c *ServerRelationManger) LinkISOImage(ctx context.Context) error {
	d := c.getData()
	client := c.getGSClient()
	if attr, ok := d.GetOk("isoimage"); ok {
		err := client.LinkIsoImage(ctx, d.Id(), attr.(string))
		if err != nil {
			return fmt.Errorf(
				"Error waiting for iso-image (%s) to be attached to server (%s): %s",
				attr.(string),
				d.Id(),
				err,
			)
		}
	}
	return nil
}

//LinkNetworks links networks to server
func (c *ServerRelationManger) LinkNetworks(ctx context.Context) error {
	d := c.getData()
	client := c.getGSClient()
	if attrNetRel, ok := d.GetOk("network"); ok {
		for _, value := range attrNetRel.(*schema.Set).List() {
			// customFwRulesPtr is nil initially, that mean the fw is inactive
			var customFwRulesPtr *gsclient.FirewallRules
			network := value.(map[string]interface{})
			//Read custom firewall rules from `network` property (field)
			customFwRules := readCustomFirewallRules(network)
			// if customFwRules is not empty, customFwRulesPtr is not nil (fw is active)
			if !reflect.DeepEqual(customFwRules, gsclient.FirewallRules{}) {
				customFwRulesPtr = &customFwRules
			}
			err := client.LinkNetwork(
				ctx,
				d.Id(),
				network["object_uuid"].(string),
				network["firewall_template_uuid"].(string),
				network["bootdevice"].(bool),
				network["ordering"].(int),
				nil,
				customFwRulesPtr,
			)
			if err != nil {
				return fmt.Errorf(
					"Error waiting for network (%s) to be attached to server (%s): %s",
					network["object_uuid"],
					d.Id(),
					err,
				)
			}
		}
	}
	return nil
}

//readCustomFirewallRules reads custom firewall rules from a specific network
//returns `gsclient.FirewallRules` type variable
func readCustomFirewallRules(netData map[string]interface{}) gsclient.FirewallRules {
	//Init firewall rule variable
	var fwRules gsclient.FirewallRules

	//Loop through all firewall rule types
	//there are 4 types: "rules_v4_in", "rules_v4_out", "rules_v6_in", "rules_v6_out".
	for _, ruleType := range firewallRuleTypes {
		//Init array of firewall rules
		var rules []gsclient.FirewallRuleProperties
		//Check if the firewall rule type is declared in the current network
		if rulesInTypeAttr, ok := netData[ruleType]; ok {
			//Loop through all rules in the current firewall type
			for _, rulesInType := range rulesInTypeAttr.([]interface{}) {
				ruleProps := rulesInType.(map[string]interface{})
				ruleProperties := gsclient.FirewallRuleProperties{
					DstPort: ruleProps["dst_port"].(string),
					SrcPort: ruleProps["src_port"].(string),
					SrcCidr: ruleProps["src_cidr"].(string),
					Action:  ruleProps["action"].(string),
					Comment: ruleProps["comment"].(string),
					DstCidr: ruleProps["dst_cidr"].(string),
					Order:   ruleProps["order"].(int),
				}
				if ruleProps["protocol"].(string) == "tcp" {
					ruleProperties.Protocol = gsclient.TCPTransport
				} else if ruleProps["protocol"].(string) == "udp" {
					ruleProperties.Protocol = gsclient.UDPTransport
				}
				//Add rule to the array of rules
				rules = append(rules, ruleProperties)
			}
		}

		//Based on rule type to place the rules in the right property of fwRules variable
		if ruleType == "rules_v4_in" {
			fwRules.RulesV4In = fwu.AddDefaultFirewallInboundRules(rules, false) // add default rules
		} else if ruleType == "rules_v4_out" {
			fwRules.RulesV4Out = rules
		} else if ruleType == "rules_v6_in" {
			fwRules.RulesV6In = fwu.AddDefaultFirewallInboundRules(rules, true) // add default rules
		} else if ruleType == "rules_v6_out" {
			fwRules.RulesV6Out = rules
		}
	}
	return fwRules
}

//IsShutdownRequired checks if server is needed to be shutdown when updating
func (c *ServerRelationManger) IsShutdownRequired(ctx context.Context) bool {
	var shutdownRequired bool
	d := c.getData()
	//If the number of cores is decreased, shutdown the server
	if d.HasChange("cores") {
		old, new := d.GetChange("cores")
		if new.(int) < old.(int) || d.Get("legacy").(bool) { //Legacy systems don't support updating the memory while running
			shutdownRequired = true
		}
	}
	//If the amount of memory is decreased, shutdown the server
	if d.HasChange("memory") {
		old, new := d.GetChange("memory")
		if new.(int) < old.(int) || d.Get("legacy").(bool) { //Legacy systems don't support updating the memory while running
			shutdownRequired = true
		}
	}
	//If IP address, storages, or networks are changed, shutdown the server
	if d.HasChange("ipv4") || d.HasChange("ipv6") || d.HasChange("storage") || d.HasChange("network") {
		shutdownRequired = true
	}
	return shutdownRequired
}

//UpdateISOImageRel updates relationship between a server and an ISO image
func (c *ServerRelationManger) UpdateISOImageRel(ctx context.Context) error {
	d := c.getData()
	client := c.getGSClient()
	var err error
	//Check if ISO image field is changed
	if d.HasChange("isoimage") {
		oldIso, _ := d.GetChange("isoimage")
		//If there is an ISO image already linked to the server
		//Unlink it
		if oldIso != "" {
			//If 404 or 409, that means ISO image is already deleted => the relation between ISO image and server is deleted automatically
			err = errHandler.RemoveErrorContainsHTTPCodes(
				client.UnlinkIsoImage(ctx, d.Id(), oldIso.(string)),
				http.StatusConflict,
				http.StatusNotFound,
			)
			if err != nil {
				return err
			}
		}
		//Link new ISO image (if there is one)
		err = c.LinkISOImage(ctx)
	}
	return err
}

//UpdateIPv4Rel updates relationship between a server and an IPv4 address
func (c *ServerRelationManger) UpdateIPv4Rel(ctx context.Context) error {
	d := c.getData()
	client := c.getGSClient()
	var err error
	//If IPv4 field is changed
	if d.HasChange("ipv4") {
		oldIp, _ := d.GetChange("ipv4")
		//If there is an IPv4 address already linked to the server
		//Unlink it
		if oldIp != "" {
			//If 404 or 409, that means IP is already deleted => the relation between IP and server is deleted automatically
			err = errHandler.RemoveErrorContainsHTTPCodes(
				client.UnlinkIP(ctx, d.Id(), oldIp.(string)),
				http.StatusConflict,
				http.StatusNotFound,
			)
			if err != nil {
				return err
			}
		}
		//Link new IPv4 (if there is one)
		err = c.LinkIPv4(ctx)
	}
	return err
}

//UpdateIPv6Rel updates relationship between a server and an IPv6 address
func (c *ServerRelationManger) UpdateIPv6Rel(ctx context.Context) error {
	d := c.getData()
	client := c.getGSClient()
	var err error
	if d.HasChange("ipv6") {
		oldIp, _ := d.GetChange("ipv6")
		//If there is an IPv6 address already linked to the server
		//Unlink it
		if oldIp != "" {
			//If 404 or 409, that means IP is already deleted => the relation between IP and server is deleted automatically
			err = errHandler.RemoveErrorContainsHTTPCodes(
				client.UnlinkIP(ctx, d.Id(), oldIp.(string)),
				http.StatusConflict,
				http.StatusNotFound,
			)
			if err != nil {
				return err
			}
		}
		//Link new IPv6 (if there is one)
		err = c.LinkIPv6(ctx)
	}
	return err
}

//UpdateNetworksRel updates relationship between a server and networks
func (c *ServerRelationManger) UpdateNetworksRel(ctx context.Context) error {
	d := c.getData()
	client := c.getGSClient()
	var err error
	if d.HasChange("network") {
		oldNetworks, _ := d.GetChange("network")
		//Unlink all old networks if there are any networks linked to the server
		for _, value := range oldNetworks.(*schema.Set).List() {
			network := value.(map[string]interface{})
			if network["object_uuid"].(string) != "" {
				//If 404 or 409, that means network is already deleted => the relation between network and server is deleted automatically
				err = errHandler.RemoveErrorContainsHTTPCodes(
					client.UnlinkNetwork(ctx, d.Id(), network["object_uuid"].(string)),
					http.StatusConflict,
					http.StatusNotFound,
				)
				if err != nil {
					return err
				}
			}
		}
		//Links all new networks (if there are some)
		err = c.LinkNetworks(ctx)
	}
	return err
}

//UpdateStoragesRel updates relationship between a server and storages
func (c *ServerRelationManger) UpdateStoragesRel(ctx context.Context) error {
	d := c.getData()
	client := c.getGSClient()
	var err error
	if d.HasChange("storage") {
		oldStorages, _ := d.GetChange("storage")
		for _, value := range oldStorages.([]interface{}) {
			storage := value.(map[string]interface{})
			if storage["object_uuid"].(string) != "" {
				//If 404 or 409, that means storage is already deleted => the relation between storage and server is deleted automatically
				err = errHandler.RemoveErrorContainsHTTPCodes(
					client.UnlinkStorage(ctx, d.Id(), storage["object_uuid"].(string)),
					http.StatusConflict,
					http.StatusNotFound,
				)
				if err != nil {
					return err
				}
			}
		}
		//Links all new storages (if there are some)
		err = c.LinkStorages(ctx)
	}
	return err
}
