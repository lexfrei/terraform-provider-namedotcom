# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Terraform provider for Name.com DNS and domain management. The provider allows managing DNS records, nameservers, and DNSSEC settings through Terraform infrastructure as code.

**Key Architecture:**
- Built using Terraform Plugin SDK v2
- Uses Name.com Go SDK v4 for API interactions
- Implements built-in rate limiting (20 req/sec, 3000 req/hour)
- Provider entry point: `main.go` with actual implementation in `namedotcom/` package

## Development Commands

### Building and Testing
```bash
# Build the provider
go build -o terraform-provider-namedotcom

# Install dependencies
go mod tidy

# Run linter (uses golangci-lint with custom config)
golangci-lint run --timeout 5m

# Check Go module status
go mod verify
```

### Code Quality
- **Linting**: Uses golangci-lint with comprehensive configuration in `.golangci.yaml`
- **Formatting**: Uses gofmt, gofumpt, and goimports
- **Security**: CodeQL analysis enabled in CI
- **Markdown**: Uses markdown-lint for documentation

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