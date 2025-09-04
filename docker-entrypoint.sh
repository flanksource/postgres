#!/bin/bash
set -e

# PostgreSQL Auto-Upgrade Docker Entrypoint
#
# This entrypoint auto-detects the PostgreSQL version in the data directory
# and upgrades it to the target version specified by PG_VERSION env var
#
# Usage: docker-entrypoint.sh [FROM_VERSION TO_VERSION]
#        docker-entrypoint.sh [command ...]
#
# Environment Variables:
#   PG_VERSION=17 (default) - Target PostgreSQL version
#   AUTO_UPGRADE=false - Disable auto-upgrade
#   POSTGRES_PASSWORD - PostgreSQL password (or use POSTGRES_PASSWORD_FILE)
#   POSTGRES_PASSWORD_FILE - File containing PostgreSQL password
#   POSTGRES_USER=postgres - PostgreSQL superuser (default: postgres)
#   POSTGRES_DB - Initial database name (default: $POSTGRES_USER)
#   POSTGRES_HOST_AUTH_METHOD - Authentication method (default: scram-sha-256)
#   START_POSTGRES=false - Start PostgreSQL after successful upgrade
#   POSTGRES_EXTENSIONS - Comma-separated list of extensions to install
#   PGBOUNCER_ENABLED=true - Enable PgBouncer connection pooling
#   POSTGREST_ENABLED=true - Enable PostgREST API server
#   WALG_ENABLED=false - Enable WAL-G backup functionality

# file_env - allow reading environment variables from files
# usage: file_env VAR [DEFAULT]
#    ie: file_env 'XYZ_DB_PASSWORD' 'example'
# (will allow for "$XYZ_DB_PASSWORD_FILE" to fill in the value of
#  "$XYZ_DB_PASSWORD" from a file, especially for Docker's secrets feature)
file_env() {
	local var="$1"
	local fileVar="${var}_FILE"
	local def="${2:-}"
	if [ "${!var:-}" ] && [ "${!fileVar:-}" ]; then
		printf >&2 'error: both %s and %s are set (but are exclusive)\n' "$var" "$fileVar"
		exit 1
	fi
	local val="$def"
	if [ "${!var:-}" ]; then
		val="${!var}"
	elif [ "${!fileVar:-}" ]; then
		val="$(< "${!fileVar}")"
	fi
	export "$var"="$val"
	unset "$fileVar"
}

# docker_setup_env - set up environment variables with defaults
docker_setup_env() {
	file_env 'POSTGRES_PASSWORD' "${POSTGRES_PASSWORD:-}"
	file_env 'POSTGRES_USER' 'postgres'
	file_env 'POSTGRES_DB' "$POSTGRES_USER"
	
	# Set authentication method default
	: "${POSTGRES_HOST_AUTH_METHOD:=}"
	
	# Validate password is set unless using trust authentication
	if [ "$POSTGRES_HOST_AUTH_METHOD" != 'trust' ] && [ -z "${POSTGRES_PASSWORD:-}" ]; then
		echo >&2 'error: database is uninitialized and password option is not specified '
		echo >&2 '  You need to specify one of POSTGRES_PASSWORD, POSTGRES_PASSWORD_FILE, or POSTGRES_HOST_AUTH_METHOD=trust'
		exit 1
	fi
}

# Function to initialize PostgreSQL extensions
init_extensions() {
    local extensions="${POSTGRES_EXTENSIONS:-}"
    if [ -n "$extensions" ]; then
        echo "🧩 Initializing PostgreSQL extensions: $extensions"
        
        # Map of extension names for special cases
        declare -A EXTENSION_MAP=(
            ["pgvector"]="vector"
            ["pgsodium"]="pgsodium"
            ["pgjwt"]="pgjwt"
            ["pgaudit"]="pgaudit"
            ["pg_tle"]="pg_tle"
            ["pg_stat_monitor"]="pg_stat_monitor"
            ["pg_repack"]="pg_repack"
            ["pg_plan_filter"]="pg_plan_filter"
            ["pg_net"]="pg_net"
            ["pg_jsonschema"]="pg_jsonschema"
            ["pg_hashids"]="pg_hashids"
            ["pg_cron"]="pg_cron"
            ["pg_safeupdate"]="safeupdate"
            ["index_advisor"]="index_advisor"
            ["wal2json"]="wal2json"
        )
        
        # Parse comma-separated list and install extensions
        IFS=',' read -ra EXT_ARRAY <<< "$extensions"
        for ext in "${EXT_ARRAY[@]}"; do
            ext=$(echo "$ext" | xargs)  # Trim whitespace
            ext_name="${EXTENSION_MAP[$ext]:-$ext}"
            
            echo "  Creating extension: $ext_name"
            
            # Special handling for certain extensions
            case "$ext" in
                pg_cron)
                    psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "CREATE EXTENSION IF NOT EXISTS pg_cron CASCADE;" 2>/dev/null || {
                        echo "    Warning: Failed to create extension $ext_name"
                        continue
                    }
                    psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "GRANT USAGE ON SCHEMA cron TO postgres;" 2>/dev/null || true
                    ;;
                pgsodium)
                    psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "CREATE EXTENSION IF NOT EXISTS pgsodium CASCADE;" 2>/dev/null || {
                        echo "    Warning: Failed to create extension $ext_name"
                        continue
                    }
                    # Create pgsodium key if not exists
                    psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT pgsodium.create_key();" 2>/dev/null || true
                    ;;
                *)
                    psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "CREATE EXTENSION IF NOT EXISTS $ext_name CASCADE;" 2>/dev/null || {
                        echo "    Warning: Failed to create extension $ext_name"
                        continue
                    }
                    ;;
            esac
        done
        
        # List installed extensions
        echo "📋 Installed extensions:"
        psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT extname, extversion FROM pg_extension WHERE extname NOT IN ('plpgsql') ORDER BY extname;" 2>/dev/null || true
    fi
}

# Function to check if we should use supervisord
use_supervisord() {
    # Use supervisord if any additional services are enabled
    if [ "${PGBOUNCER_ENABLED:-false}" = "true" ] || \
       [ "${POSTGREST_ENABLED:-false}" = "true" ] || \
       [ "${WALG_ENABLED:-false}" = "true" ] || \
       [ -n "${POSTGRES_EXTENSIONS:-}" ]; then
        return 0
    else
        return 1
    fi
}

# If first argument starts with a dash or is a known command, execute it directly
if [ "${1:0:1}" = '-' ] || [ "$1" = "task" ] || [ "$1" = "bash" ] || [ "$1" = "sh" ] || [ "$1" = "postgres" ] || [ "$1" = "supervisord" ]; then
    exec "$@"
fi

# Check if arguments are provided for upgrade mode
if [ $# -eq 2 ] && [[ "$1" =~ ^[0-9]+$ ]] && [[ "$2" =~ ^[0-9]+$ ]]; then
    # Legacy mode: FROM_VERSION TO_VERSION provided as arguments
    FROM_VERSION="$1"
    TO_VERSION="$2"
    echo "🔧 Legacy mode: upgrading from PostgreSQL $FROM_VERSION to $TO_VERSION"
    
    # Delegate to docker-upgrade-multi script for legacy compatibility
    exec /usr/local/bin/docker-upgrade-multi "$FROM_VERSION" "$TO_VERSION"
    
elif [ $# -eq 0 ]; then
    # Auto-detection mode - check if we should use s6-overlay or traditional approach
    echo "🚀 PostgreSQL Enhanced Entrypoint"
    
    # Set up environment variables with PostgreSQL Docker standards
    docker_setup_env
    
    # Map legacy environment variables for backward compatibility
    if [ -n "$TARGET_VERSION" ]; then
        export PG_VERSION="$TARGET_VERSION"
        echo "⚠️  Using legacy TARGET_VERSION, consider switching to PG_VERSION"
    fi
    
    # Check if we should use supervisord for service management
    if use_supervisord; then
        echo "🔧 Using supervisord for service management"
        echo "📋 Services enabled:"
        [ "${PGBOUNCER_ENABLED:-false}" = "true" ] && echo "  - PgBouncer (connection pooling)"
        [ "${POSTGREST_ENABLED:-false}" = "true" ] && echo "  - PostgREST (REST API)"
        [ "${WALG_ENABLED:-false}" = "true" ] && echo "  - WAL-G (backup)"
        [ -n "${POSTGRES_EXTENSIONS:-}" ] && echo "  - Extensions: ${POSTGRES_EXTENSIONS}"
        
        # Create extension initialization marker for services
        if [ -n "${POSTGRES_EXTENSIONS:-}" ]; then
            echo "$POSTGRES_EXTENSIONS" > /var/lib/postgresql/.extensions_to_install
        fi
        
        # Set environment variables for supervisord
        export PGBOUNCER_ENABLED="${PGBOUNCER_ENABLED:-false}"
        export POSTGREST_ENABLED="${POSTGREST_ENABLED:-false}"
        export WALG_ENABLED="${WALG_ENABLED:-false}"
        
        # Delegate to supervisord
        exec /usr/bin/supervisord -c /etc/supervisord.conf
    else
        echo "🔧 Using traditional upgrade mode (no additional services)"
        
        # Switch to postgres user for PostgreSQL operations if running as root
        if [ "$(id -u)" = '0' ]; then
            # Ensure we can read the data directory first
            if [ ! -f "/var/lib/postgresql/data/PG_VERSION" ]; then
                echo "❌ No PostgreSQL data found in /var/lib/postgresql/data"
                echo "Please mount a volume with existing PostgreSQL data"
                exit 1
            fi
            
            # Switch to postgres user for all PostgreSQL operations
            exec gosu postgres task auto-upgrade
        else
            # Already running as non-root user
            exec task auto-upgrade
        fi
    fi
    
else
    echo "❌ Invalid number of arguments"
    echo "Usage: $0 [FROM_VERSION TO_VERSION]"
    echo "  - No arguments: Auto-detect version from data directory and upgrade to PG_VERSION (default: 17)"
    echo "  - Two arguments: FROM_VERSION TO_VERSION (legacy mode)"
    echo ""
    echo "Environment Variables:"
    echo "  PG_VERSION=17           - Target PostgreSQL version (default: 17)"
    echo "  AUTO_UPGRADE=false      - Disable auto-upgrade (default: true)"
    echo "  RESET_PASSWORD=true     - Reset password on startup (default: false)"
    echo "  POSTGRES_PASSWORD=...   - New password for reset"
    echo "  POSTGRES_USER=postgres  - User for password reset (default: postgres)"
    echo "  START_POSTGRES=true     - Start PostgreSQL after upgrade (default: false)"
    echo ""
    echo "Legacy Variables (still supported):"
    echo "  TARGET_VERSION=17       - Same as PG_VERSION (deprecated)"
    exit 1
fi
