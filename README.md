# PostgreSQL Upgrade Docker Images

This repository provides Docker images that automatically upgrade PostgreSQL data directories to newer versions. The images support upgrading from PostgreSQL 14, 15, or 16 to versions 15, 16, or 17.

## Available Images

The images are published to GitHub Container Registry with the following tags:

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