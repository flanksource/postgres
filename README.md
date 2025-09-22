# Enhanced PostgreSQL Distribution with Extensions

This repository provides a comprehensive PostgreSQL distribution with automatic upgrade capabilities and popular extensions. Based on Supabase's enhanced PostgreSQL, it includes pgvector, pgsodium, PostgREST, PgBouncer, WAL-G backup, and many other extensions commonly used in modern applications.

## Table of Contents

- [Features](#features)
- [Available Images](#available-images)
- [Quick Start](#quick-start)
- [CLI Commands Overview](#cli-commands-overview)
- [Task Commands](#task-commands)
- [Make Commands](#make-commands)
- [pgconfig CLI Tool](#pgconfig-cli-tool)
- [Docker Usage](#docker-usage)
- [Extension Management](#extension-management)
- [Service Management](#service-management)
- [Backup and Recovery](#backup-and-recovery)
- [Testing](#testing)
- [Configuration](#configuration)
- [Development](#development)
- [CI/CD](#cicd)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)

## Features

- **Automatic PostgreSQL upgrades** from versions 14, 15, or 16 to 17
- **16 pre-compiled PostgreSQL extensions** including pgvector, pgsodium, pgjwt, and more
- **Connection pooling** with PgBouncer
- **REST API** generation with PostgREST
- **Backup and restore** with WAL-G
- **Process supervision** with s6-overlay
- **Kubernetes-ready** with Helm charts

## Available Images

The images are published to GitHub Container Registry with version-specific tags:

### Enhanced PostgreSQL with Extensions
- `ghcr.io/flanksource/postgres:17-latest` - Enhanced PostgreSQL 17 with all extensions
- `ghcr.io/flanksource/postgres:16-latest` - Enhanced PostgreSQL 16 with all extensions
- `ghcr.io/flanksource/postgres:latest` - Points to enhanced PostgreSQL 17

### Standard PostgreSQL (Upgrade Only)
- `ghcr.io/flanksource/postgres:16` - Standard PostgreSQL 16 for upgrade testing
- `ghcr.io/flanksource/postgres:17` - Standard PostgreSQL 17 for upgrade testing

### Legacy Tags (Still Supported)
- `ghcr.io/flanksource/postgres-upgrade:to-15` - Upgrades to PostgreSQL 15
- `ghcr.io/flanksource/postgres-upgrade:to-16` - Upgrades to PostgreSQL 16
- `ghcr.io/flanksource/postgres-upgrade:to-17` - Upgrades to PostgreSQL 17

## How It Works

1. The container automatically detects the PostgreSQL version in the mounted data directory
2. It performs sequential upgrades if needed (e.g., 14→15→16→17)
3. The upgrade uses `pg_upgrade` with hard links for efficiency
4. Original data is preserved with `.old` suffix on control files

## Usage

### Basic Usage

To upgrade a PostgreSQL data directory:

```bash
docker run --rm \
  -v /path/to/your/pgdata:/var/lib/postgresql/data \
  ghcr.io/flanksource/postgres-upgrade:to-17
```

### Docker Compose Example

```yaml
version: '3.8'

services:
  postgres-upgrade:
    image: ghcr.io/flanksource/postgres-upgrade:to-17
    volumes:
      - postgres_data:/var/lib/postgresql/data
    profiles:
      - upgrade

volumes:
  postgres_data:
```

Run the upgrade with:
```bash
docker-compose run --rm postgres-upgrade
```

### Kubernetes Job Example

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: postgres-upgrade-to-17
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: upgrade
        image: ghcr.io/flanksource/postgres-upgrade:to-17
        volumeMounts:
        - name: postgres-data
          mountPath: /var/lib/postgresql/data
      volumes:
      - name: postgres-data
        persistentVolumeClaim:
          claimName: postgres-pvc
```

## Supported Upgrade Paths

| From Version | To Version 15 | To Version 16 | To Version 17 |
|--------------|---------------|---------------|---------------|
| PostgreSQL 14 | ✅ | ✅ | ✅ |
| PostgreSQL 15 | - | ✅ | ✅ |
| PostgreSQL 16 | - | - | ✅ |

## PostgreSQL Extensions

The enhanced images include 16 pre-compiled PostgreSQL extensions commonly used in modern applications:

### Available Extensions

| Extension | Description | Use Case |
|-----------|-------------|----------|
| **pgvector** | Vector similarity search | AI/ML, embeddings, semantic search |
| **pgsodium** | Modern cryptography | Encryption, key management |
| **pgjwt** | JSON Web Token support | Authentication, API security |
| **pgaudit** | Audit logging | Compliance, security monitoring |
| **pg_tle** | Trusted Language Extensions | Safe extension development |
| **pg_stat_monitor** | Query performance monitoring | Performance optimization |
| **pg_repack** | Online table reorganization | Maintenance, space reclamation |
| **pg_plan_filter** | Query plan filtering | Query optimization |
| **pg_net** | Async HTTP requests | Webhooks, API integration |
| **pg_jsonschema** | JSON schema validation | Data validation |
| **pg_hashids** | Short unique ID generation | URL shortening, obfuscation |
| **pg_cron** | Job scheduler | Background tasks, maintenance |
| **pg-safeupdate** | Require WHERE in DELETE/UPDATE | Data safety |
| **index_advisor** | Index recommendations | Performance tuning |
| **wal2json** | WAL to JSON converter | Change data capture, replication |

### Using Extensions

#### Environment Variable Configuration

Enable extensions using a comma-separated list:

```bash
docker run -d \
  -e POSTGRES_EXTENSIONS="pgvector,pgaudit,pg_cron" \
  -e POSTGRES_PASSWORD=mypassword \
  ghcr.io/flanksource/postgres:17-latest
```

#### Helm Chart Configuration

```yaml
extensions:
  enabled: "pgvector,pgsodium,pgjwt,pgaudit,pg_cron"

  # Extension-specific configuration
  pgaudit:
    enabled: true
    log: "all"
    log_level: "notice"

  pg_cron:
    enabled: true
    database_name: "postgres"
```

#### Manual Installation

```bash
# List available extensions
task extensions-list

# Install extensions in running container
task extensions-install EXTENSIONS=pgvector,pgaudit,pg_cron

# Check extension health
/scripts/extension-health.sh
```

### Extension Examples

#### pgvector (Vector Similarity Search)

```sql
-- Create a table with vector column
CREATE TABLE items (id SERIAL PRIMARY KEY, embedding VECTOR(3));

-- Insert vectors
INSERT INTO items (embedding) VALUES ('[1,2,3]'), ('[4,5,6]');

-- Find similar vectors
SELECT * FROM items ORDER BY embedding <-> '[3,1,2]' LIMIT 5;
```

#### pg_cron (Job Scheduler)

```sql
-- Schedule a job to run every minute
SELECT cron.schedule('my-job', '* * * * *', 'DELETE FROM logs WHERE created_at < NOW() - INTERVAL ''1 day'';');

-- List scheduled jobs
SELECT * FROM cron.job;

-- Unschedule a job
SELECT cron.unschedule('my-job');
```

#### pgsodium (Encryption)

```sql
-- Generate a key pair
SELECT * FROM pgsodium.crypto_box_keypair();

-- Encrypt data
SELECT pgsodium.crypto_secretbox('Hello, World!', 'my-secret-key');
```

## Additional Services

### PgBouncer (Connection Pooling)

Enable PgBouncer for connection pooling:

```bash
docker run -d \
  -e PGBOUNCER_ENABLED=true \
  -e PGBOUNCER_POOL_MODE=transaction \
  -e PGBOUNCER_MAX_CLIENT_CONN=100 \
  -p 6432:6432 \
  ghcr.io/flanksource/postgres:17-latest
```

Connect through PgBouncer:
```bash
psql -h localhost -p 6432 -U postgres
```

### PostgREST (REST API)

Enable automatic REST API generation:

```bash
docker run -d \
  -e POSTGREST_ENABLED=true \
  -e POSTGREST_DB_SCHEMAS=public \
  -p 3000:3000 \
  ghcr.io/flanksource/postgres:17-latest
```

Access your database via REST API:
```bash
# GET all records from a table
curl http://localhost:3000/users

# POST new record
curl -X POST http://localhost:3000/users \
  -H "Content-Type: application/json" \
  -d '{"name": "John", "email": "john@example.com"}'
```

### WAL-G (Backup & Recovery)

Enable continuous backup with WAL-G:

```bash
docker run -d \
  -e WALG_ENABLED=true \
  -e WALG_S3_PREFIX=s3://my-bucket/postgres-backups \
  -e AWS_ACCESS_KEY_ID=your-access-key \
  -e AWS_SECRET_ACCESS_KEY=your-secret-key \
  ghcr.io/flanksource/postgres:17-latest
```

Backup operations:
```bash
# Create backup
task backup-create

# List backups
task backup-list

# Restore from backup
task backup-restore BACKUP_NAME=backup-20231201T120000Z
```

## Service Management

### Health Checks

Check the health of all services:

```bash
# Overall service health
/scripts/service-health.sh

# Extension health
/scripts/extension-health.sh

# Task-based health checks
task check-service-health
```

### Service Status

Monitor s6-overlay services:

```bash
# Show service status
task services-status

# View service logs
task services-logs SERVICE=postgresql
task services-logs SERVICE=pgbouncer
task services-logs SERVICE=postgrest
```

## Building Custom Images

To build images using the Makefile:

```bash
# Build all versions
make build-all

# Build specific version
make build-15  # Builds image that upgrades to PostgreSQL 15
make build-16  # Builds image that upgrades to PostgreSQL 16
make build-17  # Builds image that upgrades to PostgreSQL 17

# Build with custom registry and tag
REGISTRY=myregistry.io IMAGE_TAG=v1.0.0 make build-all
```

To build manually with Docker:

```bash
# Build image that upgrades to PostgreSQL 16
docker build --build-arg TARGET_VERSION=16 -t postgres-upgrade:to-16 .

# Build image that upgrades to PostgreSQL 15
docker build --build-arg TARGET_VERSION=15 -t postgres-upgrade:to-15 .
```

## Testing

The repository includes comprehensive tests using Make and Taskfile:

```bash
# Install Task (required for tests)
curl -sL https://taskfile.dev/install.sh | sh

# Run all tests
make test

# Test specific upgrade paths
make test-14-to-15  # Test upgrade from 14 to 15
make test-14-to-16  # Test upgrade from 14 to 16
make test-15-to-16  # Test upgrade from 15 to 16
make test-14-to-17  # Test upgrade from 14 to 17
make test-15-to-17  # Test upgrade from 15 to 17
make test-16-to-17  # Test upgrade from 16 to 17

# Clean test volumes
make clean
```

## CI/CD

The project includes GitHub Actions workflows:

- **build-push.yml**: Builds and pushes images to GitHub Container Registry
- **test.yml**: Runs comprehensive tests for all upgrade paths
- **ci.yml**: Lints Dockerfile and shell scripts

### Output Formatting

The Taskfiles are configured to use GitHub Actions' output grouping when running in CI. This creates collapsible sections in the workflow logs:

```yaml
# In GitHub Actions, task output will be grouped like:
::group::test:seed-version
... task output ...
::endgroup::
```

To enable grouped output locally:
```bash
TASK_OUTPUT=group task test
```

### Manual Image Publishing

To manually publish images:

```bash
# Push all versions
make push-all

# Push specific version
make push-15
make push-16
make push-17
```

## Important Notes

1. **Always backup your data** before running upgrades
2. The upgrade process modifies the data directory in-place
3. Once upgraded, you cannot downgrade to an older PostgreSQL version
4. The container runs as the `postgres` user (UID 999)
5. Ensure your data directory has correct permissions

## Environment Variables

- `TARGET_VERSION`: The PostgreSQL version to upgrade to (default: 17)

## Troubleshooting

### Permission Errors

If you encounter permission errors, ensure the data directory is owned by UID 999:

```bash
sudo chown -R 999:999 /path/to/your/pgdata
```

### Upgrade Failures

If an upgrade fails:

1. Check the container logs for specific errors
2. Ensure the data directory contains valid PostgreSQL data
3. Verify the source version is supported (14, 15, or 16)

### Data Corruption

The upgrade process uses hard links and preserves original data with `.old` suffix. If needed, you can recover the original data by:

1. Removing the upgraded data
2. Renaming `.old` files back to their original names
3. Using the original PostgreSQL version

## CLI Commands Overview

This project provides three main command interfaces:

- **Task Commands** - Primary interface using [Task](https://taskfile.dev)
- **Make Commands** - Traditional Makefile interface
- **pgconfig CLI** - Configuration management tool

### Prerequisites

Install Task (recommended):
```bash
# Linux/macOS
curl -sL https://taskfile.dev/install.sh | sh

# macOS via Homebrew
brew install go-task/tap/go-task

# Windows via Chocolatey
choco install go-task
```

## Task Commands

Task is the primary CLI interface. All commands support `--summary` for detailed help.

### Core Commands

```bash
# Main upgrade command
task                    # Auto-detect and upgrade PostgreSQL to version 17
task auto-upgrade       # Same as above with explicit name

# Help and information
task help               # Show comprehensive help for all commands
task --list             # List all available tasks
task status             # Show status of volumes and images
```

### Build Commands

```bash
# Build Docker images
task build              # Build default postgres-upgrade image
task build-all          # Build all target version images

# Individual version builds (via included Taskfile.build.yaml)
task build:build-15     # Build image targeting PostgreSQL 15
task build:build-16     # Build image targeting PostgreSQL 16
task build:build-17     # Build image targeting PostgreSQL 17
task build:push-all     # Push all images to registry
```

### Test Commands

```bash
# Test execution
task test               # Run all PostgreSQL upgrade tests
task test-image         # Build and test Docker image with integration tests
task dev-test-quick     # Quick development test (14→17 upgrade only)

# Test management
task clean              # Clean up test volumes and images
task dev-setup          # Set up development environment
```

### Extension Management Commands

```bash
# List and install extensions
task extensions-list    # Show all available PostgreSQL extensions
task extensions-install EXTENSIONS=pgvector,pgaudit,pg_cron

# Example: Install specific extensions
task extensions-install EXTENSIONS="pgvector,pgsodium,pg_cron"
```

### Service Management Commands

```bash
# Service monitoring
task services-status    # Show status of all services (PostgreSQL, PgBouncer, etc.)
task services-logs SERVICE=postgresql    # View logs for specific service
task services-logs SERVICE=pgbouncer     # PgBouncer logs
task services-logs SERVICE=postgrest     # PostgREST logs
task services-logs SERVICE=wal-g         # WAL-G logs
```

### Backup and Recovery Commands

```bash
# WAL-G backup operations (requires WAL-G enabled)
task backup-create      # Create a new WAL-G backup
task backup-list        # List all available backups
task backup-restore BACKUP_NAME=backup-20231201T120000Z
```

### Configuration Commands

```bash
# Configuration management
task generate-structs   # Generate Go structs from JSON schema
task validate-schema    # Validate JSON schema files
task build-pgconfig     # Build the pgconfig CLI tool
task test-config        # Run configuration tests
```

### Advanced/Legacy Commands

```bash
# Password management
task reset-password     # Reset PostgreSQL password (if RESET_PASSWORD=true)

# Manual upgrade control
task upgrade-single     # Perform single PostgreSQL upgrade (FROM= TO=)
task upgrade-from-env   # Upgrade using FROM/TO environment variables
```

## Make Commands

Traditional Makefile interface for common operations:

### Build Commands

```bash
make build              # Build default image
make build-15           # Build PostgreSQL 15 upgrade image
make build-16           # Build PostgreSQL 16 upgrade image
make build-17           # Build PostgreSQL 17 upgrade image
make build-all          # Build all version images

# Custom registry/tag
REGISTRY=myregistry.io IMAGE_TAG=v1.0.0 make build-all
```

### Push Commands

```bash
make push-15            # Push PostgreSQL 15 image
make push-16            # Push PostgreSQL 16 image
make push-17            # Push PostgreSQL 17 image
make push-all           # Push all images

# Custom registry
REGISTRY=ghcr.io IMAGE_BASE=myorg/postgres make push-all
```

### Test Commands

```bash
make test               # Run all tests
make test-simple        # Simple upgrade tests
make test-compose       # Docker Compose tests
make test-all           # Comprehensive test suite
make clean              # Clean test artifacts
```

### Utility Commands

```bash
make help               # Show Makefile help
make status             # Show system status
```

## pgconfig CLI Tool

The `pgconfig` tool manages PostgreSQL configuration and services.

### Installation

```bash
# Build from source
task build-pgconfig

# Or use pre-built from container
docker run --rm ghcr.io/flanksource/postgres:latest pgconfig version
```

### Core Commands

```bash
pgconfig version        # Show version information
pgconfig help          # Show usage help
```

### Configuration Generation

```bash
# Generate configuration files
pgconfig generate conf           # Generate postgresql.conf only
pgconfig generate hba            # Generate pg_hba.conf only
pgconfig generate recovery       # Generate recovery.conf
pgconfig generate all            # Generate all config files

# With custom options
pgconfig generate conf --memory=4GB --connections=200
pgconfig generate hba --auth-method=md5 --allow-host=192.168.1.0/24
```

### Validation Commands

```bash
# Validate configuration files
pgconfig validate --config=/path/to/postgresql.conf
pgconfig validate --hba=/path/to/pg_hba.conf
pgconfig validate --all          # Validate all found configs
```

### Server Management

```bash
# Health check server
pgconfig server --port=8080      # Start health check server
pgconfig server --host=0.0.0.0 --port=3001
```

### Service Management

```bash
# Supervisord integration
pgconfig supervisord start       # Start supervisord services
pgconfig supervisord stop        # Stop supervisord services
pgconfig supervisord status      # Show service status
pgconfig supervisord restart SERVICE_NAME
```

### Installation Management

```bash
# Install binary tools
pgconfig install postgres        # Install PostgreSQL binaries
pgconfig install postgrest       # Install PostgREST binary
pgconfig install wal-g           # Install WAL-G binary
pgconfig install all             # Install all tools
```

### Schema Management

```bash
# JSON schema operations
pgconfig schema generate         # Generate JSON schemas from PostgreSQL
pgconfig schema validate FILE    # Validate JSON schema file
pgconfig schema export FORMAT    # Export schema in various formats
```

## Docker Usage

### Quick Start

```bash
# Basic PostgreSQL upgrade
docker run --rm \
  -v /path/to/pgdata:/var/lib/postgresql/data \
  ghcr.io/flanksource/postgres-upgrade:to-17

# With environment variables
docker run --rm \
  -e PG_VERSION=17 \
  -e RESET_PASSWORD=true \
  -e POSTGRES_PASSWORD=newpassword \
  -v /path/to/pgdata:/var/lib/postgresql/data \
  ghcr.io/flanksource/postgres:latest
```

### Enhanced PostgreSQL with Extensions

```bash
# Run with extensions
docker run -d \
  -e POSTGRES_EXTENSIONS="pgvector,pgsodium,pg_cron" \
  -e POSTGRES_PASSWORD=mypassword \
  -p 5432:5432 \
  ghcr.io/flanksource/postgres:17-latest

# With additional services
docker run -d \
  -e POSTGRES_PASSWORD=mypassword \
  -e PGBOUNCER_ENABLED=true \
  -e POSTGREST_ENABLED=true \
  -e WALG_ENABLED=true \
  -p 5432:5432 \
  -p 6432:6432 \
  -p 3000:3000 \
  ghcr.io/flanksource/postgres:17-latest
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PG_VERSION` | Target PostgreSQL version | `17` |
| `AUTO_UPGRADE` | Enable automatic upgrade | `true` |
| `RESET_PASSWORD` | Reset password on startup | `false` |
| `START_POSTGRES` | Start PostgreSQL after upgrade | `false` |
| `POSTGRES_PASSWORD` | PostgreSQL password | - |
| `POSTGRES_USER` | PostgreSQL username | `postgres` |
| `POSTGRES_EXTENSIONS` | Extensions to enable | - |
| `PGBOUNCER_ENABLED` | Enable PgBouncer | `false` |
| `POSTGREST_ENABLED` | Enable PostgREST | `false` |
| `WALG_ENABLED` | Enable WAL-G backups | `false` |

## Extension Management

### Available Extensions (16 total)

| Extension | Description | Use Case |
|-----------|-------------|----------|
| **pgvector** | Vector similarity search | AI/ML, embeddings |
| **pgsodium** | Modern cryptography | Encryption, security |
| **pgjwt** | JSON Web Token support | Authentication |
| **pgaudit** | Audit logging | Compliance |
| **pg_tle** | Trusted Language Extensions | Safe development |
| **pg_stat_monitor** | Query performance monitoring | Optimization |
| **pg_repack** | Online table reorganization | Maintenance |
| **pg_plan_filter** | Query plan filtering | Performance |
| **pg_net** | Async HTTP requests | Integration |
| **pg_jsonschema** | JSON schema validation | Data validation |
| **pg_hashids** | Short unique ID generation | URL shortening |
| **pg_cron** | Job scheduler | Automation |
| **pg-safeupdate** | Require WHERE clause | Data safety |
| **index_advisor** | Index recommendations | Performance |
| **wal2json** | WAL to JSON converter | CDC, replication |

### Extension Usage Examples

#### Enable via Environment Variable
```bash
docker run -e POSTGRES_EXTENSIONS="pgvector,pgaudit,pg_cron" \
  ghcr.io/flanksource/postgres:17-latest
```

#### Enable via Task Command
```bash
task extensions-install EXTENSIONS="pgsodium,pgjwt,pg_net"
```

#### Manual SQL Installation
```sql
-- Install extensions manually
CREATE EXTENSION IF NOT EXISTS pgvector;
CREATE EXTENSION IF NOT EXISTS pgsodium;
CREATE EXTENSION IF NOT EXISTS pg_cron;
```

## Service Management

### PgBouncer (Connection Pooling)

```bash
# Enable via environment
docker run -d \
  -e PGBOUNCER_ENABLED=true \
  -e PGBOUNCER_POOL_MODE=transaction \
  -e PGBOUNCER_MAX_CLIENT_CONN=100 \
  -p 6432:6432 \
  ghcr.io/flanksource/postgres:17-latest

# Connect through PgBouncer
psql -h localhost -p 6432 -U postgres
```

### PostgREST (REST API)

```bash
# Enable REST API
docker run -d \
  -e POSTGREST_ENABLED=true \
  -e POSTGREST_DB_SCHEMAS=public \
  -p 3000:3000 \
  ghcr.io/flanksource/postgres:17-latest

# Use the API
curl http://localhost:3000/users
curl -X POST http://localhost:3000/users \
  -H "Content-Type: application/json" \
  -d '{"name": "John", "email": "john@example.com"}'
```

### Service Health Monitoring

```bash
# Check service status
task services-status

# View service logs
task services-logs SERVICE=postgresql
task services-logs SERVICE=pgbouncer

# Health check endpoints (if health server enabled)
curl http://localhost:8080/health
curl http://localhost:8080/metrics
```

## Backup and Recovery

### WAL-G Configuration

```bash
# Enable WAL-G with S3
docker run -d \
  -e WALG_ENABLED=true \
  -e WALG_S3_PREFIX=s3://my-bucket/postgres-backups \
  -e AWS_ACCESS_KEY_ID=your-key \
  -e AWS_SECRET_ACCESS_KEY=your-secret \
  ghcr.io/flanksource/postgres:17-latest
```

### Backup Operations

```bash
# Create backup
task backup-create

# List available backups
task backup-list

# Restore from backup
task backup-restore BACKUP_NAME=backup-20231201T120000Z

# Manual WAL-G operations (in container)
wal-g backup-push /var/lib/postgresql/data/pgdata
wal-g backup-list
wal-g backup-fetch /restore/path backup-name
```

## Testing

### Test Upgrade Paths

```bash
# Run all upgrade tests
task test

# Test specific paths
make test-14-to-15      # PostgreSQL 14 → 15
make test-14-to-16      # PostgreSQL 14 → 16
make test-14-to-17      # PostgreSQL 14 → 17
make test-15-to-16      # PostgreSQL 15 → 16
make test-15-to-17      # PostgreSQL 15 → 17
make test-16-to-17      # PostgreSQL 16 → 17
```

### Development Testing

```bash
# Quick development cycle
task dev-setup          # Set up test environment
task dev-test-quick     # Run fast tests (14→17 only)
task clean              # Clean up after testing
```

### Integration Testing

```bash
# Full integration tests
task test-image         # Test Docker image functionality
make test-compose       # Test via Docker Compose
```

## Configuration

### PostgreSQL Configuration

The system supports automatic PostgreSQL configuration tuning:

```bash
# Generate optimized config
pgconfig generate conf --memory=4GB --connections=200 --disk=ssd

# Validate configuration
pgconfig validate --config=/path/to/postgresql.conf
```

### Extension Configuration

```bash
# Configure pg_cron
echo "cron.database_name = 'postgres'" >> postgresql.conf

# Configure pgaudit
echo "shared_preload_libraries = 'pgaudit'" >> postgresql.conf
echo "pgaudit.log = 'all'" >> postgresql.conf
```

### Environment-based Configuration

```bash
# Via environment variables
docker run -d \
  -e POSTGRES_SHARED_PRELOAD_LIBRARIES="pgaudit,pg_stat_statements" \
  -e POSTGRES_MAX_CONNECTIONS=200 \
  -e POSTGRES_SHARED_BUFFERS=1GB \
  ghcr.io/flanksource/postgres:17-latest
```

## Development

### Building from Source

```bash
# Clone repository
git clone https://github.com/flanksource/postgres.git
cd postgres

# Install dependencies
task dev-setup

# Build all images
task build-all

# Run tests
task test
```

### Custom Image Development

```bash
# Build with custom extensions
docker build --build-arg EXTENSIONS="custom_ext1,custom_ext2" .

# Build for specific architecture
docker buildx build --platform linux/amd64,linux/arm64 .
```

### Configuration Development

```bash
# Generate configuration structs
task generate-structs

# Validate schema files
task validate-schema

# Test configuration changes
task test-config
```

## CI/CD

### GitHub Actions Integration

The project includes workflows for:

- **build-push.yml** - Build and push images
- **test.yml** - Run upgrade tests
- **ci.yml** - Lint and validation

### Local CI Testing

```bash
# Test like CI
GITHUB_ACTIONS=true task test

# With output grouping
TASK_OUTPUT=group task test
```

### Release Process

```bash
# Tag and push release
git tag v1.2.3
git push origin v1.2.3

# Manual image publishing
REGISTRY=ghcr.io IMAGE_TAG=v1.2.3 make push-all
```

## Troubleshooting

### Common Issues

#### Permission Errors
```bash
# Fix data directory permissions
sudo chown -R 999:999 /path/to/pgdata

# Or use user mapping
docker run --user $(id -u):$(id -g) ...
```

#### Upgrade Failures
```bash
# Check logs
task services-logs SERVICE=postgresql

# Validate data directory
pgconfig validate --data-dir=/path/to/pgdata

# Recovery from failed upgrade
mv /path/to/pgdata/PG_VERSION.old /path/to/pgdata/PG_VERSION
```

#### Extension Issues
```bash
# Check extension status
docker exec container psql -c "\dx"

# Reinstall extensions
task extensions-install EXTENSIONS="pgvector,pgaudit"

# Check extension health
/scripts/extension-health.sh
```

### Debugging Commands

```bash
# Container debugging
docker exec -it container bash
docker logs container-name

# Service debugging
task services-status
task services-logs SERVICE=postgresql

# Configuration debugging
pgconfig validate --all
pgconfig server --debug
```

### Health Checks

```bash
# Overall health
/scripts/service-health.sh

# Extension health
/scripts/extension-health.sh

# Connection testing
psql -h localhost -p 5432 -U postgres -c "SELECT version();"
```

## Contributing

Contributions are welcome! Please ensure:

1. All tests pass (`task test`)
2. New features include appropriate tests
3. Documentation is updated
4. Code follows project conventions

### Development Workflow

```bash
# Set up development environment
task dev-setup

# Make changes and test
task dev-test-quick

# Run full test suite
task test

# Submit pull request
```

## License

This project is licensed under the MIT License.
