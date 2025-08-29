#!/bin/bash
# PostgreSQL Upgrade Verification Script
# Usage: verify.sh <version> [database]

VERSION=$1
DATABASE=${2:-postgres}

if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version> [database]"
    exit 1
fi

echo "🔍 Starting verification for PostgreSQL $VERSION"

# Start PostgreSQL
echo "Starting PostgreSQL..."
pg_ctl -D /var/lib/postgresql/data start -w || {
    echo "ERROR: Failed to start PostgreSQL"
    exit 1
}

# Check version
echo "Checking PostgreSQL version..."
actual_version=$(psql -U postgres -t -c "SHOW server_version;" | grep -oE "^[0-9]+" | head -1)
if [ "$actual_version" != "$VERSION" ]; then
    echo "ERROR: Version mismatch. Expected $VERSION, got $actual_version"
    pg_ctl -D /var/lib/postgresql/data stop -w
    exit 1
fi
echo "✅ PostgreSQL version: $VERSION"

# Check task log table
if psql -U postgres -d postgres -c "SELECT 1 FROM postgres_task_log LIMIT 1;" > /dev/null 2>&1; then
    echo "✅ Task log table verified"
    
    # Log verification
    psql -U postgres -d postgres -c "
        INSERT INTO postgres_task_log (task_name, task_type, status, details) 
        VALUES ('verify-v$VERSION', 'verification', 'completed', json_build_object('version', $VERSION, 'database', '$DATABASE'));"
    
    # Show recent tasks
    echo ""
    echo "📋 Recent tasks:"
    psql -U postgres -d postgres -c "
        SELECT task_name, status, to_char(started_at, 'HH24:MI:SS') as time 
        FROM postgres_task_log 
        ORDER BY started_at DESC 
        LIMIT 5;"
else
    echo "⚠️  Task log table not found (may be first run)"
fi

# Check test data if exists
if psql -U postgres -d $DATABASE -c "SELECT 1 FROM test_upgrade LIMIT 1;" > /dev/null 2>&1; then
    count=$(psql -U postgres -d $DATABASE -tAc "SELECT COUNT(*) FROM test_upgrade;")
    echo "✅ Test data verified: $count records"
    
    # Check view
    if psql -U postgres -d $DATABASE -c "SELECT 1 FROM test_upgrade_summary LIMIT 1;" > /dev/null 2>&1; then
        total=$(psql -U postgres -d $DATABASE -tAc "SELECT total_records FROM test_upgrade_summary;")
        echo "✅ Test view verified: $total total records"
    fi
fi

# Stop PostgreSQL
echo "Stopping PostgreSQL..."
pg_ctl -D /var/lib/postgresql/data stop -w
echo "✅ Verification completed successfully"