package namedotcom

import (
	"github.com/cockroachdb/errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/namedotcom/go/v4/namecom"
)

//nolint:lll // Line length is acceptable here
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("NAMEDOTCOM_USERNAME", nil),
				Description: "Name.com API Username; can alternatively be specified via `NAMEDOTCOM_USERNAME` environment variable.",
			},
			"token": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("NAMEDOTCOM_TOKEN", nil),
				Description: "Name.com API Token Value; can alternatively be specified via `NAMEDOTCOM_TOKEN` environment variable.",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"namedotcom_record": resourceRecord(),
			// "namedotcom_domain":             resourceDomain(),
			"namedotcom_domain_nameservers": resourceDomainNameServers(),
			// "namedotcom_domain_autorenew":  resourceDomainAutoRenew(),
			// "namedotcom_domain_contact":    resourceDomainContact(),
			// "namedotcom_domain_lock":       resourceDomainLock(),
			"namedotcom_dnssec": resourceDNSSEC(),
			// "namedotcom_email_forwarding":  resourceEmailForwarding(),
			// "namedotcom_transfer":          resourceTransfer(),
			// "namedotcom_url_forwarding":    resourceUrlForwarding(),
			// "namedotcom_vanity_nameserver": resourceVanityNameserver(),
		},
		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(data *schema.ResourceData) (interface{}, error) {
	// Check for required fields
	token, ok := data.Get("token").(string)
	if !ok {
		return nil, errors.New("token is required")
	}

	username, ok := data.Get("username").(string)
	if !ok {
		return nil, errors.New("username is required")
	}

	if token == "" || username == "" {
		return nil, errors.New("Token and Username are required")
	}

	// Create a new Name.com client
	nc := namecom.New(username, token)

	return nc, nil
}
