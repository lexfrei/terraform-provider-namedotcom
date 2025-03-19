package namedotcom

import (
	"math"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/namedotcom/go/v4/namecom"
)

// Constants for record types and TTL values.
const (
	RecordTypeMX  = "MX"
	RecordTypeSRV = "SRV"
	MinTTL        = 300
	MaxPriority   = 65535
)

// Valid DNS record types.
var validRecordTypes = map[string]bool{
	"A": true, "AAAA": true, "CNAME": true, RecordTypeMX: true,
	"TXT": true, "NS": true, RecordTypeSRV: true, "CAA": true,
	"TLSA": true, "SSHFP": true, "PTR": true,
}

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

// resourceRecordCreate creates a new DNS record in the Name.com API.
func resourceRecordCreate(data *schema.ResourceData, meta interface{}) error {
	client, ok := meta.(*namecom.NameCom)
	if !ok {
		return errors.New("error getting client")
	}

	domainName, ok := data.Get("domain_name").(string)
	if !ok || domainName == "" {
		return errors.New("error getting domain_name: must be a non-empty string")
	}

	host, ok := data.Get("host").(string)
	if !ok {
		return errors.New("error getting host: must be a string")
	}

	recordType, ok := data.Get("type").(string)
	if !ok || recordType == "" {
		return errors.New("error getting type: must be a non-empty string")
	}

	// Validate record type
	if !validRecordTypes[recordType] {
		return errors.New("error: type must be a valid DNS record type")
	}

	answer, ok := data.Get("answer").(string)
	if !ok || answer == "" {
		return errors.New("error getting answer: must be a non-empty string")
	}

	ttlInt, ok := data.Get("ttl").(int)
	if !ok {
		return errors.New("error getting ttl: must be an integer")
	}

	// Validate TTL
	if ttlInt < 0 {
		return errors.New("error: ttl cannot be negative")
	}

	if ttlInt > math.MaxUint32 {
		return errors.New("error: ttl exceeds maximum value for uint32")
	}

	if uint32(ttlInt) < MinTTL {
		return errors.New("error: ttl must be at least 300 seconds")
	}

	if ttlInt < math.MinInt32 || ttlInt > math.MaxInt32 {
		return errors.New("error: ttl is outside the valid range for int32")
	}

	ttl := uint32(ttlInt)

	// Create the record with validated parameters
	record := &namecom.Record{
		DomainName: domainName,
		Host:       host,
		Type:       recordType,
		Answer:     answer,
		TTL:        ttl,
	}

	// Handle priority for MX and SRV records
	//nolint:nestif // This is a valid use of nesting
	if recordType == RecordTypeMX || recordType == RecordTypeSRV {
		priorityInt, ok := data.Get("priority").(int)
		if !ok {
			return errors.New("error getting priority: must be an integer for MX or SRV records")
		}

		// Validate priority
		if priorityInt < 0 {
			return errors.New("error: priority cannot be negative")
		}

		if priorityInt > math.MaxUint32 {
			return errors.New("error: priority exceeds maximum value for uint32")
		}

		if uint32(priorityInt) > MaxPriority {
			return errors.New("error: priority must be between 0 and 65535")
		}

		if priorityInt < math.MinInt32 || priorityInt > math.MaxInt32 {
			return errors.New("error: priority is outside the valid range for int32")
		}

		record.Priority = uint32(priorityInt)
	}

	createResponse, err := client.CreateRecord(record)
	if err != nil {
		return errors.Wrap(err, "error CreateRecord")
	}

	data.SetId(strconv.Itoa(int(createResponse.ID)))

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
	parts := strings.SplitN(id, ":", 2)

	// Check that the ID is in the expected format.
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("unexpected format of ID, expected domain:id")
	}

	return parts[0], parts[1], nil
}

// resourceRecordRead reads a record from the Name.com API.
func resourceRecordRead(data *schema.ResourceData, meta interface{}) error {
	client, ok := meta.(*namecom.NameCom)
	if !ok {
		return errors.New("Error converting meta to Name.com client")
	}

	recordID, err := strconv.ParseInt(data.Id(), 10, 32)
	if err != nil {
		return errors.Wrap(err, "error converting Record ID")
	}

	domainString, ok := data.Get("domain_name").(string)
	if !ok {
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
	client, ok := meta.(*namecom.NameCom)
	if !ok {
		return errors.New("Error converting meta to Name.com client")
	}

	recordID, err := strconv.ParseInt(data.Id(), 10, 32)
	if err != nil {
		return errors.Wrap(err, "error converting Record ID")
	}

	domainNameString, ok := data.Get("domain_name").(string)
	if !ok {
		return errors.New("Error getting domain_name")
	}

	hostString, ok := data.Get("host").(string)
	if !ok {
		return errors.New("Error getting host")
	}

	recordTypeString, ok := data.Get("record_type").(string)
	if !ok {
		return errors.New("Error getting record_type")
	}

	answerString, ok := data.Get("answer").(string)
	if !ok {
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
	client, ok := meta.(*namecom.NameCom)
	if !ok {
		return errors.New("Error converting meta to Name.com client")
	}

	recordID, err := strconv.ParseInt(data.Id(), 10, 32)
	if err != nil {
		return errors.Wrap(err, "error converting Record ID")
	}

	domainNameString, ok := data.Get("domain_name").(string)
	if !ok {
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
