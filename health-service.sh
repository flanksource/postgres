#!/bin/bash

# Health check service script for supervisord
# Provides HTTP health endpoints for monitoring PostgreSQL services

set -e

# Source the enhanced logging library if available, fallback to simple logging
if [[ -f "/usr/local/bin/service-logging.sh" ]]; then
    source /usr/local/bin/service-logging.sh
    init_service_logging "health-service" "1.0.0"
    
    # Use enhanced logging functions
    log() { log_info "$*"; }
    log_error() { log_error_with_context "$1" "${2:-health_service_error}"; }
    log_warn() { log_warn "$*"; }
    log_debug() { log_debug "$*"; }
else
    # Fallback to simple logging
    log() {
        echo "[health-service] $*" >&2
    }
    log_error() { log "ERROR: $*"; }
    log_warn() { log "WARN: $*"; }
    log_debug() { log "DEBUG: $*"; }
fi

# Check if health service is enabled
if [[ "${HEALTH_SERVICE_ENABLED:-false}" != "true" ]]; then
    log "Health service is disabled, exiting..."
    exit 0
fi

log "🏥 Starting health check service..."

# Configuration
HEALTH_PORT="${HEALTH_PORT:-8080}"
HEALTH_HOST="${HEALTH_HOST:-0.0.0.0}"

log "Health service will listen on ${HEALTH_HOST}:${HEALTH_PORT}"

# Wait for other services to be ready before starting health server
log "Waiting for other services to initialize..."
sleep 10

# Start the health server
log "Starting health HTTP server..."

# Set final health status if enhanced logging is available
if command -v set_health_status >/dev/null 2>&1; then
    set_health_status "HEALTHY" "Health service is ready to serve requests" 2>/dev/null || true
fi

# Execute health server (this will replace the current process)
exec /usr/local/bin/health-server.sh server "$HEALTH_PORT"