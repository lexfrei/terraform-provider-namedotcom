# Terraform Provider for [name.com](https://name.com)

Special thanks to @mhumeSF for the original [name.com provider](https://github.com/mhumeSF/terraform-provider-namedotcom).

[API Docs](https://www.name.com/api-docs)

Supported features:

- DNS records
- NS records
- DNSSEC

## Usage

```HCL
# Set up the provider
terraform {
  required_providers {
    namedotcom = {
      source  = "lexfrei/namedotcom"
      version = "1.1.6"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "3.38.0"
    }
  }
}

provider "aws" {
  region = "us-west-2"
}

resource "aws_route53_zone" "example_com" {
  name = "example.com"
}

# Create the provider with your account details
provider "namedotcom" {
  username = var.namedotcom_username
  token    = var.namedotcom_token
}

# Example usage for creating DNS records
resource "namedotcom_record" "bar" {
  domain_name = "example.com"
  host        = ""
  record_type = "cname"
  answer      = "foo.com"
}

resource "namedotcom_record" "foo" {
  domain_name = "example.com"
  host        = "foo"
  record_type = "A"
  answer      = "1.2.3.4"
}

# Example usage for creating many records per domain
resource "namedotcom_record" "domain-me" {
  domain_name = "domain.me"
  record_type = "A"
  for_each = {
    ""   = "1.2.3.4"
    www  = "2.3.4.5"
    www1 = "3.4.5.6"
    www2 = "4.5.6.7"
  }

  host   = each.key
  answer = each.value
}

# Example usage for setting nameservers from a generated hosted_zone
resource "namedotcom_domain_nameservers" "example_com" {
  domain_name = "example.com"
  nameservers = [
    "${aws_route53_zone.example_com.name_servers.0}",
    "${aws_route53_zone.example_com.name_servers.1}",
    "${aws_route53_zone.example_com.name_servers.2}",
    "${aws_route53_zone.example_com.name_servers.3}",
  ]
}

# Example usage for using DNSSEC
resource "aws_route53_key_signing_key" "dnssec" {
  name                       = data.aws_route53_zone.example_com.name
  hosted_zone_id             = data.aws_route53_zone.example_com.id
  key_management_service_arn = aws_kms_key.dnssec.arn
  lifecycle {
    create_before_destroy = true
  }
}

resource "namedotcom_dnssec" "dnssec" {
  domain_name = aws_route53_zone.example_com.name
  key_tag     = aws_route53_key_signing_key.dnssec.key_tag
  algorithm   = aws_route53_key_signing_key.dnssec.signing_algorithm_type
  digest_type = aws_route53_key_signing_key.dnssec.digest_algorithm_type
  digest      = aws_route53_key_signing_key.dnssec.digest_value
}

```

### How to import record

You need to use format "domain:id" as last parameter for import command

```bash
# Import single record
terraform import namedotcom_record.example_record domain_name:recordId

# Import one of mentionned records in for_each
terraform import 'namedotcom_record.example_record["hostname"]' domain_name:recordId
```

To get recordId, you need to use namedotcom API for domain ListRecords and use ID for appropriate host

```bash
curl -u 'username:token' 'https://api.name.com/v4/domains/example.org/records'
```

### How to import dnssec entry

You need to use format "domain" as last parameter for import command

```bash
# Import single record
terraform import namedotcom_dnssec.dnssec domain_name

# Import one of mentionned records in for_each
terraform import 'namedotcom_dnssec.dnssec["domain_name"]' domain_name
```
