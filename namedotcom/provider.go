package namedotcom

import (
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/namedotcom/go/namecom"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"token": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("TOKEN", nil),
				Description: "Name.com API Token Value",
			},
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("USERNAME", nil),
				Description: "Name.com API Username",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"namedotcom_record": resourceRecord(),
			// "namedotcom_domain":             resourceDomain(),
			"namedotcom_domain_nameservers": resourceDomainNameServers(),
			// "namedotcom_domain_autorenew":  resourceDomainAutoRenew(),
			// "namedotcom_domain_contact":    resourceDomainContact(),
			// "namedotcom_domain_lock":       resourceDomainLock(),
			// "namedotcom_dnssec":            resourceDnssec(),
			// "namedotcom_email_forwarding":  resourceEmailForwarding(),
			// "namedotcom_transfer":          resourceTransfer(),
			// "namedotcom_url_forwarding":    resourceUrlForwarding(),
			// "namedotcom_vanity_nameserver": resourceVanityNameserver(),
		},
		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	nc := namecom.New(d.Get("username").(string), d.Get("token").(string))
	return nc, nil
}
