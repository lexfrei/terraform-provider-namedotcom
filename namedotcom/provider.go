package namedotcom

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namedotcom/go/v4/namecom"
)

const (
	// providerTypeName is the provider name used in Terraform configurations.
	providerTypeName = "namedotcom"

	// Default rate limiting values.
	defaultRateLimitPerSecond = 20
	defaultRateLimitPerHour   = 3000
	defaultTimeoutSeconds     = 120
)

// Ensure the provider satisfies the framework interface.
var _ provider.Provider = (*nameDotComProvider)(nil)

// nameDotComProvider is the Name.com provider implementation.
type nameDotComProvider struct {
	// version is set to the provider version on release, "dev" otherwise.
	version string
}

// New returns a provider factory wired with the build version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &nameDotComProvider{version: version}
	}
}

// providerModel maps the provider configuration block to a Go struct.
type providerModel struct {
	Username           types.String `tfsdk:"username"`
	Token              types.String `tfsdk:"token"`
	RateLimitPerSecond types.Int64  `tfsdk:"rate_limit_per_second"`
	RateLimitPerHour   types.Int64  `tfsdk:"rate_limit_per_hour"`
	Timeout            types.Int64  `tfsdk:"timeout"`
}

func (p *nameDotComProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = providerTypeName
	resp.Version = p.version
}

func (p *nameDotComProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			keyUsername: schema.StringAttribute{
				Optional:    true,
				Description: "Name.com API Username; can alternatively be specified via the `NAMEDOTCOM_USERNAME` environment variable.",
			},
			keyToken: schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Name.com API Token Value; can alternatively be specified via the `NAMEDOTCOM_TOKEN` environment variable.",
			},
			keyRateLimitPerSecond: schema.Int64Attribute{
				Optional:    true,
				Description: "Maximum number of API requests per second. Defaults to 20.",
			},
			keyRateLimitPerHour: schema.Int64Attribute{
				Optional:    true,
				Description: "Maximum number of API requests per hour. Defaults to 3000.",
			},
			keyTimeout: schema.Int64Attribute{
				Optional:    true,
				Description: "Timeout in seconds for API requests. Defaults to 120 seconds.",
			},
		},
	}
}

func (p *nameDotComProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A credential that is unknown until apply cannot be used to build the
	// client now; surface a clear error instead of misreporting it as missing.
	if cfg.Username.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root(keyUsername),
			"Unknown Name.com API Username",
			"The username cannot depend on a value that is unknown until apply. "+
				"Set it to a known value or use the NAMEDOTCOM_USERNAME environment variable.",
		)
	}

	if cfg.Token.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root(keyToken),
			"Unknown Name.com API Token",
			"The token cannot depend on a value that is unknown until apply. "+
				"Set it to a known value or use the NAMEDOTCOM_TOKEN environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve credentials, falling back to environment variables (replacing the
	// SDKv2 EnvDefaultFunc behaviour, which the framework does not provide).
	username, token, missing := resolveCredentials(cfg.Username, cfg.Token)

	for _, field := range missing {
		resp.Diagnostics.AddAttributeError(
			path.Root(field),
			"Missing Name.com API credential",
			"The "+field+" must be set via the provider configuration or the corresponding "+
				"NAMEDOTCOM_"+upperEnvName(field)+" environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	client := buildClient(username, token, cfg.RateLimitPerSecond, cfg.RateLimitPerHour, cfg.Timeout)

	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *nameDotComProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRecordResource,
		NewDomainNameServersResource,
		NewDNSSECResource,
	}
}

func (p *nameDotComProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

// resolveCredentials returns the effective username/token, preferring the
// configuration value and falling back to the environment. It reports any
// credential that ends up empty so the caller can raise diagnostics.
func resolveCredentials(cfgUsername, cfgToken types.String) (username, token string, missing []string) {
	username = cfgUsername.ValueString()
	if cfgUsername.IsNull() {
		username = os.Getenv("NAMEDOTCOM_USERNAME")
	}

	token = cfgToken.ValueString()
	if cfgToken.IsNull() {
		token = os.Getenv("NAMEDOTCOM_TOKEN")
	}

	if username == "" {
		missing = append(missing, keyUsername)
	}

	if token == "" {
		missing = append(missing, keyToken)
	}

	return username, token, missing
}

// upperEnvName maps a schema key to the suffix used in its environment variable.
func upperEnvName(field string) string {
	switch field {
	case keyUsername:
		return "USERNAME"
	case keyToken:
		return "TOKEN"
	default:
		return field
	}
}

// buildClient constructs a configured Name.com client and initializes the
// global rate limiters. It is kept free of framework plumbing so it can be
// unit-tested directly.
func buildClient(username, token string, perSecond, perHour, timeout types.Int64) *namecom.NameCom {
	perSecondLimit := defaultRateLimitPerSecond
	if !perSecond.IsNull() {
		perSecondLimit = int(perSecond.ValueInt64())
	}

	perHourLimit := defaultRateLimitPerHour
	if !perHour.IsNull() {
		perHourLimit = int(perHour.ValueInt64())
	}

	InitRateLimiters(perSecondLimit, perHourLimit)

	client := namecom.New(username, token)

	timeoutSeconds := defaultTimeoutSeconds
	if !timeout.IsNull() {
		timeoutSeconds = int(timeout.ValueInt64())
	}

	client.Client.Timeout = time.Duration(timeoutSeconds) * time.Second

	return client
}

// configureClient extracts the *namecom.NameCom client from the data the
// provider passes to each resource's Configure method. It returns false (and
// raises no error) when ProviderData is nil, which is the normal state before
// the provider's own Configure has run.
func configureClient(providerData any, diags *diag.Diagnostics) (*namecom.NameCom, bool) {
	if providerData == nil {
		return nil, false
	}

	client, ok := providerData.(*namecom.NameCom)
	if !ok {
		diags.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *namecom.NameCom, got %T. This is a bug in the provider.", providerData),
		)

		return nil, false
	}

	return client, true
}
