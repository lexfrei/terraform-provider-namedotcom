package namedotcom

// Test-only exports for white-box testing from namedotcom_test package.

var (
	ResourceRecordImporterParseID   = resourceRecordImporterParseID
	ResourceDNSSECImporterParseID   = resourceDNSSECImporterParseID
	ValidateIntForInt32             = validateIntForInt32
	ValidateClient                  = validateClient
	ResourceRecordCreate            = resourceRecordCreate
	ResourceRecordRead              = resourceRecordRead
	ResourceRecordUpdate            = resourceRecordUpdate
	ResourceRecordDelete            = resourceRecordDelete
	ResourceRecordImporter          = resourceRecordImporter
	ResourceDNSSECCreate            = resourceDNSSECCreate
	ResourceDNSSECRead              = resourceDNSSECRead
	ResourceDNSSECDelete            = resourceDNSSECDelete
	ResourceDNSSECImporter          = resourceDNSSECImporter
	ResourceDomainNameServersCreate = resourceDomainNameServersCreate
	ResourceDomainNameServersRead   = resourceDomainNameServersRead
	ResourceDomainNameServersUpdate = resourceDomainNameServersUpdate
	ResourceDomainNameServersDelete = resourceDomainNameServersDelete
	GetDNSSECFromResourceData       = getDNSSECFromResourceData
	ResourceRecord                  = resourceRecord
	ResourceDNSSEC                  = resourceDNSSEC
	ResourceDomainNameServers       = resourceDomainNameServers
	DefaultPerSecondLimit           = defaultPerSecondLimit
	DefaultPerHourLimit             = defaultPerHourLimit
)
