#!/bin/bash

# PgBouncer startup script for supervisord
set -e

# Source the enhanced logging library if available, fallback to simple logging
if [[ -f "/usr/local/bin/service-logging.sh" ]]; then
    source /usr/local/bin/service-logging.sh
    init_service_logging "pgbouncer" "1.21.0"

    # Use enhanced logging functions
    log() { log_info "$*"; }
    log_error() { log_error_with_context "$1" "${2:-pgbouncer_error}"; }
    log_warn() { log_warn "$*"; }
    log_debug() { log_debug "$*"; }
else
    # Fallback to simple logging
    log() {
        echo "[pgbouncer] $*" >&2
    }
    log_error() { log "ERROR: $*"; }
    log_warn() { log "WARN: $*"; }
    log_debug() { log "DEBUG: $*"; }
fi

# Check if PgBouncer is enabled
if [ "${PGBOUNCER_ENABLED:-false}" != "true" ]; then
    log "PgBouncer is disabled, exiting..."
    exit 0
fi

log "🏊 Starting PgBouncer connection pooler..."

# Wait for PostgreSQL to be ready with retry logic
log "Waiting for PostgreSQL to be ready..."
MAX_RETRIES=30
RETRY_COUNT=0
START_TIME=$(date +%s%3N 2>/dev/null || date +%s)

until pg_isready -h localhost -p 5432 -U ${POSTGRES_USER:-postgres} > /dev/null 2>&1; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        END_TIME=$(date +%s%3N 2>/dev/null || date +%s)
        DURATION=$((END_TIME - START_TIME))
        log_error "PostgreSQL did not become ready after $MAX_RETRIES attempts (${DURATION}ms)" "postgres_timeout"
        exit 1
    fi

    # Log every 10 attempts to reduce noise
    if [ $((RETRY_COUNT % 10)) -eq 0 ]; then
        log "PostgreSQL not ready (attempt $RETRY_COUNT/$MAX_RETRIES), waiting..."
    else
        log_debug "PostgreSQL not ready (attempt $RETRY_COUNT/$MAX_RETRIES)"
    fi
    sleep 2
done


log "PostgreSQL is accepting connections"

# Wait for PostgreSQL to be ready for authentication
log "Testing database connection with credentials..."
DB_RETRY_COUNT=0
DB_START_TIME=$(date +%s%3N 2>/dev/null || date +%s)

until PGPASSWORD="${POSTGRES_PASSWORD}" psql -h localhost -p 5432 -U ${POSTGRES_USER:-postgres} -d ${POSTGRES_DB:-postgres} -c "SELECT 1;" > /dev/null 2>&1; do
    DB_RETRY_COUNT=$((DB_RETRY_COUNT + 1))
    if [ $DB_RETRY_COUNT -ge 15 ]; then
        DB_END_TIME=$(date +%s%3N 2>/dev/null || date +%s)
        DB_DURATION=$((DB_END_TIME - DB_START_TIME))
        log_error "Could not authenticate to PostgreSQL after 15 attempts (${DB_DURATION}ms)" "auth_timeout"
        log_error "This might indicate an authentication configuration issue" "auth_config"
        exit 1
    fi

    # Log every 5 attempts to reduce noise
    if [ $((DB_RETRY_COUNT % 5)) -eq 0 ]; then
        log "Database authentication not ready (attempt $DB_RETRY_COUNT/15), waiting..."
    else
        log_debug "Database authentication not ready (attempt $DB_RETRY_COUNT/15)"
    fi
    sleep 2
done

log "Database connection successful"

# Generate PgBouncer configuration
log "Generating PgBouncer configuration..."

# Create configuration directory
mkdir -p /etc/pgbouncer

if [ ! -e /etc/pgbouncer/pgbouncer.ini ]; then
cat > /etc/pgbouncer/pgbouncer.ini << EOF
[databases]
${POSTGRES_DB:-postgres} = host=localhost port=5432 dbname=${POSTGRES_DB:-postgres}
* = host=localhost port=5432

[pgbouncer]
listen_port = ${PGBOUNCER_PORT:-6432}
listen_addr = 0.0.0.0
auth_type = ${PGBOUNCER_AUTH_TYPE:-md5}
auth_file = /etc/pgbouncer/userlist.txt
auth_query = SELECT usename, passwd FROM pg_shadow WHERE usename=\$1

admin_users = ${POSTGRES_USER:-postgres}
stats_users = stats, ${POSTGRES_USER:-postgres}

pool_mode = ${PGBOUNCER_POOL_MODE:-transaction}
server_reset_query = DISCARD ALL
server_check_query = select 1

max_client_conn = ${PGBOUNCER_MAX_CLIENT_CONN:-100}
default_pool_size = ${PGBOUNCER_DEFAULT_POOL_SIZE:-25}
min_pool_size = 0
reserve_pool_size = 0
reserve_pool_timeout = 5

server_lifetime = ${PGBOUNCER_SERVER_LIFETIME:-3600}
server_idle_timeout = ${PGBOUNCER_SERVER_IDLE_TIMEOUT:-600}

log_connections = 1
log_disconnections = 1
log_pooler_errors = 1
verbose = ${PGBOUNCER_VERBOSE:-0}

pidfile = /var/run/pgbouncer/pgbouncer.pid
unix_socket_dir = /var/run/postgresql
EOF

log "PgBouncer configuration generated successfully"

fi


if [ ! -e /etc/pgbouncer/userlist.txt ]; then
# Generate user list for authentication
log "Generating user authentication file..."


# Get password hash from PostgreSQL
POSTGRES_PASSWORD_HASH=""
if [ -n "$POSTGRES_PASSWORD" ]; then
    # Create MD5 hash for PgBouncer userlist
    POSTGRES_PASSWORD_HASH=$(echo -n "${POSTGRES_PASSWORD}${POSTGRES_USER:-postgres}" | md5sum | cut -d' ' -f1)
    POSTGRES_PASSWORD_HASH="md5${POSTGRES_PASSWORD_HASH}"
    log_debug "Generated MD5 hash for user authentication"
else
    # Try to get hash from PostgreSQL
    log "Attempting to retrieve password hash from PostgreSQL"
    POSTGRES_PASSWORD_HASH=$(PGPASSWORD="$POSTGRES_PASSWORD" psql -h localhost -p 5432 -U ${POSTGRES_USER:-postgres} -t -c "SELECT passwd FROM pg_shadow WHERE usename='${POSTGRES_USER:-postgres}';" 2>/dev/null | tr -d ' \n' || echo "")

    if [ -n "$POSTGRES_PASSWORD_HASH" ]; then
        log "Retrieved password hash from PostgreSQL"
    else
        log_warn "Could not retrieve password hash from PostgreSQL, authentication may fail"
    fi
fi

cat > /etc/pgbouncer/userlist.txt << EOF
"${POSTGRES_USER:-postgres}" "${POSTGRES_PASSWORD_HASH}"
EOF


log "User authentication file generated successfully"

fi
mkdir -p /var/run/pgbouncer
chown postgres:postgres /var/run/pgbouncer
log_debug "Created PID directory: /var/run/pgbouncer"

# Set proper permissions on configuration files
chown postgres:postgres /etc/pgbouncer/


# Start PgBouncer
log "PgBouncer configuration completed, starting process..."


exec su postgres -c "pgbouncer /etc/pgbouncer/pgbouncer.ini"
