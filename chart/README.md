# PostgreSQL Upgrade Helm Chart

A Helm chart for deploying PostgreSQL with automatic upgrade capabilities using Flanksource's PostgreSQL upgrade container.

## Features

- **Automatic PostgreSQL version upgrades** on startup
- **Dynamic configuration** based on Kubernetes resource limits
- **Admin password reset** on startup (optional)
- **Persistent storage** with configurable storage classes
- **Security-focused** with non-root containers and security contexts
- **Comprehensive monitoring** with startup, readiness, and liveness probes
- **Helm tests** for validation and performance testing

## Installation

### Add the Helm repository

```bash
helm repo add flanksource oci://ghcr.io/flanksource/charts
helm repo update
```

### Install the chart

```bash
helm install my-postgres flanksource/postgres-upgrade \
  --set postgresql.password=your-secure-password
```

### Install with custom configuration

```bash
helm install my-postgres flanksource/postgres-upgrade \
  --set postgresql.password=your-secure-password \
  --set postgresql.version=17 \
  --set resources.limits.memory=4Gi \
  --set persistence.size=50Gi
```

## Configuration

### PostgreSQL Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `postgresql.version` | Target PostgreSQL version | `17` |
| `postgresql.database` | Default database name | `postgres` |
| `postgresql.username` | PostgreSQL username | `postgres` |
| `postgresql.password` | PostgreSQL password (required) | `""` |
| `postgresql.resetPassword` | Reset password on startup | `false` |
| `postgresql.autoUpgrade` | Enable automatic upgrades | `true` |

### PostgreSQL Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `postgresql.config.max_connections` | Maximum connections | `100` |
| `postgresql.config.shared_buffers` | Shared buffers (auto-calculated if empty) | `""` |
| `postgresql.config.effective_cache_size` | Effective cache size (auto-calculated if empty) | `""` |
| `postgresql.config.work_mem` | Work memory (auto-calculated if empty) | `""` |
| `postgresql.config.maintenance_work_mem` | Maintenance work memory (auto-calculated if empty) | `""` |
| `postgresql.config.custom` | Custom configuration key-value pairs | `{}` |

### Image Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.registry` | Container registry | `ghcr.io` |
| `image.repository` | Image repository | `flanksource/postgres-upgrade` |
| `image.tag` | Image tag | `to-17-latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |

### Resource Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.limits.cpu` | CPU limit | `2000m` |
| `resources.limits.memory` | Memory limit | `4Gi` |
| `resources.requests.cpu` | CPU request | `500m` |
| `resources.requests.memory` | Memory request | `1Gi` |

### Persistence

| Parameter | Description | Default |
|-----------|-------------|---------|
| `persistence.enabled` | Enable persistent storage | `true` |
| `persistence.storageClass` | Storage class | `""` |
| `persistence.accessMode` | Access mode | `ReadWriteOnce` |
| `persistence.size` | Storage size | `10Gi` |

### Service Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `5432` |
| `service.annotations` | Service annotations | `{}` |

## Dynamic Configuration

The chart automatically calculates optimal PostgreSQL settings based on your Kubernetes resource limits:

- **shared_buffers**: 25% of memory limit
- **effective_cache_size**: 75% of memory limit
- **work_mem**: 0.5% of memory limit
- **maintenance_work_mem**: 6.25% of memory limit
- **wal_buffers**: 0.78% of memory limit

You can override these by setting explicit values in `postgresql.config.*`.

## Automatic Upgrades

When `postgresql.autoUpgrade` is enabled (default), the container will:

1. Detect the current PostgreSQL version in the data directory
2. Compare it with the target version (`postgresql.version`)
3. Automatically run `pg_upgrade` if needed
4. Preserve all data and configuration during the upgrade

### Supported Upgrade Paths

- PostgreSQL 14 → 15, 16, 17
- PostgreSQL 15 → 16, 17
- PostgreSQL 16 → 17

## Testing

Run the included Helm tests to verify your deployment:

```bash
helm test my-postgres
```

This will run:
- Connection tests
- Basic SQL operations
- Performance validation
- Configuration verification

## Security

The chart follows security best practices:

- Runs as non-root user (UID 999)
- Uses read-only root filesystem where possible
- Drops all capabilities
- Supports Pod Security Standards
- Configurable network policies
- Secure password management

## Monitoring

The chart includes comprehensive health checks:

- **Startup probe**: Ensures PostgreSQL starts successfully (up to 5 minutes)
- **Readiness probe**: Confirms PostgreSQL is ready to accept connections
- **Liveness probe**: Monitors PostgreSQL health during runtime

## Examples

### High Availability Setup

```yaml
resources:
  limits:
    cpu: 4000m
    memory: 8Gi
  requests:
    cpu: 1000m
    memory: 2Gi

persistence:
  size: 100Gi
  storageClass: "fast-ssd"

database:
  config:
    max_connections: "200"
    custom:
      synchronous_commit: "on"
      wal_level: "replica"
```

### Development Environment

```yaml
resources:
  limits:
    cpu: 1000m
    memory: 2Gi
  requests:
    cpu: 200m
    memory: 512Mi

persistence:
  size: 5Gi

database:
  config:
    max_connections: "50"
    custom:
      log_statement: "all"
      log_min_duration_statement: "0"
```

## Troubleshooting

### Check PostgreSQL logs

```bash
kubectl logs -l app.kubernetes.io/name=postgres-upgrade
```

### Check configuration

```bash
kubectl exec -it postgres-upgrade-0 -- psql -U postgres -c "SHOW ALL;"
```

### Verify upgrade status

```bash
kubectl exec -it postgres-upgrade-0 -- cat /var/lib/postgresql/data/pgdata/PG_VERSION
```

## Contributing

Please read our [contributing guidelines](https://github.com/flanksource/postgres/blob/main/CONTRIBUTING.md) before submitting pull requests.

## License

This chart is licensed under the Apache License 2.0. See [LICENSE](https://github.com/flanksource/postgres/blob/main/LICENSE) for details.
