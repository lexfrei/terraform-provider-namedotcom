package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/lexfrei/terraform-provider-namedotcom/namedotcom"
)

// protocolVersion is the Terraform plugin protocol the framework serves.
// Protocol 6 requires Terraform >= 1.0 / OpenTofu.
const protocolVersion = 6

// These are set by the linker via ldflags during the release build.
var (
	version = "dev"
	commit  = "none"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	log.Printf("[INFO] terraform-provider-namedotcom version %s (commit %s)", version, commit)

	err := providerserver.Serve(context.Background(), namedotcom.New(version), providerserver.ServeOpts{
		Address:         "registry.terraform.io/lexfrei/namedotcom",
		ProtocolVersion: protocolVersion,
		Debug:           debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
