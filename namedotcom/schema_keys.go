package namedotcom

// Schema attribute keys reused across the provider and resource definitions.
// Kept here so goconst stays satisfied and renames stay cheap.
const (
	keyDomainName         = "domain_name"
	keyHost               = "host"
	keyRecordType         = "record_type"
	keyAnswer             = "answer"
	keyRecordID           = "record_id"
	keyKeyTag             = "key_tag"
	keyAlgorithm          = "algorithm"
	keyDigestType         = "digest_type"
	keyDigest             = "digest"
	keyNameservers        = "nameservers"
	keyToken              = "token"
	keyUsername           = "username"
	keyRateLimitPerSecond = "rate_limit_per_second"
	keyRateLimitPerHour   = "rate_limit_per_hour"
	keyTimeout            = "timeout"
)
