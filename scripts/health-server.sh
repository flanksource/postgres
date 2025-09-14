#!/bin/bash
# Health check server script
# Provides health endpoints for container health monitoring

set -e

# Default health check port
HEALTH_PORT=${HEALTH_PORT:-8080}

# Function to check PostgreSQL health
check_postgres() {
    pg_isready -U postgres -h localhost > /dev/null 2>&1
    return $?
}

# Function to check PgBouncer health
check_pgbouncer() {
    if pgrep -x "pgbouncer" > /dev/null; then
        return 0
    else
        return 1
    fi
}

# Function to check PostgREST health
check_postgrest() {
    if pgrep -x "postgrest" > /dev/null; then
        return 0
    else
        return 1
    fi
}

# Simple HTTP server for health checks
echo "Starting health check server on port $HEALTH_PORT"

while true; do
    # Wait for incoming connection
    { 
        # Read the request
        read -r method path version
        
        # Process different health check endpoints
        case "$path" in
            /health|/healthz)
                # Basic health check
                if check_postgres; then
                    echo -e "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\nHealthy"
                else
                    echo -e "HTTP/1.1 503 Service Unavailable\r\nContent-Type: text/plain\r\n\r\nUnhealthy"
                fi
                ;;
            /ready|/readyz)
                # Readiness check - all services should be running
                if check_postgres && check_pgbouncer && check_postgrest; then
                    echo -e "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\nReady"
                else
                    echo -e "HTTP/1.1 503 Service Unavailable\r\nContent-Type: text/plain\r\n\r\nNot Ready"
                fi
                ;;
            /postgres)
                # PostgreSQL specific health check
                if check_postgres; then
                    echo -e "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\nPostgreSQL is healthy"
                else
                    echo -e "HTTP/1.1 503 Service Unavailable\r\nContent-Type: text/plain\r\n\r\nPostgreSQL is unhealthy"
                fi
                ;;
            *)
                echo -e "HTTP/1.1 404 Not Found\r\nContent-Type: text/plain\r\n\r\nNot Found"
                ;;
        esac
    } | nc -l -p "$HEALTH_PORT" -q 1
done