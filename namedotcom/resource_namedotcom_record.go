package namedotcom

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"

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

// resourceRecordCreate creates a new record in the Name.com API
func resourceRecordCreate(data *schema.ResourceData, meta interface{}) error {
	resp, err := meta.(*namecom.NameCom).CreateRecord(
		&namecom.Record{
			DomainName: data.Get("domain_name").(string),
			Host:       data.Get("host").(string),
			Type:       data.Get("record_type").(string),
			Answer:     data.Get("answer").(string),
		},
	)
	if err != nil {
		return errors.Wrap(err, "Error CreateRecord")
	}

	data.SetId(strconv.Itoa(int(resp.ID)))
	return resourceRecordRead(data, meta)
}

// resourceRecordImporter import existing record from the Name.com API
func resourceRecordImporter(data *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	// data.Id() is the last argument passed to terraform import, in format domain:id
	importDomainName, importId, err := resourceRecordImporterParseId(data.Id())
	if err != nil {
		return nil, err
	}
	err = data.Set("domain_name", importDomainName)
	if err != nil {
		return nil, errors.Wrap(err, "Error setting domain_name")
	}
	data.SetId(importId)

	// possible block to switch using format of importing id domain:host instead of domain:id

	// resp, err := meta.(*namecom.NameCom).ListRecords(
	// 	&namecom.ListRecordsRequest{
	// 		DomainName: data.Get("domain_name").(string),
	// 	},
	// )
	// if err != nil {
	// 	return nil, fmt.Errorf("Error ImportRecord: %s", err)
	// }
	// for _, record := range resp.Records {
	// 	if record.Host == importId{
	// 		data.SetId(strconv.Itoa(int(record.ID)))
	// 		return []*schema.ResourceData{data}, err
	// 	}
	// }
	// return nil, fmt.Errorf("Error ImportRecord, host %s not found: %s", importId, err)

	return []*schema.ResourceData{data}, nil
}

func resourceRecordImporterParseId(id string) (string, string, error) {
	parts := strings.SplitN(id, ":", 2)

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("unexpected format of ID, expected domain:id")
	}

	return parts[0], parts[1], nil
}

// resourceRecordRead reads a record from the Name.com API
func resourceRecordRead(data *schema.ResourceData, meta interface{}) error {
	client := meta.(*namecom.NameCom)

	recordID, err := strconv.ParseInt(data.Id(), 10, 32)
	if err != nil {
		return errors.Wrap(err, "error converting Record ID")
	}

	request := namecom.GetRecordRequest{
		DomainName: data.Get("domain_name").(string),
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

// resourceRecordUpdate updates a record in the Name.com API
func resourceRecordUpdate(data *schema.ResourceData, meta interface{}) error {
	client := meta.(*namecom.NameCom)

	recordID, err := strconv.ParseInt(data.Id(), 10, 32)
	if err != nil {
		return errors.Wrap(err, "error converting Record ID")
	}

	updatedRecord := namecom.Record{
		ID:         int32(recordID),
		DomainName: data.Get("domain_name").(string),
		Host:       data.Get("host").(string),
		Type:       data.Get("record_type").(string),
		Answer:     data.Get("answer").(string),
	}

	_, err = client.UpdateRecord(&updatedRecord)
	if err != nil {
		return errors.Wrap(err, "Error UpdateRecord")
	}
	return resourceRecordRead(data, meta)
}

// resourceRecordDelete deletes a record from the Name.com API
func resourceRecordDelete(data *schema.ResourceData, meta interface{}) error {
	client := meta.(*namecom.NameCom)

	recordID, err := strconv.ParseInt(data.Id(), 10, 32)
	if err != nil {
		return errors.Wrap(err, "error converting Record ID")
	}

	deleteRequest := namecom.DeleteRecordRequest{
		DomainName: data.Get("domain_name").(string),
		ID:         int32(recordID),
	}

	_, err = client.DeleteRecord(&deleteRequest)
	if err != nil {
		return errors.Wrap(err, "Error DeleteRecord")
	}

	data.SetId("")
	return nil
}
