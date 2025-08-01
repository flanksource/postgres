#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to verify upgraded data
verify_upgrade() {
    local original_version=$1
    local target_version=$2
    local volume_name=$3
    
    echo -e "${YELLOW}Verifying upgrade from PostgreSQL ${original_version} to ${target_version}...${NC}"
    
    # Start a container with the target version to check the data
    local container_name="verify-upgrade-${original_version}-to-${target_version}"
    
    docker rm -f ${container_name} 2>/dev/null || true
    
    docker run -d \
        --name ${container_name} \
        -e POSTGRES_PASSWORD=testpass \
        -e POSTGRES_USER=testuser \
        -e POSTGRES_DB=testdb \
        -v ${volume_name}:/var/lib/postgresql/data \
        postgres:${target_version}
    
    # Wait for PostgreSQL to be ready
    echo -n "Waiting for PostgreSQL ${target_version} to be ready..."
    for i in {1..30}; do
        if docker exec ${container_name} pg_isready -U testuser >/dev/null 2>&1; then
            echo -e " ${GREEN}Ready!${NC}"
            break
        fi
        echo -n "."
        sleep 1
    done
    
    # Check PostgreSQL version
    echo "Checking PostgreSQL version..."
    actual_version=$(docker exec ${container_name} psql -U testuser -d testdb -t -c "SHOW server_version;" | grep -oE '^[0-9]+' | head -1)
    
    if [ "$actual_version" = "$target_version" ]; then
        echo -e "${GREEN}✓ PostgreSQL version is ${target_version}${NC}"
    else
        echo -e "${RED}✗ PostgreSQL version mismatch. Expected ${target_version}, got ${actual_version}${NC}"
        docker stop ${container_name}
        docker rm ${container_name}
        return 1
    fi
    
    # Verify data integrity
    echo "Verifying data integrity..."
    
    # Check if tables exist
    table_count=$(docker exec ${container_name} psql -U testuser -d testdb -t -c "SELECT COUNT(*) FROM pg_tables WHERE schemaname = 'public';")
    if [ $table_count -ge 2 ]; then
        echo -e "${GREEN}✓ Tables exist${NC}"
    else
        echo -e "${RED}✗ Tables missing${NC}"
        docker stop ${container_name}
        docker rm ${container_name}
        return 1
    fi
    
    # Check test data
    record_count=$(docker exec ${container_name} psql -U testuser -d testdb -t -c "SELECT COUNT(*) FROM test_table;" | tr -d ' ')
    if [ "$record_count" = "3" ]; then
        echo -e "${GREEN}✓ Test data intact (${record_count} records)${NC}"
    else
        echo -e "${RED}✗ Test data corrupted. Expected 3 records, found ${record_count}${NC}"
        docker stop ${container_name}
        docker rm ${container_name}
        return 1
    fi
    
    # Check version info
    original_stored=$(docker exec ${container_name} psql -U testuser -d testdb -t -c "SELECT original_version FROM version_info WHERE id = 1;" | tr -d ' ')
    if [ "$original_stored" = "$original_version" ]; then
        echo -e "${GREEN}✓ Original version info preserved (${original_stored})${NC}"
    else
        echo -e "${RED}✗ Version info corrupted. Expected ${original_version}, found ${original_stored}${NC}"
        docker stop ${container_name}
        docker rm ${container_name}
        return 1
    fi
    
    # Display sample data
    echo "Sample data after upgrade:"
    docker exec ${container_name} psql -U testuser -d testdb -c "SELECT * FROM test_table LIMIT 2;"
    
    # Clean up
    docker stop ${container_name}
    docker rm ${container_name}
    
    echo -e "${GREEN}✓ Upgrade from PostgreSQL ${original_version} to ${target_version} verified successfully!${NC}\n"
    return 0
}

# Main verification logic
if [ $# -eq 3 ]; then
    verify_upgrade $1 $2 $3
else
    echo "Usage: $0 <original_version> <target_version> <volume_name>"
    echo "Example: $0 14 17 pg14-test-data"
    exit 1
fi