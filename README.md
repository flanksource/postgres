# PostgreSQL with Extensions and Auto-Upgrades

PostgreSQL distribution with automatic version upgrades, password recovery, performance auto-tuning, and 16 pre-compiled extensions. Optimized for Kubernetes deployments.

## Key Features

- **Automatic PostgreSQL upgrades** - Handles upgrade paths from 14→15→16→17 using pg_upgrade with hard links
- **Password recovery** - Reset passwords without data loss using single-user mode in init containers
- **PgTune auto-configuration** - Calculates optimal settings based on container memory/CPU limits
- **16 pre-compiled extensions** - pgvector, pgsodium, pgjwt, pgaudit, pg_cron, and more included
- **Connection pooling** - PgBouncer integrated with transaction pooling mode
- **REST API** - PostgREST generates RESTful APIs from database schema
- **Backup integration** - WAL-G configured for S3/GCS/Azure backups
- **Helm charts** - Kubernetes deployment with StatefulSets, probes, and PVCs

## ⚠️ Breaking Change: Security Update

**Version 2.x** introduces a security improvement where the container now runs as the `postgres` user (UID 999) by default instead of root. This follows container security best practices and reduces the attack surface.

### What Changed

- **Previous behavior**: Container ran as root user
- **New behavior**: Container runs as `postgres` user (UID 999) by default
- **Impact**: Existing volumes with incorrect ownership will cause permission errors

### Migration Guide

#### For Docker Users

If you encounter permission errors after upgrading, fix volume ownership:

```bash
# Check current volume ownership
docker run --rm -v your-volume:/data alpine ls -la /data

# Fix permissions (if owned by root or other user)
docker run --rm --user root -v your-volume:/data alpine chown -R 999:999 /data

# Then start normally (will run as postgres user)
docker run -v your-volume:/var/lib/postgresql/data flanksource/postgres:latest
```

**Recommended**: Use named volumes (Docker handles permissions automatically):

```bash
docker run -d \
  -v pgdata:/var/lib/postgresql/data \
  -e PGPASSWORD=mypassword \
  ghcr.io/flanksource/postgres:latest
```

#### For Kubernetes Users

Add `securityContext` to your Pod/StatefulSet spec:

```yaml
apiVersion: v1
kind: Pod
spec:
  securityContext:
    runAsUser: 999
    runAsGroup: 999
    fsGroup: 999  # Ensures PVC is owned by postgres
  containers:
  - name: postgres
    image: ghcr.io/flanksource/postgres:latest
    volumeMounts:
    - name: pgdata
      mountPath: /var/lib/postgresql/data
```

#### Permission Fix Mode

If you need to fix permissions on existing volumes, temporarily run as root:

```bash
# Run once as root to fix permissions
docker run --user root \
  -v your-volume:/var/lib/postgresql/data \
  ghcr.io/flanksource/postgres:latest

# Container will detect wrong ownership, fix it, and exit
# Then restart without --user flag (runs as postgres by default)
docker run -v your-volume:/var/lib/postgresql/data \
  ghcr.io/flanksource/postgres:latest
```

#### Validation

Use `--dry-run` to validate permissions before starting:

```bash
docker run --rm \
  -v your-volume:/var/lib/postgresql/data \
  --entrypoint postgres-cli \
  ghcr.io/flanksource/postgres:latest \
  auto-start --dry-run --data-dir /var/lib/postgresql/data
```

### Why This Change?

- **Security**: Running as non-root reduces attack surface and follows least-privilege principle
- **Compliance**: Aligns with container security best practices and PodSecurityStandards
- **Consistency**: Matches official PostgreSQL Docker image behavior

## Quick Start

### Helm (Kubernetes)

```bash
# Add repository
helm repo add flanksource https://flanksource.github.io/charts
helm repo update

# Install
helm install my-postgres flanksource/postgres \
  --set database.password=your-password
```

### Docker

```bash
docker run -d \
  -e PGPASSWORD=mypassword \
  -p 5432:5432 \
  ghcr.io/flanksource/postgres:17
```

### CLI

```bash
# Install
go install github.com/flanksource/postgres/cmd@latest

# Generate configuration
postgres-cli generate conf --memory=4GB --connections=200
```

## How postgres-cli Orchestrates Upgrades

The postgres-cli tool provides intelligent PostgreSQL version upgrades with zero data loss through a multi-phase orchestration process:

### Upgrade Detection and Planning

1. **Version Detection**: Reads `/var/lib/postgresql/data/PG_VERSION` to identify current version
2. **Upgrade Path Planning**: Determines sequential upgrade steps (e.g., 14→15→16→17)
3. **Validation**: Ensures data directory exists and PostgreSQL is stopped

### Multi-Phase Upgrade Process

```
Current Data (v14) → Backup → Sequential Upgrades → Final Version (v17)
                        ↓
                  /backups/data-14 (preserved)
```

The `Postgres.Upgrade()` method in `pkg/server/postgres.go` orchestrates:

1. **Pre-upgrade Backup** (`backupDataDirectory`):
   - Creates timestamped backup in `/var/lib/postgresql/data/backups/data-{version}`
   - Preserves original data for rollback capability
   - Excludes recursive backup/upgrade directories

2. **Sequential Version Upgrades** (`upgradeSingle`):
   - For each version hop (14→15, 15→16, 16→17):
     - Validates current cluster with `pg_controldata`
     - Initializes new cluster in `/var/lib/postgresql/data/upgrades/{version}`
     - Runs `pg_upgrade --check` for compatibility verification
     - Executes `pg_upgrade` with hard links (no data duplication)
     - Validates upgraded cluster state
     - Moves upgraded data to main location

3. **Data Migration** (`moveUpgradedData`):
   - Safely removes old version files from main directory
   - Moves upgraded data from staging to production location
   - Preserves backup and upgrade directories

### Failure Protection

- **Backup Preservation**: Original data always preserved in `/backups/data-{version}`
- **Validation Checks**: Pre and post-upgrade cluster validation using `pg_controldata`
- **Atomic Operations**: Each version upgrade is atomic with rollback capability
- **Detailed Logging**: Captures stdout/stderr for debugging failed upgrades

### Docker Container Integration

The Docker container orchestrates upgrades through a layered approach:

1. **Entry Point** (`docker-entrypoint.sh`):
   ```bash
   # Auto-detection mode when no arguments provided
   if [ $# -eq 0 ]; then
       # Delegates to Task runner for orchestration
       exec gosu postgres task auto-upgrade
   fi
   ```

2. **Task Runner** (`Taskfile.run.yaml:auto-upgrade`):
   ```bash
   # Detects current version from PG_VERSION
   CURRENT_VERSION=$(cat /var/lib/postgresql/data/PG_VERSION)
   TARGET_VERSION="${PG_VERSION:-17}"

   # Calls postgres-cli for actual upgrade
   exec postgres-cli upgrade --target-version $TARGET_VERSION
   ```

3. **CLI Command** (`cmd/server.go:createUpgradeCommand`):
   - Parses target version from flags
   - Creates `Postgres` instance with data directory
   - Calls `Postgres.Upgrade(targetVersion)` for orchestration

4. **Permission Handling**:
   - Container starts as root for flexibility
   - Uses `gosu postgres` to switch to postgres user
   - Ensures proper file ownership throughout upgrade

## Available Images

| Image | Description |
|-------|-------------|
| `ghcr.io/flanksource/postgres:17` | PostgreSQL 17 with the ability to upgrade from 14 |
| `ghcr.io/flanksource/postgres:16` | PostgreSQL 16 with the ability to upgrade from 14 |

## Automatic Performance Tuning

The container detects resource limits and configures PostgreSQL accordingly:

### Configuration Algorithm

| Parameter | Calculation | Example (8GB RAM) |
|-----------|-------------|-------------------|
| `shared_buffers` | 25% of RAM | 2GB |
| `effective_cache_size` | 75% of RAM | 6GB |
| `work_mem` | RAM × 0.5% ÷ max_connections | 40MB |
| `maintenance_work_mem` | 6.25% of RAM | 512MB |
| `wal_buffers` | 3% of shared_buffers | 64MB |
| `max_wal_size` | 2GB default | 2GB |

### Resource Detection

```yaml
# Kubernetes
resources:
  limits:
    memory: 8Gi    # Triggers auto-configuration
    cpu: 4

# Docker
docker run --memory="8g" --cpus="4" ...
```

### Manual Override

```yaml
database:
  config:
    shared_buffers: "4GB"        # Override calculated value
    max_connections: "200"       # Set explicitly
```

## Password Reset

Password recovery without data loss:

### Kubernetes

```bash
helm upgrade my-postgres flanksource/postgres \
  --set database.resetPassword=true \
  --set database.password=new-password \
  --reuse-values
```

### Docker

```bash
docker run --rm \
  -v postgres_data:/var/lib/postgresql/data \
  -e RESET_PASSWORD=true \
  -e PGPASSWORD=new-password \
  ghcr.io/flanksource/postgres:17-latest
```

### Implementation

1. Starts PostgreSQL in single-user mode
2. Updates password in pg_authid
3. Restarts in normal multi-user mode
4. No downtime for existing connections

## Kubernetes Deployment

### values.yaml

```yaml
database:
  version: "17"
  password: "change-me"
  autoUpgrade: true
  resetPassword: false

resources:
  limits:
    cpu: 4
    memory: 8Gi
  requests:
    cpu: 2
    memory: 4Gi

persistence:
  size: 100Gi
  storageClass: fast-ssd

extensions:
  enabled: "pgvector,pgsodium,pgaudit,pg_cron"

pgbouncer:
  enabled: true
  poolMode: transaction
  maxClientConn: 1000

postgrest:
  enabled: true
  schemas: public

walg:
  enabled: true
  s3:
    bucket: postgres-backups
    region: us-east-1
```

### Deploy

```bash
helm install my-postgres flanksource/postgres -f values.yaml
```

### Upgrade PostgreSQL Version

```bash
helm upgrade my-postgres flanksource/postgres \
  --set database.version=17 \
  --reuse-values

kubectl rollout restart statefulset/my-postgres
```


## Automatic Upgrades

The postgres-cli orchestrates safe, sequential PostgreSQL upgrades with full data preservation:



### Upgrade Orchestration Details

1. **Pre-Upgrade Validation** (`validateCluster`):
   - Verifies PG_VERSION file matches expected version
   - Runs `pg_controldata` to check cluster state
   - Ensures data directory permissions are correct

2. **New Cluster Initialization** (`initNewCluster`):
   - Creates upgrade workspace: `/data/upgrades/{version}`
   - Runs `initdb` for target PostgreSQL version
   - Configures new cluster with same settings

3. **pg_upgrade Execution** (`runPgUpgrade`):
   ```bash
   # Compatibility check first
   pg_upgrade --check \
     --old-bindir=/usr/lib/postgresql/14/bin \
     --new-bindir=/usr/lib/postgresql/15/bin \
     --old-datadir=/var/lib/postgresql/data \
     --new-datadir=/data/upgrades/15

   # Actual upgrade with hard links (no data duplication)
   pg_upgrade \
     --old-bindir=/usr/lib/postgresql/14/bin \
     --new-bindir=/usr/lib/postgresql/15/bin \
     --old-datadir=/var/lib/postgresql/data \
     --new-datadir=/data/upgrades/15
   ```

4. **Data Migration** (`moveUpgradedData`):
   - Removes old version files from main directory
   - Moves upgraded data from `/data/upgrades/{version}` to main location
   - Preserves backup directories for rollback capability

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTO_UPGRADE` | `true` | Enable automatic version detection and upgrade |
| `PG_VERSION` | `17` | Target PostgreSQL version for upgrades |
| `START_POSTGRES` | `false` | Start PostgreSQL after successful upgrade |

### Failure Recovery

If an upgrade fails:
1. Original data remains in `/data/backups/data-{version}`
2. Each upgrade step logs detailed output for debugging
3. Manual recovery: `cp -r /data/backups/data-{version}/* /var/lib/postgresql/data/`

## Configuration

### Environment Variables

#### PostgreSQL Core Settings

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `PG_VERSION` | Target PostgreSQL version (14-17) | `17` | `16` |
| `PGDATA` | PostgreSQL data directory | `/var/lib/postgresql/data` | Custom path |
| `POSTGRES_DB` | Default database name | `postgres` | `myapp` |
| `POSTGRES_USER` | Database superuser username | `postgres` | `admin` |
| `PGPASSWORD` | Database password (direct) | - | `mySecurePassword` |
| `PGPASSWORD_FILE` | Path to file containing password | - | `/run/secrets/db_password` |


#### Auto-Configuration

| Variable | Description | Default | Values |
|----------|-------------|---------|--------|
| `PG_TUNE` | Alias for PGCONFIG_AUTO_TUNE | `true` | `true`, `false` |
| `POSTGRES_CLI_ARGS` | Custom postgres-cli arguments | See below | `--dry-run --pg-tune` |
| `UPGRADE_ONLY` | Exit after upgrade (no start) | `false` | `true`, `false` |

#### Performance Tuning (pg_tune)

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `PG_TUNE_MAX_CONNECTIONS` | Override max connections | Auto-calculated | `200` |
| `PG_TUNE_MEMORY` | Override memory in MB | Auto-detected | `8192` |
| `PG_TUNE_CPUS` | Override CPU count | Auto-detected | `4` |
| `PG_AUTH_METHOD` | Authentication method | `scram-sha-256` | `md5`, `trust` |



## postgres-cli Reference

The `postgres-cli` tool provides comprehensive PostgreSQL management. It's included in the Docker image and can be installed standalone.

### Installation

```bash
# Install CLI tool
go install github.com/flanksource/postgres/cmd@latest

# Or use Docker image
docker run --rm ghcr.io/flanksource/postgres:17 postgres-cli --help
```

### Global Flags

Available for all commands:

| Flag | Short | Description | Default | Example |
|------|-------|-------------|---------|---------|
| `--username` | `-U` | PostgreSQL username | `postgres` or `$PG_USER` | `-U admin` |
| `--password` | `-W` | PostgreSQL password | `$PGPASSWORD` or `$PGPASSWORD_FILE` | `-W mypass` |
| `--database` | `-d` | Database name | `postgres` or `$PG_DATABASE` | `-d myapp` |
| `--host` | | PostgreSQL host | `localhost` or `$PG_HOST` | `--host db.example.com` |
| `--port` | `-p` | PostgreSQL port | `5432` or `$PG_PORT` | `-p 5433` |
| `--data-dir` | | Data directory path | `$PGDATA` or auto-detected | `--data-dir /pgdata` |
| `--bin-dir` | | PostgreSQL binary directory | Auto-detected | `--bin-dir /usr/lib/postgresql/17/bin` |
| `--config` | `-c` | Configuration file path | - | `-c /etc/postgresql.conf` |
| `--locale` | | Database locale | `C` | `--locale en_US.UTF-8` |
| `--encoding` | | Database encoding | `UTF8` | `--encoding UTF8` |
| `--dry-run` | | Simulate without changes | `false` | `--dry-run` |

### Commands

#### auto-start

Automatically start PostgreSQL with optional pre-start tasks.

```bash
postgres-cli auto-start [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--auto-init` | Initialize database if not exists | `true` |
| `--auto-upgrade` | Upgrade PostgreSQL if version mismatch | `true` |
| `--auto-reset-password` | Reset password on startup | `true` |
| `--pg-tune` | Run pg_tune optimization | `true` |
| `--upgrade-to <N>` | Target PostgreSQL version | `0` (auto-detect latest) |
| `--max-connections` | Max connections for pg_tune | `0` (auto-calculate) |
| `--memory` | Override memory in MB | `0` (auto-detect) |
| `--cpus` | Override CPU count | `0` (auto-detect) |
| `--type` | Database type for pg_tune | `web` |
| `--auth-method` | pg_hba.conf auth method | `scram-sha-256` |

**Examples:**

```bash
# Start with all auto features
postgres-cli auto-start

# Initialize new database and start
postgres-cli auto-start --auto-init

# Upgrade to version 17 without starting
postgres-cli auto-start --upgrade-to=17 --dry-run

# Start with custom tuning
postgres-cli auto-start --pg-tune --max-connections=200 --memory=8192
```

#### server Commands

Manage PostgreSQL server instances:

```bash
postgres-cli server <subcommand> [flags]
```

**Subcommands:**

| Command | Description |
|---------|-------------|
| `status` | Show comprehensive PostgreSQL status |
| `health` | Perform health check |
| `start` | Start PostgreSQL server |
| `stop` | Stop PostgreSQL server gracefully |
| `restart` | Restart PostgreSQL server |
| `initdb` | Initialize PostgreSQL data directory |
| `reset-password` | Reset PostgreSQL superuser password |
| `upgrade` | Upgrade PostgreSQL to target version |
| `backup` | Create PostgreSQL backup using pg_dump |
| `sql` | Execute SQL query |

**Examples:**

```bash
# Check PostgreSQL status
postgres-cli server status

# Initialize new cluster
postgres-cli server initdb --data-dir=/pgdata

# Reset password
postgres-cli server reset-password --password=newpass

# Upgrade to version 17
postgres-cli server upgrade --target-version=17

# Execute SQL query
postgres-cli server sql --query="SELECT version();"

# Execute SQL from file
postgres-cli server sql --file=/path/to/script.sql
```

#### version

Show version information:

```bash
postgres-cli version
```

### Usage Examples

#### Docker Container Usage

The docker-entrypoint.sh automatically calls `postgres-cli auto-start`:

```bash
# Default behavior (all features enabled)
docker run ghcr.io/flanksource/postgres:17

# Custom postgres-cli arguments
docker run -e POSTGRES_CLI_ARGS="--pg-tune --dry-run" \
  ghcr.io/flanksource/postgres:17

# Upgrade only (no start)
docker run -e UPGRADE_ONLY=true \
  -v pgdata:/var/lib/postgresql/data \
  ghcr.io/flanksource/postgres:17
```

#### Standalone CLI Usage

```bash
# Check status of local PostgreSQL
postgres-cli server status --data-dir=/var/lib/postgresql/data

# Upgrade local PostgreSQL installation
postgres-cli server upgrade \
  --target-version=17 \
  --data-dir=/var/lib/postgresql/data

# Generate optimized configuration
postgres-cli auto-start \
  --pg-tune \
  --memory=8192 \
  --max-connections=200 \
  --dry-run

# Connect to remote PostgreSQL
postgres-cli server sql \
  --host=db.example.com \
  --port=5432 \
  --username=admin \
  --query="SELECT count(*) FROM users;"
```

#### Password Management

```bash
# Reset password using environment variable
export PGPASSWORD=oldpass
export POSTGRES_PASSWORD=newpass
postgres-cli server reset-password

# Reset password using file
echo "newpassword" > /tmp/password
postgres-cli server reset-password \
  --password=$(cat /tmp/password)
rm /tmp/password

# Reset via auto-start
postgres-cli auto-start --auto-reset-password
```



## Documentation

- [Contributing Guide](CONTRIBUTING.md) - Development setup and guidelines
- [Helm Chart Documentation](chart/README.md) - Chart configuration details
- [Extension Examples](docs/extensions.md) - Extension usage
- [Migration Guide](docs/migration.md) - Migrating from other distributions

## Support

- [GitHub Issues](https://github.com/flanksource/postgres/issues)
- [GitHub Discussions](https://github.com/flanksource/postgres/discussions)
- Security: security@flanksource.com

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
