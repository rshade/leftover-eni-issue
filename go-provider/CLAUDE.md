# CLAUDE.md for go-provider

This file provides guidance to Claude Code (claude.ai/code) when working with code in this go-provider directory.

## Commands
- Build: `go build ./...`
- Test: `go test ./...`
- Lint: `golangci-lint run`
- Format: `gofmt -w .`

## Guidelines
- **Imports**: Group imports by standard library, third-party libraries, then project imports
- **Naming**: Use camelCase for variables, PascalCase for exported functions/types
- **Resources**: Use descriptive names with resource type suffix
- **Error Handling**: Always check errors and provide context in error messages
- **Comments**: Document exported functions and types with godoc-style comments
- **Secrets**: Never hardcode credentials
- **ENI Cleanup**: Implement destroy-time hooks to clean up orphaned ENIs
- **Fallback Mechanisms**: Implement multiple fallback strategies when ENI deletion fails:
  1. Security group disassociation: Remove security group associations and retry deletion
  2. Tagging for manual review: Tag ENIs that couldn't be automatically cleaned up
  3. Comprehensive error handling with detailed logs