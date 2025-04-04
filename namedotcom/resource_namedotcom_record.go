package namedotcom

import (
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/namedotcom/go/v4/namecom"
)

func resourceRecord() *schema.Resource {
	return &schema.Resource{
		Create: resourceRecordCreate,
		Read:   resourceRecordRead,
		Update: resourceRecordUpdate,
		Delete: resourceRecordDelete,
		Importer: &schema.ResourceImporter{
			State: resourceRecordImporter,
		},

		Schema: map[string]*schema.Schema{
			"record_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Unique record id. Value is ignored on Create, and must match the URI on Update.",
			},
			"domain_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "DomainName is the zone that the record belongs to",
			},
			"host": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Host is the hostname relative to the zone",
			},
			"record_type": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Type is one of the following: A, AAAA, ANAME, CNAME, MX, NS, SRV, or TXT",
			},
			"answer": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Answer is either the IP address for A or AAAA records",
			},
		},
	}
}

// resourceRecordCreate creates a new record in the Name.com API.
func resourceRecordCreate(data *schema.ResourceData, meta interface{}) error {
	client, isNamecom := meta.(*namecom.NameCom)
	if !isNamecom {
		return errors.New("Error converting meta to Name.com client")
	}

	domainName, isStr := data.Get("domain_name").(string)
	if !isStr {
		return errors.New("Error getting domain_name as string")
	}

	host, isStr := data.Get("host").(string)
	if !isStr {
		return errors.New("Error getting host as string")
	}

	recordType, isStr := data.Get("record_type").(string)
	if !isStr {
		return errors.New("Error getting record_type as string")
	}

	answer, isStr := data.Get("answer").(string)
	if !isStr {
		return errors.New("Error getting answer as string")
	}

	record := &namecom.Record{
		DomainName: domainName,
		Host:       host,
		Type:       recordType,
		Answer:     answer,
	}

	// Create the record
	resp, err := client.CreateRecord(record)
	if err != nil {
		return errors.Wrap(err, "Error CreateRecord")
	}

	data.SetId(strconv.Itoa(int(resp.ID)))

	return resourceRecordRead(data, meta)
}

// resourceRecordImporter import existing record from the Name.com API.
func resourceRecordImporter(data *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	// data.Id() is the last argument passed to terraform import, in format domain:id
	importDomainName, importID, err := resourceRecordImporterParseID(data.Id())
	if err != nil {
		return nil, err
	}

	err = data.Set("domain_name", importDomainName)
	if err != nil {
		return nil, errors.Wrap(err, "Error setting domain_name")
	}

	data.SetId(importID)

	return []*schema.ResourceData{data}, nil
}

func resourceRecordImporterParseID(id string) (domain, recordID string, err error) {
	// Split the ID into two parts, the domain and the record ID.
	//nolint:mnd // 2 is the expected number of parts
	parts := strings.SplitN(id, ":", 2)

	// Check that the ID is in the expected format.
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("unexpected format of ID, expected domain:id")
	}

	return parts[0], parts[1], nil
}

// resourceRecordRead reads a record from the Name.com API.
func resourceRecordRead(data *schema.ResourceData, meta interface{}) error {
	client, isNamecom := meta.(*namecom.NameCom)
	if !isNamecom {
		return errors.New("Error converting meta to Name.com client")
	}

	recordID, err := strconv.ParseInt(data.Id(), 10, 32)
	if err != nil {
		return errors.Wrap(err, "error converting Record ID")
	}

	domainString, isStr := data.Get("domain_name").(string)
	if !isStr {
		return errors.New("Error getting domain_name")
	}

	request := namecom.GetRecordRequest{
		DomainName: domainString,
		ID:         int32(recordID),
	}

	record, err := client.GetRecord(&request)
	if err != nil {
		return errors.Wrap(err, "Error GetRecord")
	}

	err = data.Set("domain_name", record.DomainName)
	if err != nil {
		return errors.Wrap(err, "Error setting domain_name")
	}

	err = data.Set("host", record.Host)
	if err != nil {
		return errors.Wrap(err, "Error setting host")
	}

	err = data.Set("record_type", record.Type)
	if err != nil {
		return errors.Wrap(err, "Error setting record_type")
	}

	err = data.Set("answer", record.Answer)
	if err != nil {
		return errors.Wrap(err, "Error setting answer")
	}

	return nil
}

// resourceRecordUpdate updates a record in the Name.com API.
func resourceRecordUpdate(data *schema.ResourceData, meta interface{}) error {
	client, isNamecom := meta.(*namecom.NameCom)
	if !isNamecom {
		return errors.New("Error converting meta to Name.com client")
	}

	recordID, err := strconv.ParseInt(data.Id(), 10, 32)
	if err != nil {
		return errors.Wrap(err, "error converting Record ID")
	}

	domainNameString, isStr := data.Get("domain_name").(string)
	if !isStr {
		return errors.New("Error getting domain_name")
	}

	hostString, isStr := data.Get("host").(string)
	if !isStr {
		return errors.New("Error getting host")
	}

	recordTypeString, isStr := data.Get("record_type").(string)
	if !isStr {
		return errors.New("Error getting record_type")
	}

	answerString, isStr := data.Get("answer").(string)
	if !isStr {
		return errors.New("Error getting answer")
	}

	updatedRecord := namecom.Record{
		ID:         int32(recordID),
		DomainName: domainNameString,
		Host:       hostString,
		Type:       recordTypeString,
		Answer:     answerString,
	}

	_, err = client.UpdateRecord(&updatedRecord)
	if err != nil {
		return errors.Wrap(err, "Error UpdateRecord")
	}

	return resourceRecordRead(data, meta)
}

// resourceRecordDelete deletes a record from the Name.com API.
func resourceRecordDelete(data *schema.ResourceData, meta interface{}) error {
	client, isNamecom := meta.(*namecom.NameCom)
	if !isNamecom {
		return errors.New("Error converting meta to Name.com client")
	}

	recordID, err := strconv.ParseInt(data.Id(), 10, 32)
	if err != nil {
		return errors.Wrap(err, "error converting Record ID")
	}

	domainNameString, isStr := data.Get("domain_name").(string)
	if !isStr {
		return errors.New("Error getting domain_name")
	}

	deleteRequest := namecom.DeleteRecordRequest{
		DomainName: domainNameString,
		ID:         int32(recordID),
	}

	_, err = client.DeleteRecord(&deleteRequest)
	if err != nil {
		return errors.Wrap(err, "Error DeleteRecord")
	}

	data.SetId("")

	return nil
}
