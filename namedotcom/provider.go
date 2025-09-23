package namedotcom

import (
	"time"

	"github.com/cockroachdb/errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/namedotcom/go/v4/namecom"
)

const (
	// Default rate limiting values.
	defaultRateLimitPerSecond = 20
	defaultRateLimitPerHour   = 3000
	defaultTimeoutSeconds     = 120
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
			"rate_limit_per_second": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     defaultRateLimitPerSecond,
				Description: "Maximum number of API requests per second. Defaults to 20.",
			},
			"rate_limit_per_hour": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     defaultRateLimitPerHour,
				Description: "Maximum number of API requests per hour. Defaults to 3000.",
			},
			"timeout": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     defaultTimeoutSeconds,
				Description: "Timeout in seconds for API requests. Defaults to 120 seconds.",
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

	// Get rate limits from configuration or use defaults
	perSecondLimit := defaultRateLimitPerSecond
	if v, ok := data.GetOk("rate_limit_per_second"); ok {
		if val, validType := v.(int); validType {
			perSecondLimit = val
		}
	}

	perHourLimit := defaultRateLimitPerHour
	if v, ok := data.GetOk("rate_limit_per_hour"); ok {
		if val, validType := v.(int); validType {
			perHourLimit = val
		}
	}

	// Initialize rate limiters with configured values
	InitRateLimiters(perSecondLimit, perHourLimit)

	// Create a new Name.com client
	namecomClient := namecom.New(username, token)

	// Set timeout on the client
	timeoutValue := data.Get("timeout")
	if timeoutInt, ok := timeoutValue.(int); ok {
		namecomClient.Client.Timeout = time.Duration(timeoutInt) * time.Second
	} else {
		namecomClient.Client.Timeout = time.Duration(defaultTimeoutSeconds) * time.Second
	}

	return namecomClient, nil
}
