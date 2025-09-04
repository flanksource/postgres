#!/bin/bash

# PostgREST startup script for supervisord
set -e

# Source the enhanced logging library if available, fallback to simple logging
if [[ -f "/usr/local/bin/service-logging.sh" ]]; then
    source /usr/local/bin/service-logging.sh
    init_service_logging "postgrest" "11.2.0"

    # Use enhanced logging functions
    log() { log_info "$*"; }
    log_error() { log_error_with_context "$1" "${2:-postgrest_error}"; }
    log_warn() { log_warn "$*"; }
    log_debug() { log_debug "$*"; }
else
    # Fallback to simple logging
    log() {
        echo "[postgrest] $*" >&2
    }
    log_error() { log "ERROR: $*"; }
    log_warn() { log "WARN: $*"; }
    log_debug() { log "DEBUG: $*"; }
fi

# Check if PostgREST is enabled
if [ "${POSTGREST_ENABLED:-false}" != "true" ]; then
    log "PostgREST is disabled, exiting..."
    exit 0
fi

log "🚀 Starting PostgREST API server..."


if [ -e "$POSTGREST_JWT_SECRET_FILE" ]; then
    POSTGREST_JWT_SECRET=$(cat "$POSTGREST_JWT_SECRET_FILE")
    export POSTGREST_JWT_SECRET
fi

# Generate JWT secret if not provided
if [  "${POSTGREST_JWT_SECRET}" == "random" ]; then
    POSTGREST_JWT_SECRET=$(openssl rand --hex 16 | tr -d '\n')
    export POSTGREST_JWT_SECRET
    log "🔑 Generated JWT secret for PostgREST:"
    log "   POSTGREST_JWT_SECRET=${POSTGREST_JWT_SECRET}"
    log "   ⚠️  Save this secret for client authentication!"

fi

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

END_TIME=$(date +%s%3N 2>/dev/null || date +%s)
DURATION=$((END_TIME - START_TIME))
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
    sleep 5
done

log "Database connection successful"



mkdir -p /etc/postgrest


if [ ! -e /etc/postgrest/postgrest.conf ] ; then

log "Generating PostgREST configuration..."
cat > /etc/postgrest/postgrest.conf << EOF
# Database connection
db-uri = "postgres://${POSTGRES_USER:-postgres}:${POSTGRES_PASSWORD}@localhost:5432/${POSTGRES_DB:-postgres}"

# Database schema to expose via API
db-schemas = "${POSTGREST_DB_SCHEMAS:-public}"
db-anon-role = "${POSTGREST_DB_ANON_ROLE:-postgres}"

# Server configuration
server-host = "${POSTGREST_SERVER_HOST:-0.0.0.0}"
server-port = ${POSTGREST_SERVER_PORT:-3000}

# JWT configuration (optional)
jwt-secret = "${POSTGREST_JWT_SECRET}"
jwt-aud = "${POSTGREST_JWT_AUD:-}"

# OpenAPI configuration
openapi-mode = "${POSTGREST_OPENAPI_MODE:-follow-privileges}"
openapi-security-active = ${POSTGREST_OPENAPI_SECURITY_ACTIVE:-false}

# Logging
log-level = "${POSTGREST_LOG_LEVEL:-info}"

# Connection pool
db-pool = ${POSTGREST_DB_POOL:-10}
db-pool-acquisition-timeout = ${POSTGREST_DB_POOL_TIMEOUT:-10}

# Raw media types
raw-media-types = "${POSTGREST_RAW_MEDIA_TYPES:-}"
EOF
chmod 600 /etc/postgrest/postgrest.conf


fi

mkdir -p /var/log/postgrest
# Also ensure the directory is owned by postgrest
chown -R postgres:postgres /etc/postgrest
chown -R postgres:postgres /var/log/postgrest


exec su postgres -c "postgrest"
