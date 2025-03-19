package namedotcom

import (
	"math"
	"strings"

	"github.com/cockroachdb/errors"

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
	client, ok := meta.(*namecom.NameCom)
	if !ok {
		return errors.New("Error getting client")
	}

	domainName, ok := data.Get("domain_name").(string)
	if !ok || domainName == "" {
		return errors.New("Error getting domain_name: must be a non-empty string")
	}

	keyTagInt, ok := data.Get("key_tag").(int)
	if !ok {
		return errors.New("Error getting key_tag: must be an integer")
	}
	// Check for integer overflow before conversion
	if keyTagInt < math.MinInt32 || keyTagInt > math.MaxInt32 {
		return errors.New("Error: key_tag is outside the valid range for int32")
	}

	keyTag := int32(keyTagInt)

	algorithmInt, ok := data.Get("algorithm").(int)
	if !ok {
		return errors.New("Error getting algorithm: must be an integer")
	}
	// Check for integer overflow before conversion
	if algorithmInt < math.MinInt32 || algorithmInt > math.MaxInt32 {
		return errors.New("Error: algorithm is outside the valid range for int32")
	}

	algorithm := int32(algorithmInt)

	digestTypeInt, ok := data.Get("digest_type").(int)
	if !ok {
		return errors.New("Error getting digest_type: must be an integer")
	}
	// Check for integer overflow before conversion
	if digestTypeInt < math.MinInt32 || digestTypeInt > math.MaxInt32 {
		return errors.New("Error: digest_type is outside the valid range for int32")
	}

	digestType := int32(digestTypeInt)

	digest, ok := data.Get("digest").(string)
	if !ok || digest == "" {
		return errors.New("Error getting digest: must be a non-empty string")
	}

	// Further validation of DNSSEC parameters
	if keyTag < 0 || keyTag > 65535 {
		return errors.New("Error: key_tag must be between 0 and 65535")
	}

	// Algorithm validation based on IANA registry
	validAlgorithms := map[int32]bool{
		1: true, 2: true, 3: true, 5: true, 6: true, 7: true, 8: true,
		10: true, 12: true, 13: true, 14: true, 15: true, 16: true,
	}
	if !validAlgorithms[algorithm] {
		return errors.New("Error: algorithm is not a valid DNSSEC algorithm number")
	}

	// Digest type validation based on IANA registry
	validDigestTypes := map[int32]bool{1: true, 2: true, 3: true, 4: true}
	if !validDigestTypes[digestType] {
		return errors.New("Error: digest_type is not a valid DNSSEC digest type")
	}

	// Create the DNSSEC record with validated parameters
	_, err := client.CreateDNSSEC(
		&namecom.DNSSEC{
			DomainName: domainName,
			KeyTag:     keyTag,
			Algorithm:  algorithm,
			DigestType: digestType,
			Digest:     digest,
		},
	)
	if err != nil {
		return errors.Wrap(err, "Error CreateDNSSEC")
	}

	data.SetId(domainName)

	return resourceDNSSECRead(data, meta)
}

// resourceDNSSECImporter import existing DNSSEC from the Name.com API.
func resourceDNSSECImporter(data *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client, ok := meta.(*namecom.NameCom)
	if !ok {
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
	parts := strings.SplitN(id, "_", 2)

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("unexpected format of ID, expected DomainName_Digest")
	}

	return parts[0], parts[1], nil
}

// resourceDNSSECRead reads a DNSSEC from the Name.com API.
func resourceDNSSECRead(data *schema.ResourceData, meta interface{}) error {
	client, ok := meta.(*namecom.NameCom)
	if !ok {
		return errors.New("Error getting client")
	}

	domainNameString, ok := data.Get("domain_name").(string)
	if !ok {
		return errors.New("Error getting domain_name")
	}

	digestString, ok := data.Get("digest").(string)
	if !ok {
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
	client, ok := meta.(*namecom.NameCom)
	if !ok {
		return errors.New("Error getting client")
	}

	domainNameString, ok := data.Get("domain_name").(string)
	if !ok {
		return errors.New("Error getting domain_name")
	}

	digestString, ok := data.Get("digest").(string)
	if !ok {
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
