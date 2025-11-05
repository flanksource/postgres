# postgres

A Helm chart for PostgreSQL with automatic upgrade capabilities

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 17](https://img.shields.io/badge/AppVersion-17-informational?style=flat-square)

## Chart Information

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Flanksource | <support@flanksource.com> | <https://flanksource.com> |

## Source Code

* <https://github.com/flanksource/postgres>

**Homepage:** <https://github.com/flanksource/postgres>

## Features

- 🚀 **Automatic Upgrades**: Built-in support for automatic PostgreSQL version upgrades
- 🎯 **Auto-tuning**: Automatic performance tuning based on available resources
- 🔐 **Security**: Runs as non-root user with configurable security contexts
- 💾 **Persistence**: Optional persistent storage with PVC support
- 🔧 **Flexible Configuration**: Extensive PostgreSQL configuration options
- 📊 **Health Checks**: Comprehensive startup, liveness, and readiness probes

## Quick Start

### Basic Installation

Install PostgreSQL with default settings:

```bash
helm repo add flanksource https://flanksource.github.io/charts
helm repo update
helm install postgres flanksource/postgres
```

### Installation with Custom Password

```bash
kubectl create secret generic postgres-password --from-literal=password=mySecurePassword123
helm install postgres flanksource/postgres --set passwordRef.create=false
```

### Installation with Custom Resources

```bash
helm install postgres flanksource/postgres \
  --set resources.requests.memory=512Mi \
  --set resources.requests.cpu=500m \
  --set resources.limits.memory=2Gi \
  --set resources.limits.cpu=2000m
```

### Installation with Specific PostgreSQL Version

```bash
helm install postgres flanksource/postgres \
  --set version="16" \
  --set image.tag="16"
```

### Installation with Custom Storage

```bash
helm install postgres flanksource/postgres \
  --set persistence.size=50Gi \
  --set persistence.storageClass=fast-ssd
```

## Connecting to PostgreSQL

### From within the cluster

Get the connection details:

```bash
export POSTGRES_PASSWORD=$(kubectl get secret postgres-password -o jsonpath="{.data.password}" | base64 --decode)
export POSTGRES_HOST=$(kubectl get service postgres -o jsonpath="{.status.loadBalancer.ingress[0].ip}")

# Connect using psql
kubectl run postgres-client --rm --tty -i --restart='Never' --image postgres:17 \
  --env="PGPASSWORD=$POSTGRES_PASSWORD" \
  --command -- psql -h postgres -U postgres -d postgres
```

### Port forwarding for local access

```bash
kubectl port-forward svc/postgres 5432:5432
export POSTGRES_PASSWORD=$(kubectl get secret postgres-password -o jsonpath="{.data.password}" | base64 --decode)
psql -h localhost -U postgres -d postgres
```

## Configuration Examples

### Disable Auto-upgrade

```yaml
autoUpgrade:
  enabled: false
```

### Custom PostgreSQL Configuration

```yaml
conf:
  max_connections: 200
  shared_buffers: 2GB
  effective_cache_size: 6GB
  log_statement: "all"
  log_min_duration_statement: "1s"
```

### Using Existing PVC

```yaml
persistence:
  enabled: true
  existingClaim: "my-existing-pvc"
```

### Node Affinity and Tolerations

```yaml
nodeSelector:
  disktype: ssd

tolerations:
  - key: "database"
    operator: "Equal"
    value: "postgres"
    effect: "NoSchedule"
```

## Upgrade Notes

When upgrading the chart, the PostgreSQL version can be automatically upgraded if `autoUpgrade.enabled` is set to `true` (default). The upgrade process is handled by the `postgres-cli` tool which:

1. Detects the current PostgreSQL version
2. Performs pg_upgrade if needed
3. Updates the data directory structure
4. Restarts PostgreSQL with the new version

To upgrade the chart:

```bash
helm upgrade postgres flanksource/postgres --set version="17"
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| autoUpgrade.enabled | bool | `true` |  |
| conf.listen_addresses | string | `"*"` |  |
| conf.log_autovacuum_min_duration | string | `"10s"` |  |
| conf.log_connections | string | `"off"` |  |
| conf.log_destination | string | `"stderr"` |  |
| conf.log_directory | string | `"/var/log/postgresql"` |  |
| conf.log_disconnections | string | `"off"` |  |
| conf.log_file_mode | int | `420` |  |
| conf.log_filename | string | `"postgresql-%d.log"` |  |
| conf.log_line_prefix | string | `"%m [%p] %q[user=%u,db=%d,app=%a]"` |  |
| conf.log_lock_waits | string | `"on"` |  |
| conf.log_min_duration_statement | string | `"1s"` |  |
| conf.log_statement | string | `"all"` |  |
| conf.log_temp_files | string | `"100MB"` |  |
| conf.log_timezone | string | `"UTC"` |  |
| conf.logging_collector | string | `"off"` |  |
| conf.password_encryption | string | `"scram-sha-256"` |  |
| conf.ssl | string | `"off"` |  |
| conf.timezone | string | `"UTC"` |  |
| database.name | string | `"postgres"` |  |
| database.username | string | `"postgres"` |  |
| dshmSize | string | `"256Mi"` |  |
| env | list | `[]` |  |
| extraVolumeMounts | list | `[]` |  |
| extraVolumes | list | `[]` |  |
| fullnameOverride | string | `""` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.registry | string | `"ghcr.io"` |  |
| image.repository | string | `"flanksource/postgres"` |  |
| image.tag | string | `"17"` |  |
| imagePullSecrets | list | `[]` |  |
| livenessProbe.enabled | bool | `true` |  |
| livenessProbe.failureThreshold | int | `3` |  |
| livenessProbe.initialDelaySeconds | int | `10` |  |
| livenessProbe.periodSeconds | int | `30` |  |
| livenessProbe.successThreshold | int | `1` |  |
| livenessProbe.timeoutSeconds | int | `5` |  |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` | Node selector |
| passwordRef.create | bool | `true` |  |
| passwordRef.key | string | `"password"` |  |
| passwordRef.secretName | string | `"postgres-password"` |  |
| persistence.accessMode | string | `"ReadWriteOnce"` |  |
| persistence.annotations | object | `{}` |  |
| persistence.enabled | bool | `true` |  |
| persistence.existingClaim | string | `""` |  |
| persistence.size | string | `"10Gi"` |  |
| persistence.storageClass | string | `""` |  |
| persistence.volumeName | string | `""` |  |
| podAnnotations | object | `{}` |  |
| podLabels | object | `{}` |  |
| podSecurityContext.fsGroup | int | `999` |  |
| podSecurityContext.fsGroupChangePolicy | string | `"OnRootMismatch"` |  |
| podSecurityContext.runAsGroup | int | `999` |  |
| podSecurityContext.runAsUser | int | `999` |  |
| postgresCliArgs | string | `"--pg-tune --auto-upgrade --auto-reset-password  --auto-init"` | custom arguments to pass to postgres-cli startup |
| readinessProbe.enabled | bool | `true` |  |
| readinessProbe.failureThreshold | int | `3` |  |
| readinessProbe.initialDelaySeconds | int | `5` |  |
| readinessProbe.periodSeconds | int | `10` |  |
| readinessProbe.successThreshold | int | `1` |  |
| readinessProbe.timeoutSeconds | int | `5` |  |
| resetVolumePermissions | bool | `true` | Change ownership and permissions of mounted volumes on startup |
| resources.limits.cpu | string | `"2000m"` |  |
| resources.limits.memory | string | `"4Gi"` |  |
| resources.requests.cpu | string | `"20m"` |  |
| resources.requests.memory | string | `"128Mi"` |  |
| securityContext.allowPrivilegeEscalation | bool | `false` |  |
| securityContext.capabilities.drop[0] | string | `"ALL"` |  |
| securityContext.readOnlyRootFilesystem | bool | `false` |  |
| securityContext.runAsNonRoot | bool | `true` |  |
| securityContext.runAsUser | int | `999` |  |
| service.annotations | object | `{}` |  |
| service.port | int | `5432` |  |
| service.type | string | `"ClusterIP"` |  |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |
| startupProbe.enabled | bool | `true` |  |
| startupProbe.failureThreshold | int | `30` |  |
| startupProbe.initialDelaySeconds | int | `10` |  |
| startupProbe.periodSeconds | int | `10` |  |
| startupProbe.successThreshold | int | `1` |  |
| startupProbe.timeoutSeconds | int | `5` |  |
| tolerations | list | `[]` |  |
| version | string | `"17"` | Postgres version to run, flanksource/postgres images include multiple versions allowing you to select at runtime |
