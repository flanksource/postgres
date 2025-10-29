#!/bin/bash
set -e

echo "PGDATA is set to: $PGDATA"

# Detect current user
CURRENT_USER=$(id -u)
CURRENT_USER_NAME=$(id -un)

echo "Running as user: $CURRENT_USER_NAME (UID: $CURRENT_USER)"

# Check if running as root (not recommended)
if [ "$CURRENT_USER" = "0" ]; then
    echo "WARNING: Running as root is not recommended for security reasons."
    echo "This mode is only for fixing permission issues."

    # Check if PGDATA exists and has wrong ownership
    if [ -d "$PGDATA" ]; then
        PGDATA_OWNER=$(stat -c '%u' "$PGDATA" 2>/dev/null || stat -f '%u' "$PGDATA" 2>/dev/null || echo "999")
        if [ "$PGDATA_OWNER" != "999" ]; then
            echo "Fixing PGDATA ownership: $PGDATA (currently owned by UID $PGDATA_OWNER)"
            chown -R postgres:postgres "$PGDATA" "$PGCONFIG_CONFIG_DIR" /var/lib/postgresql 2>/dev/null || true
            echo "Permissions fixed. Please restart the container without --user root flag."
            echo "Example: docker run -v <volume>:/var/lib/postgresql/data flanksource/postgres:latest"
            exit 0
        fi
    fi

    echo "Continuing as root (not recommended). Consider restarting as postgres user."
fi

export PGBIN=/usr/lib/postgresql/${PG_VERSION}/bin


if [ ! -f $PGDATA/PG_VERSION ]; then
    echo "Initializing database cluster at $PGDATA ..."
    # starting and stopping the DB to initialize the directory for tuning
    $PGBIN/initdb -D $PGDATA
    $PGBIN/pg_ctl start -D $PGDATA --wait
    $PGBIN/pg_ctl stop -D $PGDATA --wait
else
    echo "Database cluster already initialized at $PGDATA with version $(cat $PGDATA/PG_VERSION)"

fi

postgres-cli server status

# Run postgres-cli auto-start (includes permission checks)
postgres-cli auto-start --pg-tune --auto-upgrade --upgrade-to=$PG_VERSION --auto-init --data-dir "$PGDATA" --auto-reset-password

cat $PGDATA/pg_hba.conf


if [ "$UPGRADE_ONLY" = "true" ]; then
    echo "UPGRADE_ONLY is set. Exiting after upgrade."
    exit 0
fi

echo "Starting PostgreSQL server with command: $PGBIN/postgres $@"

# Start PostgreSQL server
exec "$PGBIN/postgres" "$@"
