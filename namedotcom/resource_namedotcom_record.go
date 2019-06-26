package namedotcom

import (
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
	client.CreateRecord(&record)
	return resourceRecordRead(d, m)
}

func resourceRecordRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*namecom.NameCom)

	domain_name := d.Get("domain_name").(string)
	request := namecom.ListRecordsRequest{DomainName: domain_name}
	r, _ := client.ListRecords(&request)

	// Get record_id from list of records matching `domain_name`
	var record_id int32
	for _, v := range r.Records {
		if v.DomainName == domain_name {
			record_id = v.ID
		}
	}

	d.SetId(strconv.Itoa(int(record_id)))
	// d.Set("DomainName", folder.Name)
	// d.Set("Host", folder.Parent)
	// d.Set("Type", folder.DisplayName)
	// d.Set("Answer", folder.LifecycleState)

	return nil
}

func resourceRecordUpdate(d *schema.ResourceData, m interface{}) error {
	client := m.(*namecom.NameCom)

	// TODO
	// Pagination???

	domain_name := d.Get("domain_name").(string)
	request := namecom.ListRecordsRequest{DomainName: domain_name}
	r, _ := client.ListRecords(&request)

	// Get record_id from list of records matching `domain_name`
	var record_id int32
	for _, v := range r.Records {
		if v.DomainName == domain_name {
			record_id = v.ID
		}
	}

	updatedRecord := namecom.Record{
		ID:         record_id,
		DomainName: d.Get("domain_name").(string),
		Host:       d.Get("host").(string),
		Type:       d.Get("record_type").(string),
		Answer:     d.Get("answer").(string),
	}
	client.UpdateRecord(&updatedRecord)
	return resourceRecordRead(d, m)
}

func resourceRecordDelete(d *schema.ResourceData, m interface{}) error {
	return nil
}
