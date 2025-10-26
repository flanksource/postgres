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
  -e POSTGRES_PASSWORD=mypassword \
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
helm install my-postgres flanksource/postgres-upgrade \
  --set database.password=your-password
```

### Docker

```bash
docker run -d \
  -e POSTGRES_PASSWORD=mypassword \
  -e POSTGRES_EXTENSIONS="pgvector,pgsodium,pg_cron" \
  -p 5432:5432 \
  ghcr.io/flanksource/postgres:17-latest
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
| `ghcr.io/flanksource/postgres:17-latest` | PostgreSQL 17 with all extensions |
| `ghcr.io/flanksource/postgres:16-latest` | PostgreSQL 16 with all extensions |
| `ghcr.io/flanksource/postgres-upgrade:to-17` | Upgrade-only image to PostgreSQL 17 |

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
helm upgrade my-postgres flanksource/postgres-upgrade \
  --set database.resetPassword=true \
  --set database.password=new-password \
  --reuse-values
```

### Docker

```bash
docker run --rm \
  -v postgres_data:/var/lib/postgresql/data \
  -e RESET_PASSWORD=true \
  -e POSTGRES_PASSWORD=new-password \
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
helm install my-postgres flanksource/postgres-upgrade -f values.yaml
```

### Upgrade PostgreSQL Version

```bash
helm upgrade my-postgres flanksource/postgres-upgrade \
  --set database.version=17 \
  --reuse-values

kubectl rollout restart statefulset/my-postgres
```

## PostgreSQL Extensions

Pre-compiled extensions available:

| Extension | Purpose | Usage |
|-----------|---------|-------|
| `pgvector` | Vector similarity search | `CREATE EXTENSION pgvector;` |
| `pgsodium` | Cryptography | `CREATE EXTENSION pgsodium;` |
| `pg_cron` | Job scheduler | `CREATE EXTENSION pg_cron;` |
| `pgaudit` | Audit logging | `CREATE EXTENSION pgaudit;` |
| `pg_stat_monitor` | Query monitoring | `CREATE EXTENSION pg_stat_monitor;` |
| `pgjwt` | JWT authentication | `CREATE EXTENSION pgjwt;` |
| `pg_net` | HTTP client | `CREATE EXTENSION pg_net;` |
| `pg_jsonschema` | JSON validation | `CREATE EXTENSION pg_jsonschema;` |
| `pg_hashids` | Short IDs | `CREATE EXTENSION pg_hashids;` |
| `pg-safeupdate` | Require WHERE clause | `CREATE EXTENSION pg-safeupdate;` |
| `wal2json` | CDC output | `CREATE EXTENSION wal2json;` |
| `pg_repack` | Table reorganization | `CREATE EXTENSION pg_repack;` |
| `pg_plan_filter` | Query plan filtering | `CREATE EXTENSION pg_plan_filter;` |
| `pg_tle` | Trusted Language Extensions | `CREATE EXTENSION pg_tle;` |
| `index_advisor` | Index recommendations | `CREATE EXTENSION index_advisor;` |

### Enable Extensions

```bash
# Environment variable
POSTGRES_EXTENSIONS="pgvector,pgsodium,pg_cron"

# Helm values
extensions:
  enabled: "pgvector,pgsodium,pg_cron"
```

## Docker Usage

### Production Configuration

```bash
docker run -d \
  --name postgres \
  --memory="8g" \
  --cpus="4" \
  -e POSTGRES_PASSWORD=mypassword \
  -e POSTGRES_EXTENSIONS="pgvector,pgsodium" \
  -e PGBOUNCER_ENABLED=true \
  -e POSTGREST_ENABLED=true \
  -e WALG_ENABLED=true \
  -e WALG_S3_PREFIX=s3://bucket/backups \
  -e AWS_ACCESS_KEY_ID=$AWS_KEY \
  -e AWS_SECRET_ACCESS_KEY=$AWS_SECRET \
  -p 5432:5432 \
  -p 6432:6432 \
  -p 3000:3000 \
  -v postgres_data:/var/lib/postgresql/data \
  ghcr.io/flanksource/postgres:17-latest
```

### Docker Compose

```yaml
version: '3.8'
services:
  postgres:
    image: ghcr.io/flanksource/postgres:17-latest
    deploy:
      resources:
        limits:
          memory: 8G
          cpus: '4'
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_EXTENSIONS: pgvector,pgsodium,pg_cron
      PGBOUNCER_ENABLED: "true"
      POSTGREST_ENABLED: "true"
    ports:
      - "5432:5432"
      - "6432:6432"
      - "3000:3000"
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  postgres_data:
```

## Automatic Upgrades

The postgres-cli orchestrates safe, sequential PostgreSQL upgrades with full data preservation:

### Supported Upgrade Paths

| From | To | Process | Backup Location |
|------|-----|---------|-----------------|
| PostgreSQL 14 | PostgreSQL 17 | Sequential: 14→15→16→17 | `/data/backups/data-14` |
| PostgreSQL 15 | PostgreSQL 17 | Sequential: 15→16→17 | `/data/backups/data-15` |
| PostgreSQL 16 | PostgreSQL 17 | Direct: 16→17 | `/data/backups/data-16` |

### Technical Implementation

The upgrade process is managed by `pkg/server/postgres.go:Upgrade()`:

```go
// Simplified upgrade flow
func (p *Postgres) Upgrade(targetVersion int) error {
    // 1. Detect current version from PG_VERSION file
    currentVersion := p.DetectVersion()

    // 2. Create backup of original data
    backupPath := fmt.Sprintf("/data/backups/data-%d", currentVersion)
    p.backupDataDirectory(backupPath)

    // 3. Sequential upgrades through each version
    for v := currentVersion; v < targetVersion; v++ {
        p.upgradeSingle(v, v+1)  // Handles pg_upgrade orchestration
    }
}
```

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

| Variable | Description | Default |
|----------|-------------|---------|
| `POSTGRES_PASSWORD` | Database password | Required |
| `POSTGRES_USER` | Database user | `postgres` |
| `POSTGRES_DB` | Default database | `postgres` |
| `POSTGRES_EXTENSIONS` | Comma-separated extensions | None |
| `PG_VERSION` | Target PostgreSQL version | `17` |
| `AUTO_UPGRADE` | Enable automatic upgrades | `true` |
| `RESET_PASSWORD` | Reset password on startup | `false` |
| `PGBOUNCER_ENABLED` | Enable PgBouncer | `false` |
| `POSTGREST_ENABLED` | Enable PostgREST | `false` |
| `WALG_ENABLED` | Enable WAL-G backups | `false` |

## Backup and Recovery

### WAL-G Configuration

```yaml
walg:
  enabled: true
  s3:
    bucket: postgres-backups
    region: us-east-1
  schedule: "0 2 * * *"
```

### Backup Operations

```bash
# Create backup
kubectl exec -it my-postgres-0 -- wal-g backup-push

# List backups
kubectl exec -it my-postgres-0 -- wal-g backup-list

# Restore
kubectl exec -it my-postgres-0 -- wal-g backup-fetch /restore/path LATEST
```

## Health Monitoring

### Kubernetes Probes

```yaml
startupProbe:
  exec:
    command: ["pg_isready", "-U", "postgres"]
  initialDelaySeconds: 10
  periodSeconds: 10
  failureThreshold: 30

readinessProbe:
  exec:
    command: ["pg_isready", "-U", "postgres"]
  periodSeconds: 10

livenessProbe:
  exec:
    command: ["pg_isready", "-U", "postgres"]
  periodSeconds: 30
```

### Health Check

```bash
# Kubernetes
kubectl exec my-postgres-0 -- pg_isready

# Docker
docker exec postgres pg_isready
```

## Troubleshooting

### Check Logs

```bash
# Kubernetes
kubectl logs my-postgres-0
kubectl logs my-postgres-0 --previous

# Docker
docker logs postgres
```

### Common Issues

#### Password Reset Failed
```bash
# Check single-user mode logs
kubectl logs my-postgres-0 -c password-reset

# Verify permissions
ls -la /var/lib/postgresql/data/pgdata
```

#### Upgrade Failed
```bash
# Check pg_upgrade logs
kubectl exec my-postgres-0 -- cat /var/lib/postgresql/pg_upgrade_output.d/pg_upgrade_server.log

# Verify versions
kubectl exec my-postgres-0 -- cat /var/lib/postgresql/data/pgdata/PG_VERSION
```

#### Performance Issues
```bash
# Check current settings
kubectl exec my-postgres-0 -- psql -c "SHOW ALL;" | grep -E "shared_buffers|work_mem|effective_cache"

# Monitor connections
kubectl exec my-postgres-0 -- psql -c "SELECT count(*) FROM pg_stat_activity;"
```

## Additional Services

### PgBouncer
Connection pooler on port 6432:
- Transaction pooling mode
- Configurable max connections
- Automatic user/database discovery

### PostgREST
REST API on port 3000:
- Auto-generates OpenAPI spec
- JWT authentication
- Row-level security support

### WAL-G
Backup tool:
- Incremental backups
- Point-in-time recovery
- S3/GCS/Azure support

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