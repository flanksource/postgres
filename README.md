# Enhanced PostgreSQL Distribution with Extensions

This repository provides a comprehensive PostgreSQL distribution with automatic upgrade capabilities and popular extensions. Based on Supabase's enhanced PostgreSQL, it includes pgvector, pgsodium, PostgREST, PgBouncer, WAL-G backup, and many other extensions commonly used in modern applications.

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

## Contributing

Contributions are welcome! Please ensure:

1. All tests pass (`task test`)
2. New features include appropriate tests
3. Documentation is updated

## License

This project is licensed under the MIT License.