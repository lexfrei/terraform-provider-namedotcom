package namedotcom

import (
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/namedotcom/go/v4/namecom"
)

// ErrValueOutsideInt32Range is returned when a value cannot be safely converted to int32.
var ErrValueOutsideInt32Range = errors.New("value outside of int32 range")

// validateIntForInt32 ensures an integer is within int32 range.
func validateIntForInt32(value int, fieldName string) error {
	if value > 2147483647 || value < -2147483648 {
		return fmt.Errorf("%s: %w", fieldName, ErrValueOutsideInt32Range)
	}

	return nil
}

// getDNSSECFromResourceData builds a DNSSEC struct from ResourceData.
func getDNSSECFromResourceData(data *schema.ResourceData) (*namecom.DNSSEC, error) {
	// Get and validate values
	keyTagValue, isInt := data.Get("key_tag").(int)
	if !isInt {
		return nil, errors.New("Error getting key_tag as int")
	}

	algorithmValue, isInt := data.Get("algorithm").(int)
	if !isInt {
		return nil, errors.New("Error getting algorithm as int")
	}

	digestTypeValue, isInt := data.Get("digest_type").(int)
	if !isInt {
		return nil, errors.New("Error getting digest_type as int")
	}

	// Validate int32 ranges
	if err := validateIntForInt32(keyTagValue, "key_tag"); err != nil {
		return nil, err
	}

	if err := validateIntForInt32(algorithmValue, "algorithm"); err != nil {
		return nil, err
	}

	if err := validateIntForInt32(digestTypeValue, "digest_type"); err != nil {
		return nil, err
	}

	// Get string values
	domainName, isStr := data.Get("domain_name").(string)
	if !isStr {
		return nil, errors.New("Error getting domain_name as string")
	}

	digest, isStr := data.Get("digest").(string)
	if !isStr {
		return nil, errors.New("Error getting digest as string")
	}

	// Build the DNSSEC struct
	//nolint:gosec // Safe to convert to int32 now (validated above)
	keyTag32 := int32(keyTagValue)
	//nolint:gosec // Safe to convert to int32 now (validated above)
	algorithm32 := int32(algorithmValue)
	//nolint:gosec // Safe to convert to int32 now (validated above)
	digestType32 := int32(digestTypeValue)

	return &namecom.DNSSEC{
		DomainName: domainName,
		KeyTag:     keyTag32,
		Algorithm:  algorithm32,
		DigestType: digestType32,
		Digest:     digest,
	}, nil
}

func resourceDNSSEC() *schema.Resource {
	return &schema.Resource{
		Create: resourceDNSSECCreate,
		Read:   resourceDNSSECRead,
		Delete: resourceDNSSECDelete,
		Importer: &schema.ResourceImporter{
			State: resourceDNSSECImporter,
		},

		Schema: map[string]*schema.Schema{
			"domain_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "DomainName is the zone that the DNSSEC belongs to",
			},
			"key_tag": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "KeyTag contains the key tag value of the DNSKEY RR that validates this signature.",
			},
			"algorithm": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Algorithm is an integer identifying the algorithm used for signing. ",
			},
			"digest_type": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "DigestType is an integer identifying the algorithm used to create the digest.",
			},
			"digest": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Digest is a digest of the DNSKEY RR that is registered with the registry.",
			},
		},
	}
}

// resourceDNSSECCreate creates a new DNSSEC in the Name.com API.
func resourceDNSSECCreate(data *schema.ResourceData, meta interface{}) error {
	client, isNamecom := meta.(*namecom.NameCom)
	if !isNamecom {
		return errors.New("Error converting meta to Name.com client")
	}

	// Build the DNSSEC struct from resource data
	dnssec, err := getDNSSECFromResourceData(data)
	if err != nil {
		return err
	}

	// Create the DNSSEC record
	_, err = client.CreateDNSSEC(dnssec)
	if err != nil {
		return errors.Wrap(err, "Error CreateDNSSEC")
	}

	data.SetId(dnssec.DomainName)

	return resourceDNSSECRead(data, meta)
}

// resourceDNSSECImporter import existing DNSSEC from the Name.com API.
func resourceDNSSECImporter(data *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client, isNamecom := meta.(*namecom.NameCom)
	if !isNamecom {
		return nil, errors.New("Error getting client")
	}

	importDomainName, importDigest, err := resourceDNSSECImporterParseID(data.Id())
	if err != nil {
		return nil, err
	}

	request := namecom.GetDNSSECRequest{
		DomainName: importDomainName,
		Digest:     importDigest,
	}

	DNSSEC, err := client.GetDNSSEC(&request)
	if err != nil {
		return nil, errors.Wrap(err, "Error GetDNSSECRequest")
	}

	err = data.Set("domain_name", DNSSEC.DomainName)
	if err != nil {
		return nil, errors.Wrap(err, "Error setting domain_name")
	}

	err = data.Set("key_tag", int(DNSSEC.KeyTag))
	if err != nil {
		return nil, errors.Wrap(err, "Error setting key_tag")
	}

	err = data.Set("algorithm", int(DNSSEC.Algorithm))
	if err != nil {
		return nil, errors.Wrap(err, "Error setting algorithm")
	}

	err = data.Set("digest_type", int(DNSSEC.DigestType))
	if err != nil {
		return nil, errors.Wrap(err, "Error setting digest_type")
	}

	err = data.Set("digest", DNSSEC.Digest)
	if err != nil {
		return nil, errors.Wrap(err, "Error setting digest")
	}

	data.SetId(importDomainName)

	return []*schema.ResourceData{data}, nil
}

// resourceDNSSECImporterParseID parses the ID of the DNSSEC.
func resourceDNSSECImporterParseID(id string) (domainName, digest string, err error) {
	//nolint:mnd // 2 is the expected number of parts
	parts := strings.SplitN(id, "_", 2)

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("unexpected format of ID, expected DomainName_Digest")
	}

	return parts[0], parts[1], nil
}

// resourceDNSSECRead reads a DNSSEC from the Name.com API.
func resourceDNSSECRead(data *schema.ResourceData, meta interface{}) error {
	client, isNamecom := meta.(*namecom.NameCom)
	if !isNamecom {
		return errors.New("Error getting client")
	}

	domainNameString, isStr := data.Get("domain_name").(string)
	if !isStr {
		return errors.New("Error getting domain_name")
	}

	digestString, isStr := data.Get("digest").(string)
	if !isStr {
		return errors.New("Error getting digest")
	}

	request := namecom.GetDNSSECRequest{
		DomainName: domainNameString,
		Digest:     digestString,
	}

	DNSSEC, err := client.GetDNSSEC(&request)
	if err != nil {
		return errors.Wrap(err, "Error GetDNSSECRequest")
	}

	err = data.Set("domain_name", DNSSEC.DomainName)
	if err != nil {
		return errors.Wrap(err, "Error setting domain_name")
	}

	err = data.Set("key_tag", int(DNSSEC.KeyTag))
	if err != nil {
		return errors.Wrap(err, "Error setting key_tag")
	}

	err = data.Set("algorithm", int(DNSSEC.Algorithm))
	if err != nil {
		return errors.Wrap(err, "Error setting algorithm")
	}

	err = data.Set("digest_type", int(DNSSEC.DigestType))
	if err != nil {
		return errors.Wrap(err, "Error setting digest_type")
	}

	err = data.Set("digest", DNSSEC.Digest)
	if err != nil {
		return errors.Wrap(err, "Error setting digest")
	}

	return nil
}

// resourceDNSSECDelete deletes a DNSSEC from the Name.com API.
func resourceDNSSECDelete(data *schema.ResourceData, meta interface{}) error {
	client, isNamecom := meta.(*namecom.NameCom)
	if !isNamecom {
		return errors.New("Error getting client")
	}

	domainNameString, isStr := data.Get("domain_name").(string)
	if !isStr {
		return errors.New("Error getting domain_name")
	}

	digestString, isStr := data.Get("digest").(string)
	if !isStr {
		return errors.New("Error getting digest")
	}

	deleteRequest := namecom.DeleteDNSSECRequest{
		DomainName: domainNameString,
		Digest:     digestString,
	}

	_, err := client.DeleteDNSSEC(&deleteRequest)
	if err != nil {
		return errors.Wrap(err, "Error DeleteDNSSEC")
	}

	data.SetId("")

	return nil
}
