## Usage

Username and token must be generated from
`https://www.name.com/account/settings/api`

```
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