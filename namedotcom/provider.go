package namedotcom

import (
	"github.com/hashicorp/terraform/helper/schema"
	namecom "github.com/namedotcom/go/namecom"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{},
		Schema: map[string]*schema.Schema{
			"api_token": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("API_TOKEN", nil),
				Description: "API Token",
			},
			"api_username": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("API_USERNAME", nil),
				Description: "API Username",
			},
		},
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	token := d.Get("api_token").(string)
	username := d.Get("api_username").(string)
	config := NameCom{
		token,
		username,
	}
	config.New()
	return namecom
}
