package namedotcom

import (
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceRecord() *schema.Resource {
	return &schema.Resource{
		Create: resourceRecordCreate,
		// Read:   resourceRecordRead,
		// Update: resourceRecordUpdate,
		// Delete: resourceRecordDelete,

		Schema: map[string]*schema.Schema{
			"id": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Unique record id",
			},
			"domainName": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "DomainName is the zone that the record belongs to",
			},
			"host": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Host is the hostname relative to the zone",
			},
			"fqdn": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "FQDN is the Fully Qualified Domain Name",
			},
			"type": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Type is one of the following: A, AAAA, ANAME, CNAME, MX, NS, SRV, or TXT",
			},
			"answer": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Answer is either the IP address for A or AAAA records",
			},
			"ttl": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "TTL is the time this record can be cached for in seconds",
			},
			"priority": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Priority is only required for MX and SRV records",
			},
		},
	}
}

func resourceRecordCreate(d *schema.ResourceData, m interface{}) error {
	// createRecord
	return nil
}

// func resourceRecordRead(d *schema.ResourceData, m interface{}) error {
// 	// readRecord
// 	return nil
// }
//
// func resourceRecordUpdate(d *schema.ResourceData, m interface{}) error {
// 	// updateRecord
// 	return nil
// }
//
// func resourceRecordDelete(d *schema.ResourceData, m interface{}) error {
// 	// deleteRecord
// 	return nil
// }
