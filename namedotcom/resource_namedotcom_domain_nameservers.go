package namedotcom

import (
	"context"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/namedotcom/go/v4/namecom"
)

const keyNameservers = "nameservers"

func resourceDomainNameServers() *schema.Resource {
	return &schema.Resource{
		Create: resourceDomainNameServersCreate,
		Read:   resourceDomainNameServersRead,
		Update: resourceDomainNameServersUpdate,
		Delete: resourceDomainNameServersDelete,

		// Bumped from 0 → 1 when nameservers switched from TypeList to TypeSet
		// so existing state values stored as cty.List can be coerced to cty.Set.
		SchemaVersion: 1,
		StateUpgraders: []schema.StateUpgrader{
			{
				Version: 0,
				Type:    resourceDomainNameServersV0().CoreConfigSchema().ImpliedType(),
				Upgrade: upgradeDomainNameServersV0ToV1,
			},
		},

		Schema: map[string]*schema.Schema{
			"domain_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "DomainName is the punycode encoded value of the domain name.",
			},
			keyNameservers: {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				//nolint:lll //Description can be long
				Description: "Nameservers is the set of nameservers for this domain. Order is not significant; the registry treats nameservers as a set. If unspecified it defaults to your account default nameservers.",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

// resourceDomainNameServersV0 returns the schema used before SchemaVersion 1,
// when nameservers was a TypeList. Required so the SDK knows the source cty
// type when reading older state during the upgrade.
func resourceDomainNameServersV0() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"domain_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			keyNameservers: {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

// upgradeDomainNameServersV0ToV1 is an identity upgrade: the raw element type
// is unchanged (list of strings → set of strings), and the SDK handles the
// cty list-to-set coercion based on the new schema.
func upgradeDomainNameServersV0ToV1(
	_ context.Context,
	rawState map[string]any,
	_ any,
) (map[string]any, error) {
	return rawState, nil
}

// setNameservers calls the Name.com API to set nameservers for a domain.
func setNameservers(data *schema.ResourceData, client *namecom.NameCom) error {
	err := RespectRateLimits(context.Background())
	if err != nil {
		return errors.Wrap(err, "rate limiting error")
	}

	domainName, isStr := data.Get("domain_name").(string)
	if !isStr {
		return errors.New("Error converting domain_name to string")
	}

	request := namecom.SetNameserversRequest{
		DomainName: domainName,
	}

	nameservers, isSet := data.Get(keyNameservers).(*schema.Set)
	if !isSet {
		return errors.New("Error converting nameservers to *schema.Set")
	}

	for _, nameserver := range nameservers.List() {
		nameserverString, isStr := nameserver.(string)
		if !isStr {
			return errors.New("Error converting nameserver to string")
		}

		request.Nameservers = append(request.Nameservers, nameserverString)
	}

	_, err = client.SetNameservers(&request)
	if err != nil {
		return errors.Wrap(err, "Error SetNameservers")
	}

	return nil
}

func resourceDomainNameServersCreate(data *schema.ResourceData, meta any) error {
	client, isNamecom := meta.(*namecom.NameCom)
	if !isNamecom {
		return errors.New("Error converting interface to NameCom")
	}

	err := setNameservers(data, client)
	if err != nil {
		return err
	}

	domainName, isStr := data.Get("domain_name").(string)
	if !isStr {
		return errors.New("Error converting domain_name to string")
	}

	data.SetId(domainName)

	return resourceDomainNameServersRead(data, meta)
}

func resourceDomainNameServersRead(data *schema.ResourceData, meta any) error {
	client, isNamecom := meta.(*namecom.NameCom)
	if !isNamecom {
		return errors.New("Error converting interface to NameCom")
	}

	err := RespectRateLimits(context.Background())
	if err != nil {
		return errors.Wrap(err, "rate limiting error")
	}

	domainName, isStr := data.Get("domain_name").(string)
	if !isStr {
		return errors.New("Error converting domain_name to string")
	}

	domain, err := client.GetDomain(&namecom.GetDomainRequest{
		DomainName: domainName,
	})
	if err != nil {
		// If the domain no longer exists, remove it from state
		if isDomainNotFound(err) {
			data.SetId("")

			return nil
		}

		return errors.Wrap(err, "Error GetDomain")
	}

	nameservers := make([]any, len(domain.Nameservers))
	for idx, ns := range domain.Nameservers {
		nameservers[idx] = ns
	}

	err = data.Set(keyNameservers, nameservers)
	if err != nil {
		return errors.Wrap(err, "Error setting nameservers")
	}

	return nil
}

// isDomainNotFound checks if the API error indicates the domain was not found.
func isDomainNotFound(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())

	return strings.Contains(errMsg, "404") ||
		strings.Contains(errMsg, "not found")
}

func resourceDomainNameServersUpdate(data *schema.ResourceData, meta any) error {
	client, isNamecom := meta.(*namecom.NameCom)
	if !isNamecom {
		return errors.New("Error converting interface to NameCom")
	}

	err := setNameservers(data, client)
	if err != nil {
		if isDomainNotFound(err) {
			data.SetId("")

			return nil
		}

		return err
	}

	return resourceDomainNameServersRead(data, meta)
}

func resourceDomainNameServersDelete(data *schema.ResourceData, meta any) error {
	client, isNamecom := meta.(*namecom.NameCom)
	if !isNamecom {
		return errors.New("Error converting interface to NameCom")
	}

	err := RespectRateLimits(context.Background())
	if err != nil {
		return errors.Wrap(err, "rate limiting error")
	}

	domainName, isStr := data.Get("domain_name").(string)
	if !isStr {
		return errors.New("Error converting domain_name to string")
	}

	request := namecom.SetNameserversRequest{
		DomainName: domainName,
	}

	_, err = client.SetNameservers(&request)
	if err != nil {
		// If domain no longer exists, resource is already gone
		if isDomainNotFound(err) {
			data.SetId("")

			return nil
		}

		return errors.Wrap(err, "Error SetNameservers")
	}

	data.SetId("")

	return nil
}
