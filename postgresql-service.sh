#!/bin/bash

# PostgreSQL startup script for supervisord
set -e

# Function to log with PostgreSQL prefix
log() {
    echo "[postgresql] $*" >&2
}

log "🐘 Starting PostgreSQL service with extensions support..."

# Import environment variables from Docker environment
log "🔍 Debug: Current environment variables:"
env | grep -E '^(POSTGRES_|PGBOUNCER_|POSTGREST_)' || log "🔍 Debug: No POSTGRES environment variables found in current environment"

# Set PGDATA if not already set
export PGDATA="${PGDATA:-/var/lib/postgresql/data}"

# Ensure PostgreSQL data directory exists and has correct permissions
if [ ! -d "$PGDATA" ]; then
    log "Creating PostgreSQL data directory: $PGDATA"
    mkdir -p "$PGDATA"
    chown postgres:postgres "$PGDATA"
    chmod 700 "$PGDATA"
fi

# Initialize database if needed
if [ ! -f "$PGDATA/PG_VERSION" ]; then
    log "Initializing PostgreSQL database..."
    su postgres -c "initdb -D '$PGDATA'"

    # Set PostgreSQL password during initial setup if provided
    if [ -n "${POSTGRES_PASSWORD:-}" ]; then
        echo "Setting PostgreSQL password for user ${POSTGRES_USER:-postgres}..."
        su postgres -c "postgres --single -D '$PGDATA' postgres" << EOF
ALTER USER ${POSTGRES_USER:-postgres} PASSWORD '$POSTGRES_PASSWORD';
EOF
        echo "Password set successfully during initialization"
    fi

    # Create POSTGRES_DB if it doesn't exist and is different from 'postgres'
    postgres_db="${POSTGRES_DB:-postgres}"
    if [ "$postgres_db" != 'postgres' ]; then
        echo "Creating database: $postgres_db"
        su postgres -c "postgres --single -D '$PGDATA' postgres" << EOF
CREATE DATABASE "$postgres_db";
EOF
        echo "Database $postgres_db created successfully"
    fi
fi

# Apply any pending upgrades (if enabled)
if [ "${AUTO_UPGRADE:-true}" = "true" ] && [ -f "$PGDATA/PG_VERSION" ]; then
    current_version=$(cat "$PGDATA/PG_VERSION")
    target_version="${TARGET_VERSION=:-17}"

    if [ "$current_version" != "$target_version" ]; then
        echo "Upgrading PostgreSQL from version $current_version to $target_version"
        # Run upgrade logic using existing task system
        cd /var/lib/postgresql
        su postgres -c "task auto-upgrade"
    fi
fi

# Configure PostgreSQL for extensions
log "Configuring PostgreSQL for extensions..."

# Set up authentication method - use trust for localhost, md5 for external
auth_method_external="${POSTGRES_HOST_AUTH_METHOD:-md5}"

# Create pg_hba.conf if it doesn't exist or needs updating
if [ ! -f "$PGDATA/pg_hba.conf" ] || [ "${RESET_CONFIG:-false}" = "true" ] || ! grep -q "0.0.0.0/0" "$PGDATA/pg_hba.conf" 2>/dev/null; then
    log "Updating pg_hba.conf with trust for localhost, $auth_method_external for external connections..."
    cat > "$PGDATA/pg_hba.conf" << EOF
# TYPE  DATABASE        USER            ADDRESS                 METHOD
local   all             postgres                                peer
local   all             all                                     peer
host    all             all             127.0.0.1/32            trust
host    all             all             ::1/128                 trust
host    all             all             0.0.0.0/0               $auth_method_external
EOF
fi

# Configure PostgreSQL settings for extensions
if [ ! -f "$PGDATA/postgresql.conf.extensions" ] || [ "${RESET_CONFIG:-false}" = "true" ]; then
    cat > "$PGDATA/postgresql.conf.extensions" << EOF
# Extensions configuration
listen_addresses = '*'
port = 5432
max_connections = ${POSTGRES_MAX_CONNECTIONS:-100}

# Memory settings
shared_buffers = ${POSTGRES_SHARED_BUFFERS:-128MB}
effective_cache_size = ${POSTGRES_EFFECTIVE_CACHE_SIZE:-512MB}

# WAL settings
wal_level = replica
max_wal_senders = 3
archive_mode = ${POSTGRES_ARCHIVE_MODE:-off}
archive_command = '${POSTGRES_ARCHIVE_COMMAND:-/bin/true}'

# Extensions that need preloading
shared_preload_libraries = 'pg_cron,pg_stat_monitor,pgaudit'

# Extension-specific settings
cron.database_name = '${POSTGRES_DB:-postgres}'
pgaudit.log = 'all'
pgaudit.log_level = 'notice'
pg_stat_monitor.pgsm_query_max_len = 2048

# Logging
log_destination = 'stderr'
log_statement = 'none'
log_duration = off
log_line_prefix = '%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h '

# Include custom config if it exists
include_if_exists = '/etc/postgresql/postgresql.conf'
EOF

    # Include the extensions configuration in main postgresql.conf
    if ! grep -q "include.*postgresql.conf.extensions" "$PGDATA/postgresql.conf"; then
        echo "include = 'postgresql.conf.extensions'" >> "$PGDATA/postgresql.conf"
    fi
fi

# Start PostgreSQL temporarily to initialize extensions
if [ -f "/var/lib/postgresql/.extensions_to_install" ]; then
    echo "🧩 Installing extensions..."

    # Start PostgreSQL in single-user mode for extension installation
    extensions=$(cat /var/lib/postgresql/.extensions_to_install)

    # Extensions that need to be installed
    IFS=',' read -ra EXT_ARRAY <<< "$extensions"
    for ext in "${EXT_ARRAY[@]}"; do
        ext=$(echo "$ext" | xargs)  # Trim whitespace

        # Map extension names
        case "$ext" in
            pgvector) ext_name="vector" ;;
            pg_safeupdate) ext_name="safeupdate" ;;
            *) ext_name="$ext" ;;
        esac

        log "  Installing extension: $ext_name"
        su postgres -c "postgres --single -D '$PGDATA' -c shared_preload_libraries= -c listen_addresses=" << EOF || log "    Warning: Failed to install $ext_name"
CREATE EXTENSION IF NOT EXISTS $ext_name CASCADE;
EOF

        # Special post-installation setup
        case "$ext" in
            pg_cron)
                su postgres -c "postgres --single -D '$PGDATA' -c shared_preload_libraries= -c listen_addresses=" << EOF || true
GRANT USAGE ON SCHEMA cron TO postgres;
EOF
                ;;
        esac
    done

    # Remove the marker file
    rm -f /var/lib/postgresql/.extensions_to_install
fi

# Process initialization scripts in /docker-entrypoint-initdb.d/
if [ ! -f "$PGDATA/.init_scripts_processed" ] && [ -d "/docker-entrypoint-initdb.d" ]; then
    echo "🚀 Processing initialization scripts..."

    # Start PostgreSQL temporarily for running init scripts
    for f in /docker-entrypoint-initdb.d/*; do
        case "$f" in
            *.sh)
                if [ -x "$f" ]; then
                    echo "Running $f"
                    "$f"
                else
                    echo "Sourcing $f"
                    . "$f"
                fi
                ;;
            *.sql)
                echo "Running $f"
                su postgres -c "postgres --single -D '$PGDATA' '$postgres_db'" < "$f"
                ;;
            *.sql.gz)
                echo "Running $f"
                gunzip -c "$f" | su postgres -c "postgres --single -D '$PGDATA' '$postgres_db'"
                ;;
            *)
                echo "Ignoring $f"
                ;;
        esac
        echo
    done

    # Mark initialization scripts as processed
    touch "$PGDATA/.init_scripts_processed"
    echo "✅ Initialization scripts processed"
fi

# Set PostgreSQL password if provided and not already set
if [ -n "${POSTGRES_PASSWORD:-}" ]; then
    postgres_user="${POSTGRES_USER:-postgres}"
    echo "🔑 Ensuring PostgreSQL password is set for user $postgres_user..."

    # Try to set the password (this will work even if already set)
    su postgres -c "postgres --single -D '$PGDATA' postgres" << EOF || echo "Password might already be set correctly"
ALTER USER $postgres_user PASSWORD '$POSTGRES_PASSWORD';
EOF
    echo "✅ Password configuration completed for user $postgres_user"
fi

# Create POSTGRES_DB if it doesn't exist and is different from 'postgres'
postgres_db="${POSTGRES_DB:-postgres}"
if [ "$postgres_db" != 'postgres' ]; then
    echo "🗄️ Ensuring database exists: $postgres_db"
    su postgres -c "postgres --single -D '$PGDATA' postgres" << EOF || echo "Database might already exist"
CREATE DATABASE "$postgres_db";
EOF
    echo "✅ Database configuration completed: $postgres_db"
fi

log "🚀 Starting PostgreSQL server..."

# Start PostgreSQL in foreground
exec su postgres -c "postgres -D '$PGDATA' -c config_file='$PGDATA/postgresql.conf'"
