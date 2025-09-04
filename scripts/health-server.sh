#!/bin/bash

# Simple HTTP health endpoints server
# Provides health status and metrics for PgBouncer and PostgREST services
# Usage: ./health-server.sh [port]

set -e

# Configuration
HEALTH_PORT="${1:-8080}"
HEALTH_HOST="${HEALTH_HOST:-0.0.0.0}"
HEALTH_LOG_LEVEL="${HEALTH_LOG_LEVEL:-INFO}"

# Source the enhanced logging library if available
if [[ -f "/usr/local/bin/service-logging.sh" ]]; then
    source /usr/local/bin/service-logging.sh
    init_service_logging "health-server" "1.0.0"
else
    # Fallback logging
    log_info() { echo "[health-server] INFO: $*" >&2; }
    log_warn() { echo "[health-server] WARN: $*" >&2; }
    log_error() { echo "[health-server] ERROR: $*" >&2; }
    log_debug() { echo "[health-server] DEBUG: $*" >&2; }
fi

# Function to check if a port is open
check_port() {
    local host=$1
    local port=$2
    timeout 3 bash -c "echo >/dev/tcp/$host/$port" 2>/dev/null
}

# Function to check PostgreSQL health
check_postgres_health() {
    local status="healthy"
    local details=""
    
    if pg_isready -h localhost -p 5432 -U "${POSTGRES_USER:-postgres}" >/dev/null 2>&1; then
        if PGPASSWORD="${POSTGRES_PASSWORD}" psql -h localhost -p 5432 \
            -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-postgres}" \
            -c "SELECT 1;" >/dev/null 2>&1; then
            details="PostgreSQL is responding and accepting connections"
        else
            status="unhealthy"
            details="PostgreSQL is running but authentication failed"
        fi
    else
        status="unhealthy"
        details="PostgreSQL is not responding"
    fi
    
    echo "{\"service\":\"postgresql\",\"status\":\"$status\",\"details\":\"$details\"}"
}

# Function to check PgBouncer health
check_pgbouncer_health() {
    if [[ "${PGBOUNCER_ENABLED:-false}" != "true" ]]; then
        echo "{\"service\":\"pgbouncer\",\"status\":\"disabled\",\"details\":\"PgBouncer is not enabled\"}"
        return
    fi
    
    local status="healthy"
    local details=""
    local port="${PGBOUNCER_PORT:-6432}"
    
    if check_port localhost "$port"; then
        if PGPASSWORD="${POSTGRES_PASSWORD}" psql -h localhost -p "$port" \
            -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-postgres}" \
            -c "SHOW VERSION;" >/dev/null 2>&1; then
            details="PgBouncer is responding and accepting connections on port $port"
        else
            status="unhealthy"
            details="PgBouncer is running but connections are failing"
        fi
    else
        status="unhealthy"
        details="PgBouncer is not responding on port $port"
    fi
    
    echo "{\"service\":\"pgbouncer\",\"status\":\"$status\",\"details\":\"$details\"}"
}

# Function to check PostgREST health
check_postgrest_health() {
    if [[ "${POSTGREST_ENABLED:-false}" != "true" ]]; then
        echo "{\"service\":\"postgrest\",\"status\":\"disabled\",\"details\":\"PostgREST is not enabled\"}"
        return
    fi
    
    local status="healthy"
    local details=""
    local port="${POSTGREST_SERVER_PORT:-3000}"
    
    if check_port localhost "$port"; then
        if command -v curl >/dev/null 2>&1; then
            if curl -f -s -m 3 "http://localhost:$port/" >/dev/null 2>&1; then
                details="PostgREST API is responding on port $port"
            else
                status="unhealthy"
                details="PostgREST is running but API requests are failing"
            fi
        else
            details="PostgREST is responding on port $port (curl not available for API test)"
        fi
    else
        status="unhealthy"
        details="PostgREST is not responding on port $port"
    fi
    
    echo "{\"service\":\"postgrest\",\"status\":\"$status\",\"details\":\"$details\"}"
}

# Function to get overall health status
get_overall_health() {
    local postgres_health=$(check_postgres_health)
    local pgbouncer_health=$(check_pgbouncer_health)
    local postgrest_health=$(check_postgrest_health)
    
    local overall_status="healthy"
    local services_array="[$postgres_health,$pgbouncer_health,$postgrest_health]"
    
    # Check if any service is unhealthy
    if echo "$services_array" | grep -q '"status":"unhealthy"'; then
        overall_status="unhealthy"
    fi
    
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S.%3NZ")
    
    echo "{\"timestamp\":\"$timestamp\",\"overall_status\":\"$overall_status\",\"services\":$services_array}"
}

# Function to get basic metrics
get_metrics() {
    local metrics=""
    
    # Add basic service info
    metrics+="# HELP service_info Service information\n"
    metrics+="# TYPE service_info gauge\n"
    metrics+="service_info{service=\"postgresql\",enabled=\"true\"} 1\n"
    
    if [[ "${PGBOUNCER_ENABLED:-false}" == "true" ]]; then
        metrics+="service_info{service=\"pgbouncer\",enabled=\"true\"} 1\n"
        
        # Try to get PgBouncer pool stats
        local pgbouncer_stats=""
        pgbouncer_stats=$(PGPASSWORD="${POSTGRES_PASSWORD}" psql -h localhost -p "${PGBOUNCER_PORT:-6432}" \
            -U "${POSTGRES_USER:-postgres}" -d "pgbouncer" \
            -t -A -c "SHOW POOLS;" 2>/dev/null || echo "")
        
        if [[ -n "$pgbouncer_stats" ]]; then
            while IFS= read -r line; do
                if [[ -n "$line" && "$line" != "database|user|cl_active|cl_waiting|sv_active|sv_idle|sv_used|sv_tested|sv_login|maxwait|maxwait_us|pool_mode" ]]; then
                    IFS='|' read -r db user cl_active cl_waiting sv_active sv_idle sv_used sv_tested sv_login maxwait maxwait_us pool_mode <<< "$line"
                    metrics+="pgbouncer_client_active{database=\"$db\",user=\"$user\"} ${cl_active:-0}\n"
                    metrics+="pgbouncer_client_waiting{database=\"$db\",user=\"$user\"} ${cl_waiting:-0}\n"
                    metrics+="pgbouncer_server_active{database=\"$db\",user=\"$user\"} ${sv_active:-0}\n"
                    metrics+="pgbouncer_server_idle{database=\"$db\",user=\"$user\"} ${sv_idle:-0}\n"
                fi
            done <<< "$pgbouncer_stats"
        fi
    else
        metrics+="service_info{service=\"pgbouncer\",enabled=\"false\"} 1\n"
    fi
    
    if [[ "${POSTGREST_ENABLED:-false}" == "true" ]]; then
        metrics+="service_info{service=\"postgrest\",enabled=\"true\"} 1\n"
    else
        metrics+="service_info{service=\"postgrest\",enabled=\"false\"} 1\n"
    fi
    
    # Add PostgreSQL connection count if available
    if PGPASSWORD="${POSTGRES_PASSWORD}" psql -h localhost -p 5432 \
        -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-postgres}" \
        -t -A -c "SELECT count(*) FROM pg_stat_activity;" 2>/dev/null >/dev/null; then
        local conn_count=$(PGPASSWORD="${POSTGRES_PASSWORD}" psql -h localhost -p 5432 \
            -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-postgres}" \
            -t -A -c "SELECT count(*) FROM pg_stat_activity;" 2>/dev/null | tr -d ' ')
        metrics+="postgresql_connections_active $conn_count\n"
    fi
    
    echo -e "$metrics"
}

# Function to handle HTTP requests
handle_request() {
    local method=$1
    local path=$2
    local response_code="200 OK"
    local content_type="application/json"
    local response_body=""
    
    case "$path" in
        "/health" | "/health/")
            response_body=$(get_overall_health)
            ;;
        "/health/postgres" | "/health/postgresql")
            response_body=$(check_postgres_health)
            ;;
        "/health/pgbouncer")
            response_body=$(check_pgbouncer_health)
            ;;
        "/health/postgrest")
            response_body=$(check_postgrest_health)
            ;;
        "/metrics")
            content_type="text/plain; version=0.0.4; charset=utf-8"
            response_body=$(get_metrics)
            ;;
        "/")
            response_body="{\"message\":\"Health check server\",\"endpoints\":[\"/health\",\"/health/postgres\",\"/health/pgbouncer\",\"/health/postgrest\",\"/metrics\"]}"
            ;;
        *)
            response_code="404 Not Found"
            response_body="{\"error\":\"Not found\",\"path\":\"$path\"}"
            ;;
    esac
    
    # Calculate content length
    local content_length=${#response_body}
    
    # Send HTTP response
    echo "HTTP/1.1 $response_code"
    echo "Content-Type: $content_type"
    echo "Content-Length: $content_length"
    echo "Connection: close"
    echo "Server: health-server/1.0"
    echo ""
    echo "$response_body"
}

# Function to parse HTTP request
parse_and_handle_request() {
    local request_line=""
    read -r request_line
    
    # Parse the request line (e.g., "GET /health HTTP/1.1")
    local method=$(echo "$request_line" | cut -d' ' -f1)
    local path=$(echo "$request_line" | cut -d' ' -f2)
    
    # Skip the rest of the HTTP headers
    while read -r line && [[ "$line" != $'\r' ]] && [[ -n "$line" ]]; do
        continue
    done
    
    # Log the request
    log_info "HTTP $method $path"
    
    # Handle the request
    handle_request "$method" "$path"
}

# Function to start the HTTP server
start_server() {
    log_info "Starting health check server on $HEALTH_HOST:$HEALTH_PORT"
    log_info "Available endpoints: /health, /health/postgres, /health/pgbouncer, /health/postgrest, /metrics"
    
    # Check if the logging library is available for enhanced features
    if command -v set_health_status >/dev/null 2>&1; then
        set_health_status "HEALTHY" "Health check server is running" 2>/dev/null || true
    fi
    
    # Use netcat or socat to create a simple HTTP server
    if command -v socat >/dev/null 2>&1; then
        log_info "Using socat for HTTP server"
        while true; do
            echo -e "HTTP/1.1 200 OK\nContent-Type: text/plain\nConnection: close\n\nHealth server is running" | \
            socat TCP-LISTEN:$HEALTH_PORT,fork,reuseaddr EXEC:"/bin/bash -c 'parse_and_handle_request'"
        done
    elif command -v nc >/dev/null 2>&1; then
        log_info "Using netcat for HTTP server"
        while true; do
            nc -l -p "$HEALTH_PORT" -c 'parse_and_handle_request' 2>/dev/null || \
            nc -l "$HEALTH_PORT" -e '/bin/bash -c parse_and_handle_request' 2>/dev/null || {
                # Fallback for different nc implementations
                mkfifo /tmp/http_pipe_$$
                while true; do
                    nc -l -p "$HEALTH_PORT" < /tmp/http_pipe_$$ | parse_and_handle_request > /tmp/http_pipe_$$
                done
            }
        done
    else
        log_error "Neither socat nor netcat (nc) is available. Cannot start HTTP server."
        log_info "Available health check functions can still be used directly:"
        log_info "  - check_postgres_health"
        log_info "  - check_pgbouncer_health" 
        log_info "  - check_postgrest_health"
        log_info "  - get_overall_health"
        exit 1
    fi
}

# Function to run health checks and exit (non-server mode)
run_health_checks() {
    local format="${1:-json}"
    
    if [[ "$format" == "json" ]]; then
        get_overall_health
    else
        echo "=== Service Health Status ==="
        check_postgres_health | jq -r '"PostgreSQL: " + .status + " - " + .details' 2>/dev/null || echo "PostgreSQL: Check failed"
        check_pgbouncer_health | jq -r '"PgBouncer: " + .status + " - " + .details' 2>/dev/null || echo "PgBouncer: Check failed"
        check_postgrest_health | jq -r '"PostgREST: " + .status + " - " + .details' 2>/dev/null || echo "PostgREST: Check failed"
    fi
}

# Main function
main() {
    case "${1:-server}" in
        "server")
            start_server
            ;;
        "check")
            run_health_checks "${2:-json}"
            ;;
        "metrics")
            get_metrics
            ;;
        "--help" | "-h")
            echo "Health Check Server for PostgreSQL Services"
            echo ""
            echo "Usage: $0 [command] [options]"
            echo ""
            echo "Commands:"
            echo "  server [port]     Start HTTP health check server (default port: 8080)"
            echo "  check [format]    Run health checks once and exit (format: json|text)"
            echo "  metrics          Output Prometheus metrics and exit"
            echo ""
            echo "Environment variables:"
            echo "  HEALTH_HOST              Server bind address (default: 0.0.0.0)"
            echo "  HEALTH_LOG_LEVEL         Log level (default: INFO)"
            echo "  PGBOUNCER_ENABLED        Enable PgBouncer checks (default: false)"
            echo "  POSTGREST_ENABLED        Enable PostgREST checks (default: false)"
            echo ""
            echo "HTTP Endpoints (when running as server):"
            echo "  GET /health              Overall health status"
            echo "  GET /health/postgres     PostgreSQL health status"
            echo "  GET /health/pgbouncer    PgBouncer health status"
            echo "  GET /health/postgrest    PostgREST health status"
            echo "  GET /metrics             Prometheus metrics"
            ;;
        *)
            if [[ "$1" =~ ^[0-9]+$ ]]; then
                HEALTH_PORT="$1"
                start_server
            else
                echo "Unknown command: $1"
                echo "Use '$0 --help' for usage information"
                exit 1
            fi
            ;;
    esac
}

# Export functions so they can be used by subprocesses
export -f parse_and_handle_request handle_request get_overall_health check_postgres_health check_pgbouncer_health check_postgrest_health get_metrics check_port
export -f log_info log_warn log_error log_debug

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi