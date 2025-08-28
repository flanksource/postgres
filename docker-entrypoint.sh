#!/bin/bash
set -e

# PostgreSQL Auto-Upgrade Docker Entrypoint
#
# This entrypoint auto-detects the PostgreSQL version in the data directory
# and upgrades it to the target version specified by PG_VERSION env var
#
# Usage: docker-entrypoint.sh
#
# Environment Variables:
#   PG_VERSION=17 (default) - Target PostgreSQL version
#   AUTO_UPGRADE=false - Disable auto-upgrade
#   RESET_PASSWORD=true - Reset password on startup
#   POSTGRES_PASSWORD - New password for reset
#   POSTGRES_USER=postgres - User for password reset

# Check if arguments are provided (legacy mode) or use auto-detection
if [ $# -eq 2 ]; then
    # Legacy mode: FROM_VERSION TO_VERSION provided as arguments
    FROM_VERSION="$1"
    PG_VERSION="$2"
    echo "🔧 Legacy mode: upgrading to $PG_VERSION"

elif [ $# -eq 0 ]; then
    # Auto-detection mode - use the new Taskfile-based approach
    echo "🚀 PostgreSQL Auto-Upgrade Entrypoint"

    # Map legacy environment variables for backward compatibility
    if [ -n "$TARGET_VERSION" ]; then
        export PG_VERSION="$TARGET_VERSION"
        echo "⚠️  Using legacy TARGET_VERSION, consider switching to PG_VERSION"
    fi
fi

exec task auto-upgrade
