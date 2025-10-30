# Contributing to Enhanced PostgreSQL

Thank you for your interest in contributing to the Enhanced PostgreSQL project! This guide provides everything you need to know about developing, testing, and contributing to the project.

## Table of Contents

- [Development Setup](#development-setup)
- [Project Architecture](#project-architecture)
- [Building from Source](#building-from-source)
- [Task Commands](#task-commands)
- [Make Commands](#make-commands)
- [Testing](#testing)
- [CI/CD](#cicd)
- [CLI Development](#cli-development)
- [Configuration Development](#configuration-development)
- [Contributing Guidelines](#contributing-guidelines)

## Development Setup

### Prerequisites

1. **Docker** (20.10+)
2. **Go** (1.21+)
3. **Task** (Taskfile runner)
4. **Make**
5. **Helm** (3.0+) for chart development

### Install Development Tools

```bash
# Install Task (recommended)
curl -sL https://taskfile.dev/install.sh | sh

# macOS via Homebrew
brew install go-task/tap/go-task

# Install Go dependencies
go mod download

# Set up development environment
task dev-setup
```

## Project Architecture

### Directory Structure

```
.
├── cmd/                    # CLI application
│   ├── main.go            # Main entry point
│   ├── server.go          # Health check server
│   ├── schema.go          # Schema generation
│   └── config.go          # Configuration management
├── pkg/                    # Core libraries
│   ├── embedded/          # Embedded PostgreSQL
│   ├── extensions/        # Extension management
│   ├── generators/        # Config generators
│   ├── health/           # Health checks
│   ├── installer/        # Binary installers
│   ├── schemas/          # JSON schemas
│   ├── server/           # Server components
│   └── utils/            # Utilities
├── chart/                  # Helm chart
│   ├── templates/        # K8s templates
│   ├── values.yaml       # Default values
│   └── Chart.yaml        # Chart metadata
├── docker/                # Docker files
│   ├── Dockerfile        # Main image
│   └── scripts/          # Container scripts
├── test/                  # Test files
│   ├── helm_test.go      # Helm tests
│   └── integration/      # Integration tests
└── Taskfile*.yaml         # Task definitions
```

### How Upgrades Work

The upgrade process:

1. **Detection**: Identifies current PostgreSQL version from data directory
2. **Planning**: Determines upgrade path (e.g., 14→15→16→17→18)
3. **Execution**: Runs pg_upgrade with hard links for efficiency
4. **Verification**: Validates upgraded database
5. **Cleanup**: Preserves old data with `.old` suffix

### Extension System

Extensions are managed through:

1. **Installation**: Pre-compiled binaries in `/usr/lib/postgresql/*/lib`
2. **Configuration**: Environment variable `POSTGRES_EXTENSIONS`
3. **Initialization**: SQL scripts in `/docker-entrypoint-initdb.d`
4. **Validation**: Health checks for each extension

## Building from Source

### Build Docker Images

```bash
# Build all images
task build-all

# Build specific version
task build:build-18    # PostgreSQL 18
task build:build-17    # PostgreSQL 17
task build:build-16    # PostgreSQL 16
task build:build-15    # PostgreSQL 15

# Build with custom registry
REGISTRY=myregistry.io task build-all
```

### Build CLI Tool

```bash
# Build and install to GOBIN
task cli-build

# Build locally
go build -o postgres-cli ./cmd

# Install to system
task cli-install
```

### Build Helm Chart

```bash
# Package chart
helm package chart/

# Test chart
helm install test ./chart --dry-run --debug
```

## Task Commands

Task is the primary development interface. All commands support detailed help with `--summary`.

### Core Development Tasks

```bash
# Default task (auto-upgrade)
task                    # Run default PostgreSQL upgrade

# Development workflow
task dev-setup          # Set up development environment
task dev-test-quick     # Quick test (14→17 only)
task clean              # Clean up everything
```

### Build Tasks

```bash
# Docker images
task build              # Build default image
task build-all          # Build all version images

# Individual versions
task build:build-15     # PostgreSQL 15 image
task build:build-16     # PostgreSQL 16 image
task build:build-17     # PostgreSQL 17 image
task build:build-18     # PostgreSQL 18 image

# Push to registry
task build:push-all     # Push all images
task build:push-17      # Push specific version
```

### Test Tasks

```bash
# Run tests
task test               # Run all tests
task test-image         # Test Docker image
task test:all           # Comprehensive test suite

# Specific test paths
task test:upgrade-14-to-18
task test:upgrade-15-to-18
task test:upgrade-16-to-18
task test:upgrade-17-to-18

# Test management
task test:seed-all      # Seed test data
task test:clean         # Clean test artifacts
task test:status        # Show test status
```

### CLI Development Tasks

```bash
# CLI building
task cli-build          # Build and install CLI
task cli-test           # Run CLI tests
task cli-ci             # Full CLI CI pipeline

# Schema tasks
task generate-schema    # Generate JSON schema
task validate-schema    # Validate schema files
```

### Service Management Tasks

```bash
# Extension management
task extensions-list    # List available extensions
task extensions-install EXTENSIONS=pgvector,pgaudit

# Service monitoring
task services-status    # Show all service status
task services-logs SERVICE=postgresql

# Backup operations
task backup-create      # Create WAL-G backup
task backup-list        # List backups
task backup-restore BACKUP_NAME=backup-20231201T120000Z
```

## Make Commands

Traditional Makefile interface for CI/CD compatibility:

### Build Commands

```bash
make build              # Build default image
make build-15           # Build PostgreSQL 15
make build-16           # Build PostgreSQL 16
make build-17           # Build PostgreSQL 17
make build-18           # Build PostgreSQL 18
make build-all          # Build all versions

# Custom registry
REGISTRY=myregistry.io IMAGE_TAG=v1.0.0 make build-all
```

### Push Commands

```bash
make push-15            # Push PostgreSQL 15
make push-16            # Push PostgreSQL 16
make push-17            # Push PostgreSQL 17
make push-18            # Push PostgreSQL 18
make push-all           # Push all images

# Custom registry
REGISTRY=ghcr.io IMAGE_BASE=myorg/postgres make push-all
```

### Test Commands

```bash
make test               # Run all tests
make test-simple        # Quick tests
make test-compose       # Docker Compose tests
make test-all           # Comprehensive suite

# Specific upgrade paths
make test-14-to-15
make test-14-to-16
make test-14-to-17
make test-14-to-18
make test-15-to-16
make test-15-to-17
make test-15-to-18
make test-16-to-17
make test-16-to-18
make test-17-to-18
```

### CLI Commands

```bash
make cli-build          # Build CLI tool
make cli-install        # Install to GOBIN
make cli-test           # Run CLI tests
make cli-ci             # Full CI pipeline
make cli-all            # Build, test, install
```

## Testing

### Unit Tests

```bash
# Run all unit tests
go test ./pkg/...

# Run specific package tests
go test ./pkg/extensions -v
go test ./pkg/generators -v

# Run with coverage
go test ./pkg/... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Integration Tests

```bash
# Run integration tests
task test:integration

# Run specific integration test
go test ./test -run TestPostgresIntegration

# Run with specific PostgreSQL version
PG_VERSION=16 go test ./test -run TestPostgresUpgrade
```

### Helm Chart Tests

```bash
# Run Helm tests
go test ./test -run TestHelm

# Test with Ginkgo
ginkgo ./test

# Test specific scenarios
go test ./test -run TestHelmBasic
go test ./test -run TestHelmUpgrade
```

### Test Environment Variables

```bash
# Control test behavior
export GO_TEST_FLAGS="-v -race"
export SKIP_CLEANUP=true
export TEST_TIMEOUT=30m
export PG_VERSION=17
```

### Writing Tests

Example test structure:

```go
func TestPostgresUpgrade(t *testing.T) {
    // Setup
    container := setupTestContainer(t)
    defer cleanupContainer(container)

    // Test upgrade
    err := container.Upgrade("14", "17")
    require.NoError(t, err)

    // Verify
    version := container.GetVersion()
    assert.Equal(t, "17", version)
}
```

## CI/CD

### GitHub Actions Workflows

#### build-push.yml
Builds and pushes Docker images:

```yaml
on:
  push:
    tags: ['v*']
  workflow_dispatch:

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: docker/setup-buildx-action@v2
      - uses: docker/login-action@v2
      - run: make build-all push-all
```

#### test.yml
Runs comprehensive tests:

```yaml
on:
  pull_request:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        from: [14, 15, 16, 17]
        to: [15, 16, 17, 18]
    steps:
      - uses: actions/checkout@v3
      - run: task test:upgrade-${{ matrix.from }}-to-${{ matrix.to }}
```

#### helm.yml
Tests and publishes Helm charts:

```yaml
on:
  push:
    paths: ['chart/**']

jobs:
  helm:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: azure/setup-helm@v3
      - run: helm lint chart/
      - run: helm package chart/
      - run: helm push *.tgz oci://ghcr.io/flanksource/charts
```

### Local CI Testing

```bash
# Test like CI
GITHUB_ACTIONS=true task test

# With output grouping
TASK_OUTPUT=group task test

# Run specific CI workflow locally
act -j test
```

### Release Process

1. **Tag Release**
```bash
git tag v1.2.3
git push origin v1.2.3
```

2. **Manual Publishing**
```bash
REGISTRY=ghcr.io IMAGE_TAG=v1.2.3 make push-all
```

3. **Helm Chart Release**
```bash
helm package chart/
helm push postgres-upgrade-*.tgz oci://ghcr.io/flanksource/charts
```

## CLI Development

### Adding New Commands

1. Create command file in `cmd/`:
```go
// cmd/newcmd.go
package main

import (
    "github.com/spf13/cobra"
)

func createNewCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "newcmd",
        Short: "Description of new command",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Implementation
            return nil
        },
    }
}
```

2. Register in main.go:
```go
rootCmd.AddCommand(createNewCommand())
```

3. Add tests:
```go
func TestNewCommand(t *testing.T) {
    cmd := createNewCommand()
    err := cmd.Execute()
    assert.NoError(t, err)
}
```

### CLI Architecture

- **cobra**: Command-line framework
- **viper**: Configuration management
- **logrus**: Structured logging
- **go-jsonschema**: Schema generation

### Building and Testing CLI

```bash
# Build
task cli-build

# Test
task cli-test

# Run locally
go run ./cmd --help

# Install globally
go install ./cmd
```

## Configuration Development

### Schema Generation

The project uses JSON Schema for configuration validation:

```bash
# Generate schema from PostgreSQL
task generate-schema

# Validate schema
task validate-schema

# Generate Go structs from schema
go generate ./pkg/schemas
```

### Adding Configuration Options

1. Update schema definition:
```json
{
  "newOption": {
    "type": "string",
    "description": "New configuration option",
    "default": "value"
  }
}
```

2. Regenerate structs:
```bash
task generate-schema
```

3. Implement in code:
```go
func applyConfig(cfg *Config) {
    if cfg.NewOption != "" {
        // Apply configuration
    }
}
```

### Configuration Validation

```go
func validateConfig(cfg *Config) error {
    validator := gojsonschema.NewGoLoader(cfg)
    schema := gojsonschema.NewStringLoader(schemaJSON)
    result, err := gojsonschema.Validate(schema, validator)
    if err != nil {
        return err
    }
    if !result.Valid() {
        return fmt.Errorf("validation errors: %v", result.Errors())
    }
    return nil
}
```

## Contributing Guidelines

### Code Standards

1. **Go Code**
   - Follow [Effective Go](https://golang.org/doc/effective_go.html)
   - Use `gofmt` and `golint`
   - Write tests for new features
   - Maintain >80% test coverage

2. **Docker**
   - Use multi-stage builds
   - Minimize layers
   - Pin versions explicitly
   - Add HEALTHCHECK instructions

3. **Shell Scripts**
   - Use `shellcheck` for linting
   - Add error handling (`set -euo pipefail`)
   - Document complex logic
   - Use consistent style

### Pull Request Process

1. **Fork and Clone**
```bash
git clone https://github.com/yourusername/postgres.git
cd postgres
git remote add upstream https://github.com/flanksource/postgres.git
```

2. **Create Feature Branch**
```bash
git checkout -b feature/my-feature
```

3. **Make Changes**
```bash
# Development
task dev-setup
# Make changes
# Test
task test
```

4. **Commit with Conventional Commits**
```bash
git add .
git commit -m "feat: add new feature

- Detail 1
- Detail 2

Closes #123"
```

5. **Push and Create PR**
```bash
git push origin feature/my-feature
# Create PR on GitHub
```

### Commit Message Format

Follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation
- `style:` Formatting
- `refactor:` Code restructuring
- `perf:` Performance improvement
- `test:` Testing
- `chore:` Maintenance

### Testing Requirements

All PRs must:
- Pass all existing tests
- Add tests for new features
- Maintain or improve coverage
- Pass CI checks

### Documentation

Update documentation for:
- New features
- Changed behavior
- New configuration options
- API changes

## Development Workflow Examples

### Adding a New Extension

1. Add extension to Dockerfile:
```dockerfile
RUN apt-get install -y postgresql-18-newext
```

2. Update extension list:
```go
// pkg/extensions/list.go
var AvailableExtensions = []Extension{
    {Name: "newext", Description: "New extension"},
}
```

3. Add initialization script:
```bash
# docker/scripts/init-newext.sh
if [[ "$POSTGRES_EXTENSIONS" == *"newext"* ]]; then
    psql -c "CREATE EXTENSION IF NOT EXISTS newext;"
fi
```

4. Add tests:
```go
func TestNewExtension(t *testing.T) {
    // Test extension installation
}
```

### Debugging Tips

1. **Container Debugging**
```bash
# Interactive shell
docker run -it --entrypoint /bin/bash image:tag

# Debug mode
docker run -e DEBUG=true image:tag

# Verbose logging
docker run -e LOG_LEVEL=debug image:tag
```

2. **Test Debugging**
```bash
# Run single test with verbose output
go test -v -run TestSpecific ./pkg/...

# Debug with delve
dlv test ./pkg/extensions

# Keep test containers
SKIP_CLEANUP=true task test
```

3. **Performance Profiling**
```bash
# CPU profiling
go test -cpuprofile=cpu.prof ./pkg/...
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof ./pkg/...
go tool pprof mem.prof
```

## Getting Help

- **Documentation**: Check this guide and README
- **Issues**: Search existing [GitHub Issues](https://github.com/flanksource/postgres/issues)
- **Discussions**: Join [GitHub Discussions](https://github.com/flanksource/postgres/discussions)
- **Security**: Report to security@flanksource.com

## License

By contributing, you agree that your contributions will be licensed under Apache License 2.0.