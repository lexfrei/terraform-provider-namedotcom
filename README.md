# Terraform Provider for Name.com

[![Go Report Card](https://goreportcard.com/badge/github.com/lexfrei/terraform-provider-namedotcom)](https://goreportcard.com/report/github.com/lexfrei/terraform-provider-namedotcom)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/lexfrei/terraform-provider-namedotcom/lint.yaml)
![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/lexfrei/terraform-provider-namedotcom)

A Terraform provider that allows you to manage your [Name.com](https://name.com) DNS records and domain settings using infrastructure as code.

> Special thanks to [@mhumeSF](https://github.com/mhumeSF) for the original [name.com provider](https://github.com/mhumeSF/terraform-provider-namedotcom) that served as the foundation for this project.

## Features

- ✅ Create and manage DNS records (A, AAAA, CNAME, MX, NS, SRV, TXT)
- ✅ Configure nameservers for domains
- ✅ Set up DNSSEC for domains
- ✅ Built-in rate limiting (20 req/sec and 3000 req/hour by default)

## Installation

### Using Terraform Registry (Recommended)

This provider is available in the [Terraform Registry](https://registry.terraform.io/providers/lexfrei/namedotcom/latest). To use it, add the following to your Terraform configuration:

```hcl
terraform {
  required_providers {
    namedotcom = {
      source  = "lexfrei/namedotcom"
      version = "1.1.6"
    }
  }
}
```

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

### Managing DNS Records

```hcl
# Create an A record
resource "namedotcom_record" "website" {
  domain_name = "example.com"
  host        = "www"
  record_type = "A"
  answer      = "192.0.2.1"
}

# Create a CNAME record
resource "namedotcom_record" "alias" {
  domain_name = "example.com"
  host        = "blog"
  record_type = "CNAME"
  answer      = "www.example.com"
}

# Create multiple records for the same domain
resource "namedotcom_record" "multi_records" {
  domain_name = "example.com"
  record_type = "A"
  
  for_each = {
    ""      = "192.0.2.1"  # Apex domain
    "www"   = "192.0.2.1"
    "api"   = "192.0.2.2"
    "admin" = "192.0.2.3"
  }

  host   = each.key
  answer = each.value
}
```

### Setting Custom Nameservers

```hcl
# Use custom nameservers (e.g., with AWS Route 53)
resource "aws_route53_zone" "example_zone" {
  name = "example.com"
}

resource "namedotcom_domain_nameservers" "example_nameservers" {
  domain_name = "example.com"
  nameservers = [
    aws_route53_zone.example_zone.name_servers[0],
    aws_route53_zone.example_zone.name_servers[1],
    aws_route53_zone.example_zone.name_servers[2],
    aws_route53_zone.example_zone.name_servers[3],
  ]
}
```

### Setting Up DNSSEC

```hcl
# Configure DNSSEC with AWS Route 53
resource "aws_route53_zone" "example_zone" {
  name = "example.com"
}

resource "aws_route53_key_signing_key" "example_ksk" {
  name                       = "example-key"
  hosted_zone_id             = aws_route53_zone.example_zone.id
  key_management_service_arn = aws_kms_key.example_kms.arn
}

resource "aws_route53_hosted_zone_dnssec" "example" {
  hosted_zone_id = aws_route53_zone.example_zone.id
}

resource "namedotcom_dnssec" "example_dnssec" {
  domain_name = "example.com"
  key_tag     = aws_route53_key_signing_key.example_ksk.key_tag
  algorithm   = aws_route53_key_signing_key.example_ksk.signing_algorithm_type
  digest_type = aws_route53_key_signing_key.example_ksk.digest_algorithm_type
  digest      = aws_route53_key_signing_key.example_ksk.digest_value
}
```

### API Rate Limiting

The provider includes built-in rate limiting to prevent hitting Name.com's API limits. The defaults should work for most use cases, but you can adjust them if needed:

```hcl
provider "namedotcom" {
  username = var.namedotcom_username
  token    = var.namedotcom_token
  
  # Optional rate limiting configuration
  rate_limit_per_second = 20   # Default: 20
  rate_limit_per_hour   = 3000 # Default: 3000
}
```

## Resource Importing

### Importing DNS Records

To import existing DNS records into your Terraform state:

```bash
# Format: terraform import namedotcom_record.[resource_name] [domain_name]:[record_id]
terraform import namedotcom_record.website example.com:12345

# For records in a for_each block
terraform import 'namedotcom_record.multi_records["www"]' example.com:12345
```

To find the record ID, use the Name.com API:

```bash
curl -u 'username:token' 'https://api.name.com/v4/domains/example.com/records'
```

### Importing DNSSEC Settings

To import existing DNSSEC settings:

```bash
# Format: terraform import namedotcom_dnssec.[resource_name] [domain_name]
terraform import namedotcom_dnssec.example_dnssec example.com

# For resources in a for_each block
terraform import 'namedotcom_dnssec.dnssec["example.com"]' example.com
```

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

This project is licensed under the [MIT License](LICENSE).

## References

- [Name.com API Documentation](https://www.name.com/api-docs)
- [Terraform Registry Documentation](https://registry.terraform.io/providers/lexfrei/namedotcom/latest/docs)
