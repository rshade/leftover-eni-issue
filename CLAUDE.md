# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands
- Build: `pulumi preview`
- Deploy: `pulumi up`
- Test: `jest --testMatch='**/tests/**/*.spec.ts'`
- Test single: `jest --testMatch='**/tests/**/filename.spec.ts'`
- Lint: `eslint . --ext .ts`
- Format: `prettier --write "**/*.ts"`

## Guidelines
- **Imports**: Group imports by: Pulumi AWS, Pulumi core, other libraries, relative imports
- **Naming**: Use camelCase for variables, PascalCase for classes/interfaces 
- **Resources**: Use descriptive names with resource type suffix (e.g., `webServerSecurityGroup`)
- **Stacks**: Organize by environment (dev/staging/prod) with consistent structure
- **Error Handling**: Use try/catch blocks for deployments, include specific error messages
- **Type Safety**: Use explicit TypeScript types, avoid `any`
- **Comments**: Document complex infrastructure relationships and decision rationale
- **Secrets**: Never hardcode credentials, use Pulumi config with `--secret` flag
- **ENI Cleanup**: Use destroy-time hooks with pulumi-command to clean up orphaned ENIs before resource destruction

## Project Todo List (COMPLETED)
1. ✅ Set up basic Pulumi TypeScript project structure
2. ✅ Set up basic Pulumi Python project structure
3. ✅ Set up basic Pulumi Go project structure
4. ✅ Implement TypeScript destroy-time ENI cleanup
5. ✅ Implement Python destroy-time ENI cleanup
6. ✅ Implement Go destroy-time ENI cleanup
7. ✅ Create pre-destroy hook using pulumi-command
8. ✅ Add automated testing for the cleanup process
9. ✅ Add cross-language documentation and implementation details