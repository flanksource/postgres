#!/bin/bash
set -e

# This entrypoint auto-detects the PostgreSQL version in the data directory
# and upgrades it to the target version specified by TARGET_VERSION env var

# Function to detect PostgreSQL version from data directory
detect_postgres_version() {
    local data_dir="/var/lib/postgresql/data"
    
    # Check if data directory exists and has PG_VERSION file
    if [ -f "$data_dir/PG_VERSION" ]; then
        local version=$(cat "$data_dir/PG_VERSION")
        echo "$version"
    else
        return 1
    fi
}

# Check if arguments are provided (legacy mode) or use auto-detection
if [ $# -eq 2 ]; then
    # Legacy mode: FROM_VERSION TO_VERSION provided as arguments
    FROM_VERSION="$1"
    TO_VERSION="$2"
    echo "Using legacy mode: upgrading from PostgreSQL $FROM_VERSION to $TO_VERSION"
    
    # Set environment variables for upgrade tasks
    export FROM_VERSION=$FROM_VERSION
    export TO_VERSION=$TO_VERSION
    
    # Run the upgrade using the multi-upgrade script
    exec /usr/local/bin/docker-upgrade-multi "$FROM_VERSION" "$TO_VERSION"
elif [ $# -eq 0 ]; then
    # Auto-detection mode: detect version from data directory
    echo "Auto-detecting PostgreSQL version in data directory..."
else
    echo "Error: Invalid number of arguments"
    echo "Usage: $0 [FROM_VERSION TO_VERSION]"
    echo "  - No arguments: Auto-detect version from data directory"
    echo "  - Two arguments: FROM_VERSION TO_VERSION (legacy mode)"
    exit 1
fi

# Auto-detection mode continues here

# Look for existing PostgreSQL data
if [ -f "/var/lib/postgresql/data/PG_VERSION" ]; then
    FROM_VERSION=$(cat /var/lib/postgresql/data/PG_VERSION)
    echo "Detected PostgreSQL version: $FROM_VERSION"
    
    # Map the data to the correct version directory
    case "$FROM_VERSION" in
        "14")
            mkdir -p /var/lib/postgresql/14/data
            cp -a /var/lib/postgresql/data/* /var/lib/postgresql/14/data/
            ;;
        "15")
            mkdir -p /var/lib/postgresql/15/data
            cp -a /var/lib/postgresql/data/* /var/lib/postgresql/15/data/
            ;;
        "16")
            mkdir -p /var/lib/postgresql/16/data
            cp -a /var/lib/postgresql/data/* /var/lib/postgresql/16/data/
            ;;
        *)
            echo "Error: Unsupported PostgreSQL version $FROM_VERSION"
            exit 1
            ;;
    esac
    
    # Set target version from environment or default to 17
    TO_VERSION="${TARGET_VERSION:-17}"
    
    # Validate target version
    if [ "$TO_VERSION" != "15" ] && [ "$TO_VERSION" != "16" ] && [ "$TO_VERSION" != "17" ]; then
        echo "Error: Invalid target version $TO_VERSION. Must be 15, 16, or 17."
        exit 1
    fi
    
    # Check if upgrade is needed
    if [ "$FROM_VERSION" -ge "$TO_VERSION" ]; then
        echo "PostgreSQL $FROM_VERSION is already at or above target version $TO_VERSION"
        
        # Check if password reset is requested
        if [ "$RESET_PASSWORD" = "true" ]; then
            echo "Password reset requested for existing installation"
            # Start PostgreSQL temporarily to reset password
            echo "Starting PostgreSQL temporarily for password reset..."
            sudo -u postgres /usr/lib/postgresql/$FROM_VERSION/bin/pg_ctl -D /var/lib/postgresql/data -l /tmp/postgres.log start
            sleep 5
            
            echo "Resetting PostgreSQL password..."
            sudo -u postgres /usr/lib/postgresql/$FROM_VERSION/bin/psql -c "ALTER USER ${POSTGRES_USER:-postgres} PASSWORD '${POSTGRES_PASSWORD}';"
            
            echo "Stopping PostgreSQL..."
            sudo -u postgres /usr/lib/postgresql/$FROM_VERSION/bin/pg_ctl -D /var/lib/postgresql/data stop
            echo "Password reset completed"
        fi
        
        # Start normal PostgreSQL service
        echo "Starting PostgreSQL $FROM_VERSION in normal mode..."
        exec sudo -u postgres /usr/lib/postgresql/$FROM_VERSION/bin/postgres -D /var/lib/postgresql/data
    fi
    
    # Check if auto-upgrade is disabled
    if [ "$AUTO_UPGRADE" = "false" ]; then
        echo "Auto-upgrade is disabled. Starting PostgreSQL $FROM_VERSION without upgrade."
        exec sudo -u postgres /usr/lib/postgresql/$FROM_VERSION/bin/postgres -D /var/lib/postgresql/data
    fi
    
    echo "Will upgrade from PostgreSQL $FROM_VERSION to $TO_VERSION"
    
    # Set environment variables for upgrade tasks
    export FROM_VERSION=$FROM_VERSION
    export TO_VERSION=$TO_VERSION
    
    # Copy initial data to version-specific directory
    echo "📦 Copying initial data to PostgreSQL $FROM_VERSION directory..."
    mkdir -p /var/lib/postgresql/$FROM_VERSION/data
    cp -a /var/lib/postgresql/data/* /var/lib/postgresql/$FROM_VERSION/data/
    
    # Fix PostgreSQL data directory permissions (required for pg_upgrade)
    chmod 700 /var/lib/postgresql/$FROM_VERSION/data
    chown postgres:postgres /var/lib/postgresql/$FROM_VERSION/data
    
    # Run the upgrade using Taskfile
    current=$FROM_VERSION
    target=$TO_VERSION
    
    while [ $current -lt $target ]; do
      next=$((current + 1))
      echo "========================================="
      echo "Upgrading from PostgreSQL $current to $next"
      echo "========================================="
      
      export FROM=$current
      export TO=$next
      task upgrade-from-env
      
      current=$next
    done
    
    # Copy final result back to main mount
    echo "📦 Copying PostgreSQL $target data back to main data directory..."
    cp -a /var/lib/postgresql/$target/data/* /var/lib/postgresql/data/
    
    echo "✅ All upgrades completed successfully!"
else
    echo "Error: No PostgreSQL data found in /var/lib/postgresql/data"
    echo "Please mount a volume with existing PostgreSQL data to /var/lib/postgresql/data"
    exit 1
fi