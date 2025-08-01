#!/bin/bash
set -e

echo "Testing PostgreSQL installations in Docker image..."

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to test a PostgreSQL version
test_postgres_version() {
    local version=$1
    local bin_path="/usr/lib/postgresql/${version}/bin"
    local data_path="/tmp/pgtest${version}"
    
    echo -n "Testing PostgreSQL ${version}... "
    
    # Check if binaries exist
    if [ ! -f "${bin_path}/postgres" ]; then
        echo -e "${RED}FAIL${NC} - postgres binary not found at ${bin_path}/postgres"
        return 1
    fi
    
    if [ ! -f "${bin_path}/initdb" ]; then
        echo -e "${RED}FAIL${NC} - initdb binary not found at ${bin_path}/initdb"
        return 1
    fi
    
    # Check version
    local reported_version=$(${bin_path}/postgres --version | grep -oE '[0-9]+\.[0-9]+')
    local major_version=$(echo $reported_version | cut -d. -f1)
    
    if [ "$major_version" != "$version" ]; then
        echo -e "${RED}FAIL${NC} - Version mismatch. Expected ${version}, got ${reported_version}"
        return 1
    fi
    
    # Initialize a test database
    rm -rf "$data_path"
    mkdir -p "$data_path"
    chown postgres:postgres "$data_path"
    
    su - postgres -c "${bin_path}/initdb -D ${data_path}" > /dev/null 2>&1
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}FAIL${NC} - initdb failed"
        return 1
    fi
    
    # Try to start PostgreSQL
    su - postgres -c "${bin_path}/pg_ctl -D ${data_path} -l /tmp/pg${version}.log start" > /dev/null 2>&1
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}FAIL${NC} - Failed to start PostgreSQL"
        cat /tmp/pg${version}.log
        return 1
    fi
    
    # Wait for PostgreSQL to be ready
    sleep 2
    
    # Test connection
    su - postgres -c "${bin_path}/psql -p 5432 -c 'SELECT version();'" > /dev/null 2>&1
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}FAIL${NC} - Failed to connect to PostgreSQL"
        su - postgres -c "${bin_path}/pg_ctl -D ${data_path} stop" > /dev/null 2>&1
        return 1
    fi
    
    # Stop PostgreSQL
    su - postgres -c "${bin_path}/pg_ctl -D ${data_path} stop" > /dev/null 2>&1
    
    # Clean up
    rm -rf "$data_path"
    
    echo -e "${GREEN}PASS${NC}"
    return 0
}

# Test all PostgreSQL versions
all_passed=true

for version in 14 15 16 17; do
    if ! test_postgres_version $version; then
        all_passed=false
    fi
done

# Check if gosu is installed
echo -n "Testing gosu installation... "
if command -v gosu >/dev/null 2>&1; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC} - gosu not found"
    all_passed=false
fi

# Check if task is installed
echo -n "Testing task installation... "
if command -v task >/dev/null 2>&1; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC} - task not found"
    all_passed=false
fi

# Check if required scripts exist
echo -n "Testing docker-upgrade-multi script... "
if [ -x "/usr/local/bin/docker-upgrade-multi" ]; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC} - docker-upgrade-multi not found or not executable"
    all_passed=false
fi

echo -n "Testing Taskfile.yml... "
if [ -f "/var/lib/postgresql/Taskfile.yml" ]; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC} - Taskfile.yml not found"
    all_passed=false
fi

# Summary
echo
if [ "$all_passed" = true ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi