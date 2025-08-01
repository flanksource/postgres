#!/bin/bash
set -e

# This entrypoint auto-detects the PostgreSQL version in the data directory
# and upgrades it to the latest available version (17)

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

# Arguments are no longer supported - we auto-detect only
if [ $# -ge 1 ]; then
    echo "Error: This container now only supports auto-detection mode"
    echo "Please mount your PostgreSQL data volume to /var/lib/postgresql/data"
    exit 1
fi

# Otherwise, auto-detect version
echo "Auto-detecting PostgreSQL version in data directory..."

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
        exit 0
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