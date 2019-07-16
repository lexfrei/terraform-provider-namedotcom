package namedotcom

import (
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/namedotcom/go/namecom"
)

func resourceDomainNameServers() *schema.Resource {
	return &schema.Resource{
		Create: resourceDomainNameServersCreate,
		// Read:   resourceDomainNameServersRead,
		// Update: resourceDomainNameServersUpdate,
		// Delete: resourceDomainNameServersDelete,

		Schema: map[string]*schema.Schema{
			"domainName": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "DomainName is the punycode encoded value of the domain name.",
			},
			"nameservers": &schema.Schema{
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Nameservers is the list of nameservers for this domain. If unspecified it defaults to your account default nameservers.",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceDomainNameServersCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*namecom.NameCom)
	// Make api request to setNameServers
	// Record state using resourceDomainNameServersRead function
	request := namecom.SetNameserversRequest{
		DomainName:  d.Get("domain_name").(string),
		Nameservers: d.Get("nameservers").([]string),
	}
	client.SetNameservers(&request)
	// return resourceDomainNameServersRead(d, m)
	return nil
}

// func resourceDomainNameServersRead(d *schema.ResourceData, m interface{}) error {
// 	client := m.(*namecom.NameCom)
// 	return nil
// }
//
// func resourceDomainNameServersUpdate(d *schema.ResourceData, m interface{}) error {
// 	client := m.(*namecom.NameCom)
// 	return resourceDomainNameServersRead(d, m)
// }
//
// func resourceDomainNameServersDelete(d *schema.ResourceData, m interface{}) error {
// 	client := m.(*namecom.NameCom)
//
// 	d.SetId("")
// 	return nil
// }
