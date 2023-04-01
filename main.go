package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	"github.com/lexfrei/terraform-provider-namedotcom/namedotcom"
)

func main() {
	plugin.Serve(
		&plugin.ServeOpts{
			ProviderFunc: namedotcom.Provider,
		},
	)
}
