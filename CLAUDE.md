# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Terraform provider for Name.com DNS and domain management. The provider allows managing DNS records, nameservers, and DNSSEC settings through Terraform infrastructure as code.

**Key Architecture:**

- Built using Terraform Plugin SDK v2 with modern patterns
- Uses Name.com Go SDK v4 for API interactions
- Implements built-in rate limiting (20 req/sec, 3000 req/hour)
- Provider entry point: `main.go` with actual implementation in `namedotcom/` package
- Uses Go 1.24+ with modern language features
- Implements context-aware provider configuration with `ConfigureContextFunc`
- Uses `diag.Diagnostics` for improved error reporting

## Development Commands

### Building and Testing

```bash
# Build the provider
go build -o terraform-provider-namedotcom

# Install dependencies
go mod tidy

# Run tests (standard)
go test -v ./...

# Run tests with race detection
go test -v -race ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...

# Run linter (uses golangci-lint with custom config)
golangci-lint run --timeout 5m

# Check Go module status
go mod verify

# Test GitHub Actions workflows locally using act
act -j test
act -j golangci-lint
act -j lint-markdown
```

### Code Quality

- **Linting**: Uses golangci-lint with comprehensive configuration in `.golangci.yaml`
- **Formatting**: Uses gofmt and goimports (gofumpt disabled due to compatibility issues)
- **Security**: CodeQL analysis enabled in CI
- **Markdown**: Uses markdown-lint for documentation with `.markdown-lint.yaml` config
- **Testing**: Comprehensive test suite with race detection enabled

## Code Structure

### Core Files

- `main.go`: Provider entry point using Terraform Plugin SDK
- `namedotcom/provider.go`: Provider schema and configuration
- `namedotcom/ratelimit.go`: API rate limiting implementation
- `namedotcom/resource_*.go`: Individual resource implementations

### Resources Implemented

- `namedotcom_record`: DNS record management (A, AAAA, CNAME, MX, NS, SRV, TXT)
- `namedotcom_domain_nameservers`: Domain nameserver configuration
- `namedotcom_dnssec`: DNSSEC settings management

### Authentication

The provider requires Name.com API credentials:

- `NAMEDOTCOM_USERNAME` environment variable or provider config
- `NAMEDOTCOM_TOKEN` environment variable or provider config

## Release Process

### GoReleaser Configuration

- Builds for multiple platforms: linux, darwin, windows, freebsd
- Architectures: amd64, 386, arm, arm64
- Artifacts are signed with GPG key: F57F85FC7975F22BBC3F25049C173EB1B531AA1F
- Registry manifest included for Terraform Registry

### Version Management

- Follows semantic versioning
- Current version referenced in documentation: 1.1.6
- Release notes generated automatically from commit messages

### CI/CD Pipeline

The project uses GitHub Actions with multiple jobs:

- **test**: Runs Go tests with race detection and uploads coverage to Codecov
- **golangci-lint**: Runs comprehensive linting with golangci-lint
- **lint-markdown**: Validates markdown files using markdown-lint
- **codeql-analyze**: Performs security analysis with GitHub CodeQL

All jobs run on `ubuntu-latest` with Go stable version.

## Documentation Structure

Terraform provider documentation is in `docs/` following standard format:

- `docs/index.md`: Provider configuration
- `docs/resources/`: Individual resource documentation

## Rate Limiting Implementation

The provider includes sophisticated rate limiting to respect Name.com API limits:

- Per-second limiting (default: 20 requests/second)
- Per-hour limiting (default: 3000 requests/hour)
- Configurable via provider settings
- Implementation in `namedotcom/ratelimit.go`

## Import Functionality

Resources support Terraform import:

- DNS records: `domain_name:record_id` format
- DNSSEC: `domain_name` format
- Use Name.com API to find record IDs for imports

## Testing Strategy

### Test Structure

The project includes comprehensive test coverage:

- **Provider tests** (`namedotcom/provider_test.go`): Configuration validation, authentication, rate limiting setup
- **Rate limiter tests** (`namedotcom/ratelimit_test.go`): Rate limiting functionality, thread safety, context handling
- **Resource tests** (`namedotcom/resource_test.go`): Schema validation, CRUD operations verification

### Test Execution Guidelines

- **Parallel execution**: Most tests run in parallel using `t.Parallel()` for efficiency
- **Sequential execution**: Provider and rate limiter tests run sequentially due to global state dependencies
- **Race detection**: Always run tests with `-race` flag to catch concurrency issues
- **Coverage**: Target comprehensive coverage of core functionality

### Race Condition Prevention

- Global rate limiter state requires sequential test execution
- Use `//nolint:paralleltest` at package level for affected test files
- Avoid `t.Parallel()` in tests that modify shared global variables

### Test Patterns

- Use `t.Helper()` in test utility functions
- Break complex tests into smaller helper functions
- Use table-driven tests for multiple scenarios
- Mock external dependencies where possible

### Local CI Testing

Use `act` to test GitHub Actions workflows locally:

```bash
# Test all workflows
act

# Test specific jobs
act -j test
act -j golangci-lint
act -j lint-markdown
act -j codeql-analyze
```

### Terraform Provider Testing Best Practices

- Test schema definitions and validation
- Verify CRUD operations exist and are properly configured
- Check import functionality
- Test error handling and edge cases
- Use `schema.TestResourceDataRaw()` for provider configuration tests

## Modern Go and Terraform SDK Patterns

### Provider Configuration

The provider uses modern Terraform Plugin SDK v2 patterns:

- **ConfigureContextFunc**: Context-aware configuration instead of deprecated `ConfigureFunc`
- **diag.Diagnostics**: Rich error reporting with severity levels and detailed messages
- **Context propagation**: Proper context handling throughout the provider lifecycle

### Go Language Features

Current implementation uses Go 1.24+ features:

- **Range over integers**: `for range numGoroutines` syntax
- **Context-aware APIs**: All API calls support context cancellation
- **Error wrapping**: Using `errors.New()` and proper error chains
- **Type assertions**: Safe type checking with comma ok idiom

### Linting Configuration

The `.golangci.yaml` configuration follows modern Go practices:

- **Comprehensive linters**: Enables most linters with specific exclusions
- **Disabled problematic linters**: `gofumpt`, `whitespace` disabled due to formatting conflicts
- **Security focus**: Includes security-focused linters like `gosec`
- **Performance awareness**: Enables performance-related checks

### Dependencies Management

- **Go modules**: Using `go.mod` with Go 1.24+ requirement
- **Toolchain specification**: Explicit `toolchain go1.25.1` declaration
- **Version pinning**: Direct and indirect dependencies properly managed
- **Vulnerability scanning**: Automated dependency security checks
