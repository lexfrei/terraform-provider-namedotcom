# Terraform Provider for [name.com](https://name.com)

[API Docs](https://www.name.com/api-docs)

Currently only supports DNS records and setting nameservers for a domain zone.

## Usage

Username and token must be generated from
`https://www.name.com/account/settings/api`

```HCL
provider "namedotcom" {
  token = "0123456789"
  username = "mhumesf"
}

// example.com CNAME -> bar.com
resource "namedotcom_record" "bar" {
  domain_name = "example.com"
  host = ""
  record_type = "cname"
  answer = "bar.com"
}

// foo.example.com -> 10.1.2.3
resource "namedotcom_record" "foo" {
  domain_name = "example.com"
  host = "foo"
  record_type = "A"
  answer = "10.1.2.3"
}
```

Setting nameservers from a generated hosted_zone

```HCL
provider "aws" {
  region = "us-west-2"
}

provider "namedotcom" {
  token = "0123456789"
  username = "mhumesf"
}

resource "aws_route53_zone" "example_com" {
  name = "example.com"
}

resource "namedotcom_domain_nameservers" "example_com" {
  domain_name = "example.com"
  nameservers = [
    "${aws_route53_zone.example_com.name_servers.0}",
    "${aws_route53_zone.example_com.name_servers.1}",
    "${aws_route53_zone.example_com.name_servers.2}",
    "${aws_route53_zone.example_com.name_servers.3}",
  ]
}
```

