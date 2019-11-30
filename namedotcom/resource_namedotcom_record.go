package namedotcom

import (
	"fmt"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/namedotcom/go/namecom"
	"strconv"
)

func resourceRecord() *schema.Resource {
	return &schema.Resource{
		Create: resourceRecordCreate,
		Read:   resourceRecordRead,
		Update: resourceRecordUpdate,
		Delete: resourceRecordDelete,

		Schema: map[string]*schema.Schema{
			"record_id": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Unique record id. Value is ignored on Create, and must match the URI on Update.",
			},
			"domain_name": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "DomainName is the zone that the record belongs to",
			},
			"host": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Host is the hostname relative to the zone",
			},
			"record_type": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Type is one of the following: A, AAAA, ANAME, CNAME, MX, NS, SRV, or TXT",
			},
			"answer": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Answer is either the IP address for A or AAAA records",
			},
		},
	}
}

func resourceRecordCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*namecom.NameCom)
	record := namecom.Record{
		DomainName: d.Get("domain_name").(string),
		Host:       d.Get("host").(string),
		Type:       d.Get("record_type").(string),
		Answer:     d.Get("answer").(string),
	}

	resp, err := client.CreateRecord(&record)
	if err != nil {
		return fmt.Errorf("Error GetRecord: %s", err)
	}

	d.SetId(strconv.Itoa(int(resp.ID)))
	return resourceRecordRead(d, m)
}

func resourceRecordRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*namecom.NameCom)

	// TODO handle error
	recordID, err := strconv.ParseInt(d.Id(), 10, 32)
	if err != nil {
		return fmt.Errorf("Error converting Record ID: %s", err)
	}

	request := namecom.GetRecordRequest{
		DomainName: d.Get("domain_name").(string),
		ID:         int32(recordID),
	}

	record, err := client.GetRecord(&request)
	if err != nil {
		return fmt.Errorf("Error GetRecord: %s", err)
	}

	d.Set("domain_name", record.DomainName)
	d.Set("host", record.Host)
	d.Set("record_type", record.Type)
	d.Set("answer", record.Answer)

	return nil
}

func resourceRecordUpdate(d *schema.ResourceData, m interface{}) error {
	client := m.(*namecom.NameCom)

	recordID, err := strconv.ParseInt(d.Id(), 10, 32)
	if err != nil {
		return fmt.Errorf("Error Parsing Record ID: %s", err)
	}

	updatedRecord := namecom.Record{
		ID:         int32(recordID),
		DomainName: d.Get("domain_name").(string),
		Host:       d.Get("host").(string),
		Type:       d.Get("record_type").(string),
		Answer:     d.Get("answer").(string),
	}

	_, err = client.UpdateRecord(&updatedRecord)
	if err != nil {
		return fmt.Errorf("Error UpdateRecord: %s", err)
	}
	return resourceRecordRead(d, m)
}

func resourceRecordDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*namecom.NameCom)

	// TODO handle error
	recordID, err := strconv.ParseInt(d.Id(), 10, 32)
	if err != nil {
		return fmt.Errorf("Error converting Record ID: %s", err)
	}

	deleteRequest := namecom.DeleteRecordRequest{
		DomainName: d.Get("domain_name").(string),
		ID:         int32(recordID),
	}
	client.DeleteRecord(&deleteRequest)

	d.SetId("")
	return nil
}
