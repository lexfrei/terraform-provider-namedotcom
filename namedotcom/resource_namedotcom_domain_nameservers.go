package namedotcom

import (
	"github.com/cockroachdb/errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
				Type:     schema.TypeList,
				Optional: true,
				//nolint:lll //Description can be long
				Description: "Nameservers is the list of nameservers for this domain. If unspecified it defaults to your account default nameservers.",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceDomainNameServersCreate(data *schema.ResourceData, m interface{}) error {
	rl.Wait(ctx)
	client, ok := m.(*namecom.NameCom)
	if !ok {
		return errors.New("Error converting interface to NameCom")
	}

	domainName, ok := data.Get("domain_name").(string)
	if !ok {
		return errors.New("Error converting domain_name to string")
	}

	// Make api request to setNameServers
	request := namecom.SetNameserversRequest{
		DomainName: domainName,
	}

	nameservers, ok := data.Get("nameservers").([]interface{})
	if !ok {
		return errors.New("Error converting nameservers to []interface{}")
	}

	for _, nameserver := range nameservers {
		nameserverString, ok := nameserver.(string)
		if !ok {
			return errors.New("Error converting nameserver to string")
		}

		request.Nameservers = append(request.Nameservers, nameserverString)
	}

	_, err := client.SetNameservers(&request)
	if err != nil {
		return errors.Wrap(err, "Error SetNameservers")
	}

	data.SetId(domainName)

	return nil
}

func resourceDomainNameServersRead(_ *schema.ResourceData, _ interface{}) error {
	return nil
}

func resourceDomainNameServersUpdate(_ *schema.ResourceData, _ interface{}) error {
	return nil
}

func resourceDomainNameServersDelete(data *schema.ResourceData, m interface{}) error {
	rl.Wait(ctx)
	client, ok := m.(*namecom.NameCom)
	if !ok {
		return errors.New("Error converting interface to NameCom")
	}

	domainName, ok := data.Get("domain_name").(string)
	if !ok {
		return errors.New("Error converting domain_name to string")
	}

	request := namecom.SetNameserversRequest{
		DomainName: domainName,
	}

	// Make api request to setNameServers
	_, err := client.SetNameservers(&request)
	if err != nil {
		return errors.Wrap(err, "Error SetNameservers")
	}

	// Record state using resourceDomainNameServersRead function
	data.SetId("")

	return nil
}
