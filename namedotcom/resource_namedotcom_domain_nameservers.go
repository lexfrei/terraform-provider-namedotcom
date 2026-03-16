package namedotcom

import (
	"context"
	"net/http"
	"strings"

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
				Required:    true,
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

	nameservers, isSlice := data.Get("nameservers").([]any)
	if !isSlice {
		return errors.New("Error converting nameservers to []any")
	}

	for _, nameserver := range nameservers {
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

	err = data.Set("nameservers", nameservers)
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

	errMsg := err.Error()

	return strings.Contains(errMsg, "404") ||
		strings.Contains(errMsg, http.StatusText(http.StatusNotFound)) ||
		strings.Contains(errMsg, "not found") ||
		strings.Contains(errMsg, "Not found")
}

func resourceDomainNameServersUpdate(data *schema.ResourceData, meta any) error {
	client, isNamecom := meta.(*namecom.NameCom)
	if !isNamecom {
		return errors.New("Error converting interface to NameCom")
	}

	err := setNameservers(data, client)
	if err != nil {
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
		return errors.Wrap(err, "Error SetNameservers")
	}

	data.SetId("")

	return nil
}
