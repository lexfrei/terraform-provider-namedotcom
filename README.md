# Terraform Provider for Name.com

[![OpenTofu Registry](https://img.shields.io/badge/OpenTofu%20Registry-namedotcom-FFDA18?logo=opentofu)](https://search.opentofu.org/provider/lexfrei/namedotcom/latest)
[![Terraform Registry](https://img.shields.io/badge/Terraform%20Registry-namedotcom-844FBA?logo=terraform)](https://registry.terraform.io/providers/lexfrei/namedotcom/latest)
[![Latest Release](https://img.shields.io/github/v/release/lexfrei/terraform-provider-namedotcom?label=release)](https://github.com/lexfrei/terraform-provider-namedotcom/releases/latest)
[![License](https://img.shields.io/github/license/lexfrei/terraform-provider-namedotcom)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/lexfrei/terraform-provider-namedotcom)](https://goreportcard.com/report/github.com/lexfrei/terraform-provider-namedotcom)
![CI](https://img.shields.io/github/actions/workflow/status/lexfrei/terraform-provider-namedotcom/lint.yaml?label=CI)

A Terraform provider that allows you to manage your [Name.com](https://name.com) DNS records and domain settings using infrastructure as code.

> Special thanks to [@mhumeSF](https://github.com/mhumeSF) for the original [name.com provider](https://github.com/mhumeSF/terraform-provider-namedotcom) that served as the foundation for this project.

## Features

- ✅ Create and manage DNS records (A, AAAA, ANAME, CNAME, MX, NS, SRV, TXT)
- ✅ Configure nameservers for domains
- ✅ Set up DNSSEC for domains
- ✅ Built-in rate limiting (20 req/sec and 3000 req/hour by default)

## Important: Terraform Registry Support

> **⚠️ The Terraform Registry is supported on a best-effort basis only.**
>
> I strongly recommend migrating to [OpenTofu](https://opentofu.org/) — a drop-in replacement for Terraform with a reliable, community-driven registry.

## Installation

### Using OpenTofu Registry (Recommended)

This provider is available in the [OpenTofu Registry](https://search.opentofu.org/provider/lexfrei/namedotcom/latest). To use it, add the following to your configuration:

```hcl
terraform {
  required_providers {
    namedotcom = {
      source  = "lexfrei/namedotcom"
      version = "~> 4.0"
    }
  }
}
```

### Using Terraform Registry

The provider is also listed in the [Terraform Registry](https://registry.terraform.io/providers/lexfrei/namedotcom/latest), but it may be outdated or broken. If you encounter issues, please switch to OpenTofu or install from source.

### Building from Source

If you prefer to build from source:

1. Clone this repository
2. Run `go mod tidy` to ensure dependencies
3. Run `go build -o terraform-provider-namedotcom`
4. Move the binary to your Terraform plugins directory

## Authentication

The provider requires API credentials from Name.com:

1. Log in to your [Name.com account](https://www.name.com/account/login)
2. Go to [API settings](https://www.name.com/account/settings/api)
3. Generate a token if you don't already have one

Then configure the provider with your credentials:

```hcl
provider "namedotcom" {
  username = var.namedotcom_username
  token    = var.namedotcom_token
}
```

**Recommended**: Store your credentials in environment variables or in a secure backend:

```hcl
provider "namedotcom" {
  # These can also be set with NAMEDOTCOM_USERNAME and NAMEDOTCOM_TOKEN environment variables
  username = var.namedotcom_username
  token    = var.namedotcom_token
}
```

## Usage Examples

A realistic configuration that exercises every resource the provider offers. It manages two domains with different strategies: `example.com` is hosted directly on Name.com, while `example.net` is delegated to an external DNS provider and secured with DNSSEC.

```hcl
provider "namedotcom" {
  # Credentials can also come from the NAMEDOTCOM_USERNAME and
  # NAMEDOTCOM_TOKEN environment variables.
  username = var.namedotcom_username
  token    = var.namedotcom_token

  # Optional: tune the built-in rate limiter (defaults shown).
  rate_limit_per_second = 20
  rate_limit_per_hour   = 3000
}

# --- example.com: DNS hosted on Name.com -------------------------------------

# Apex A record: an empty host targets the bare domain (example.com).
resource "namedotcom_record" "apex" {
  domain_name = "example.com"
  host        = ""
  record_type = "A"
  answer      = "192.0.2.1"
}

# IPv6 address for the apex.
resource "namedotcom_record" "apex_v6" {
  domain_name = "example.com"
  host        = ""
  record_type = "AAAA"
  answer      = "2001:db8::1"
}

# www as a CNAME pointing back to the apex.
resource "namedotcom_record" "www" {
  domain_name = "example.com"
  host        = "www"
  record_type = "CNAME"
  answer      = "example.com"
}

# Mail exchanger. MX records use priority; a lower value is preferred.
resource "namedotcom_record" "mail" {
  domain_name = "example.com"
  host        = ""
  record_type = "MX"
  answer      = "mail.example.com"
  priority    = 10
}

# SPF policy as a TXT record on the apex.
resource "namedotcom_record" "spf" {
  domain_name = "example.com"
  host        = ""
  record_type = "TXT"
  answer      = "v=spf1 -all"
}

# --- example.net: delegated to an external DNS provider, secured with DNSSEC -

# Point the domain at the external provider's nameservers.
resource "namedotcom_domain_nameservers" "example_net" {
  domain_name = "example.net"
  nameservers = [
    "ns1.dns.example",
    "ns2.dns.example",
  ]
}

# Register the DS record so the delegated zone is validated by the registry.
# These values come from the external DNS provider's key-signing key.
resource "namedotcom_dnssec" "example_net" {
  domain_name = "example.net"
  key_tag     = 12345
  algorithm   = 13
  digest_type = 2
  digest      = "6B3ED3311DE85004BF6DD325BA82340BC89B40B86D4055780F3BE4390B81B59A"
}
```

> Hosting a zone's records on Name.com and delegating that same zone elsewhere are mutually exclusive — once a domain is delegated, its records live with the other provider. See the [per-resource docs](#resource-documentation) for the full attribute reference and import syntax.

## Resource Documentation

Detailed documentation for each resource type is available:

- [Provider Configuration](docs/index.md)
- [DNS Records](docs/resources/record.md)
- [Domain Nameservers](docs/resources/domain_nameservers.md)
- [DNSSEC](docs/resources/dnssec.md)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -am 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the [BSD 3-Clause License](LICENSE).

## References

- [Name.com API Documentation](https://www.name.com/api-docs)
- [Terraform Registry Documentation](https://registry.terraform.io/providers/lexfrei/namedotcom/latest/docs)
