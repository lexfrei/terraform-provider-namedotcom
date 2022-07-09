package namedotcom

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/namedotcom/go/v4/namecom"
)

func resourceDomainNameServers() *schema.Resource {
	return &schema.Resource{
		Create: resourceDomainNameServersCreate,
		Read:   resourceDomainNameServersRead,
		Update: resourceDomainNameServersUpdate,
		Delete: resourceDomainNameServersDelete,

		Schema: map[string]*schema.Schema{
			"domain_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "DomainName is the punycode encoded value of the domain name.",
			},
			"nameservers": {
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

	domain_name := d.Get("domain_name").(string)

	// Make api request to setNameServers
	request := namecom.SetNameserversRequest{
		DomainName: domain_name,
	}
	for _, nameserver := range d.Get("nameservers").([]interface{}) {
		request.Nameservers = append(request.Nameservers, nameserver.(string))
	}
	client.SetNameservers(&request)

	// // Record state using resourceDomainNameServersRead function
	// if err := resourceDomainNameServersRead(d, meta); err != nil {
	// 	return err
	// }

	d.SetId(domain_name)
	return nil
}

func resourceDomainNameServersRead(d *schema.ResourceData, m interface{}) error {
	// client := m.(*namecom.NameCom)
	return nil
}

func resourceDomainNameServersUpdate(d *schema.ResourceData, m interface{}) error {
	// client := m.(*namecom.NameCom)
	// return resourceDomainNameServersRead(d, m)
	return nil
}

func resourceDomainNameServersDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*namecom.NameCom)

	domain_name := d.Get("domain_name").(string)
	request := namecom.SetNameserversRequest{
		DomainName: domain_name,
	}
	// Make api request to setNameServers
	client.SetNameservers(&request)

	// Record state using resourceDomainNameServersRead function
	d.SetId("")
	return nil
}
