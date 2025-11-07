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
            echo "Example: docker run -v <volume>:/var/lib/postgresql/data flanksource/postgres:17"
            exit 0
        fi
    fi

    echo "Continuing as root (not recommended). Consider restarting as postgres user."
fi

export PGBIN=/usr/lib/postgresql/${PG_VERSION}/bin

# Set defaults for PostgreSQL environment variables
POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_DB="${POSTGRES_DB:-$POSTGRES_USER}"

if [ ! -f $PGDATA/PG_VERSION ]; then
    echo "Initializing database cluster at $PGDATA ..."
    echo "Using POSTGRES_USER: $POSTGRES_USER"

    # Initialize database with specified user and password
    if [ -n "$POSTGRES_PASSWORD" ]; then
        echo "Initializing with password authentication"
        $PGBIN/initdb -D $PGDATA -U "$POSTGRES_USER" --pwfile=<(printf "%s\n" "$POSTGRES_PASSWORD")
    else
        echo "WARNING: No password set. Initializing without password authentication."
        $PGBIN/initdb -D $PGDATA -U "$POSTGRES_USER"
    fi

    # Start PostgreSQL temporarily to create database
    $PGBIN/pg_ctl start -D $PGDATA --wait

    # Create custom database if specified and different from default
    if [ "$POSTGRES_DB" != "postgres" ]; then
        echo "Creating database: $POSTGRES_DB"
        $PGBIN/psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<-EOSQL
			CREATE DATABASE "$POSTGRES_DB" OWNER "$POSTGRES_USER";
		EOSQL
    fi

    $PGBIN/pg_ctl stop -D $PGDATA --wait
else
    echo "Database cluster already initialized at $PGDATA with version $(cat $PGDATA/PG_VERSION)"

fi

postgres-cli server status

# Run postgres-cli auto-start (includes permission checks)
postgres-cli auto-start  --upgrade-to=$PG_VERSION --data-dir "$PGDATA" --report-caller $POSTGRES_CLI_ARGS


if [ "$UPGRADE_ONLY" = "true" ]; then
    echo "UPGRADE_ONLY is set. Exiting after upgrade."
    exit 0
fi

echo "Starting PostgreSQL server with command: $PGBIN/postgres $@"

# Start PostgreSQL server
exec "$PGBIN/postgres" "$@"
