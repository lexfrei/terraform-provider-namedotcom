package namedotcom

// Test-only exports for white-box testing from the namedotcom_test package.

var (
	// Import-ID parsing helpers.
	ResourceRecordImporterParseID = resourceRecordImporterParseID
	ResourceDNSSECImporterParseID = resourceDNSSECImporterParseID
	ParseRecordID                 = parseRecordID

	// Provider configuration helpers.
	ResolveCredentials = resolveCredentials
	BuildClient        = buildClient
	ConfigureClient    = configureClient

	// Resource helpers.
	IsNotFoundError    = isNotFoundError
	ExtractNameservers = extractNameservers

	// API translation helpers.
	CreateRecordAPI    = createRecordAPI
	ReadRecordAPI      = readRecordAPI
	UpdateRecordAPI    = updateRecordAPI
	DeleteRecordAPI    = deleteRecordAPI
	CreateDNSSECAPI    = createDNSSECAPI
	ReadDNSSECAPI      = readDNSSECAPI
	DeleteDNSSECAPI    = deleteDNSSECAPI
	SetNameserversAPI  = setNameserversAPI
	ReadNameserversAPI = readNameserversAPI

	// Rate limiter defaults.
	DefaultPerSecondLimit = defaultPerSecondLimit
	DefaultPerHourLimit   = defaultPerHourLimit
)
