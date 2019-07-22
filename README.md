## Usage

Username and token must be generated from
`https://www.name.com/account/settings/api`

```HCL
provider "namedotcom" {
  token = "0123456789"
  username = "mhumesf"
}

resource "namedotcom_record" "foo" {
  domain_name = "example.com"
  host = "foo"
  record_type = "A"
  answer = "10.1.2.3"
}
```

Or settings nameservers from a generated hosted_zone

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