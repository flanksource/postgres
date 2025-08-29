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
#   RESET_PASSWORD=true - Reset password on startup
#   POSTGRES_PASSWORD - New password for reset
#   POSTGRES_USER=postgres - User for password reset

# If first argument starts with a dash or is a known command, execute it directly
if [ "${1:0:1}" = '-' ] || [ "$1" = "task" ] || [ "$1" = "bash" ] || [ "$1" = "sh" ] || [ "$1" = "postgres" ]; then
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
    # Auto-detection mode - use the new Taskfile-based approach
    echo "🚀 PostgreSQL Auto-Upgrade Entrypoint"
    
    # Map legacy environment variables for backward compatibility
    if [ -n "$TARGET_VERSION" ]; then
        export PG_VERSION="$TARGET_VERSION"
        echo "⚠️  Using legacy TARGET_VERSION, consider switching to PG_VERSION"
    fi
    
    # Debug: Check what's in the data directory
    echo "=== Debug: Checking data directory ==="
    echo "Running as user: $(id -u):$(id -g)"
    echo "Data directory contents:"
    ls -la /var/lib/postgresql/data/ || echo "Cannot list data directory"
    echo "PG_VERSION file:"
    cat /var/lib/postgresql/data/PG_VERSION 2>/dev/null || echo "PG_VERSION not found"
    echo "=== End Debug ==="
    
    # Delegate to the main auto-upgrade task
    # Note: PostgreSQL commands will be run as postgres user when needed
    exec task auto-upgrade
    
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
    echo ""
    echo "Legacy Variables (still supported):"
    echo "  TARGET_VERSION=17       - Same as PG_VERSION (deprecated)"
    exit 1
fi
