package namedotcom

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/namedotcom/go/v4/namecom"
)

func resourceDNSSEC() *schema.Resource {
	return &schema.Resource{
		Create: resourceDNSSECCreate,
		Read:   resourceDNSSECRead,
		Delete: resourceDNSSECDelete,
		Importer: &schema.ResourceImporter{
			State: resourceDNSSECImporter,
		},

		Schema: map[string]*schema.Schema{
			"DomainName": {
				Type:        schema.TypeString,
				Optional:    false,
				ForceNew:    true,
				Description: "DomainName is the zone that the DNSSEC belongs to",
			},
			"KeyTag": {
				Type:        schema.TypeInt,
				Optional:    false,
				ForceNew:    true,
				Description: "KeyTag contains the key tag value of the DNSKEY RR that validates this signature.",
			},
			"Algorithm": {
				Type:        schema.TypeInt,
				Optional:    false,
				ForceNew:    true,
				Description: "Algorithm is an integer identifying the algorithm used for signing. ",
			},
			"DigestType": {
				Type:        schema.TypeInt,
				Optional:    false,
				ForceNew:    true,
				Description: "DigestType is an integer identifying the algorithm used to create the digest.",
			},
			"Digest": {
				Type:        schema.TypeString,
				Optional:    false,
				ForceNew:    true,
				Description: "Digest is a digest of the DNSKEY RR that is registered with the registry.",
			},
		},
	}
}

// resourceDNSSECCreate creates a new DNSSEC in the Name.com API
func resourceDNSSECCreate(data *schema.ResourceData, meta interface{}) error {
	_, err := meta.(*namecom.NameCom).CreateDNSSEC(
		&namecom.DNSSEC{
			DomainName: data.Get("DomainName").(string),
			KeyTag:     data.Get("KeyTag").(int32),
			Algorithm:  data.Get("Algorigthm").(int32),
			DigestType: data.Get("DigestType").(int32),
			Digest:     data.Get("Digest").(string),
		},
	)

	if err != nil {
		return errors.Wrap(err, "Error CreateDNSSEC")
	}

	data.SetId(data.Get("DomainName").(string))

	return resourceDNSSECRead(data, meta)
}

// resourceDNSSECImporter import existing DNSSEC from the Name.com API
func resourceDNSSECImporter(data *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	importDomainName, err := resourceDNSSECImporterParseId(data.Id())
	if err != nil {
		return nil, err
	}
	err = data.Set("DomainName", importDomainName)
	if err != nil {
		return nil, errors.Wrap(err, "Error setting DomainName")
	}

	data.SetId(importDomainName)

	return []*schema.ResourceData{data}, nil
}

func resourceDNSSECImporterParseId(id string) (string, error) {
	parts := strings.SplitN(id, "_", 1)

	if len(parts) != 1 || parts[0] == "" {
		return "", errors.New("unexpected format of ID, expected DomainName")
	}

	return parts[0], nil
}

// resourceDNSSECRead reads a DNSSEC from the Name.com API
func resourceDNSSECRead(data *schema.ResourceData, meta interface{}) error {
	client := meta.(*namecom.NameCom)

	request := namecom.GetDNSSECRequest{
		DomainName: data.Get("DomainName").(string),
		Digest:     data.Get("Digest").(string),
	}

	DNSSEC, err := client.GetDNSSEC(&request)
	if err != nil {
		return errors.Wrap(err, "Error GetDNSSECRequest")
	}

	err = data.Set("domainName", DNSSEC.DomainName)
	if err != nil {
		return errors.Wrap(err, "Error setting domainName")
	}

	err = data.Set("KeyTag", DNSSEC.KeyTag)
	if err != nil {
		return errors.Wrap(err, "Error setting KeyTag")
	}

	err = data.Set("Algorithm", DNSSEC.Algorithm)
	if err != nil {
		return errors.Wrap(err, "Error setting Algorithm")
	}

	err = data.Set("DigestType", DNSSEC.DigestType)
	if err != nil {
		return errors.Wrap(err, "Error setting DigestType")
	}

	err = data.Set("Digest", DNSSEC.Digest)
	if err != nil {
		return errors.Wrap(err, "Error setting Digest")
	}

	return nil
}

// resourceDNSSECDelete deletes a DNSSEC from the Name.com API
func resourceDNSSECDelete(data *schema.ResourceData, meta interface{}) error {
	client := meta.(*namecom.NameCom)

	deleteRequest := namecom.DeleteDNSSECRequest{
		DomainName: data.Get("DomainName").(string),
		Digest:     data.Get("Digest").(string),
	}

	_, err := client.DeleteDNSSEC(&deleteRequest)
	if err != nil {
		return errors.Wrap(err, "Error DeleteDNSSEC")
	}

	data.SetId("")
	return nil
}
