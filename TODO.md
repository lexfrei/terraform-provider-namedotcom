## TODO:
- Add tests
- Currently when deleting nameservers records from terraform; the zone's
  nameserver entries are emtpy. Need to revise function to restore/reset to
  original name.com nameservers.
- Append other resources
    - "namedotcom_domain":
    - "namedotcom_domain_autorenew":
    - "namedotcom_domain_contact":
    - "namedotcom_domain_lock":
    - "namedotcom_dnssec":
    - "namedotcom_email_forwarding":
    - "namedotcom_transfer":
    - "namedotcom_url_forwarding":
    - "namedotcom_vanity_nameserver":
