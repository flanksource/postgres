#!/bin/bash
set -e

echo "Running PostgreSQL version tests in container..."

# Run the test in a container, overriding the entrypoint
docker run --rm --entrypoint bash postgres-upgrade:latest -c '
set -e

echo "=== PostgreSQL Version Check ==="
for version in 14 15 16 17; do
    echo -n "PostgreSQL $version: "
    /usr/lib/postgresql/$version/bin/postgres --version
done

echo
echo "=== Binary Availability Check ==="
for version in 14 15 16 17; do
    echo "PostgreSQL $version binaries:"
    ls -la /usr/lib/postgresql/$version/bin/postgres /usr/lib/postgresql/$version/bin/initdb /usr/lib/postgresql/$version/bin/pg_upgrade | head -3
    echo
done

echo "=== Other Tools Check ==="
echo -n "gosu: "
gosu --version

echo -n "task: "
task --version

echo -n "curl: "
curl --version | head -1

echo
echo "=== Key Files Check ==="
ls -la /usr/local/bin/docker-upgrade-multi
ls -la /var/lib/postgresql/Taskfile.yml

echo
echo "All checks completed successfully!"
'