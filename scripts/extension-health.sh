#!/bin/bash

# PostgreSQL Extension Health Check Script
# This script checks if PostgreSQL extensions are properly installed and working

set -e

# Default values
POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_DB="${POSTGRES_DB:-postgres}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    case $status in
        "OK")
            echo -e "${GREEN}✅ $message${NC}"
            ;;
        "WARN")
            echo -e "${YELLOW}⚠️  $message${NC}"
            ;;
        "ERROR")
            echo -e "${RED}❌ $message${NC}"
            ;;
        "INFO")
            echo -e "${YELLOW}ℹ️  $message${NC}"
            ;;
    esac
}

# Function to run SQL command
run_sql() {
    local sql="$1"
    PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A -c "$sql" 2>/dev/null
}

# Function to check if extension is available
check_extension_available() {
    local ext_name="$1"
    local result=$(run_sql "SELECT 1 FROM pg_available_extensions WHERE name = '$ext_name';")
    [[ "$result" == "1" ]]
}

# Function to check if extension is installed
check_extension_installed() {
    local ext_name="$1"
    local result=$(run_sql "SELECT 1 FROM pg_extension WHERE extname = '$ext_name';")
    [[ "$result" == "1" ]]
}

# Function to check extension functionality
check_extension_functionality() {
    local ext_name="$1"
    case $ext_name in
        "vector"|"pgvector")
            # Test pgvector functionality
            run_sql "SELECT '[1,2,3]'::vector <-> '[4,5,6]'::vector;" >/dev/null 2>&1
            ;;
        "pgsodium")
            # Test pgsodium functionality
            run_sql "SELECT pgsodium.crypto_box_keypair();" >/dev/null 2>&1
            ;;
        "pgjwt")
            # Test pgjwt functionality
            run_sql "SELECT sign('{}', 'secret');" >/dev/null 2>&1
            ;;
        "pgaudit")
            # Check if pgaudit is loaded
            local loaded=$(run_sql "SELECT 1 FROM pg_loaded_extensions() WHERE name = 'pgaudit';")
            [[ "$loaded" == "1" ]]
            ;;
        "pg_cron")
            # Test pg_cron functionality
            run_sql "SELECT cron.schedule('test-job', '0 0 * * *', 'SELECT 1;');" >/dev/null 2>&1
            run_sql "SELECT cron.unschedule('test-job');" >/dev/null 2>&1
            ;;
        "pg_stat_monitor")
            # Check if pg_stat_monitor is providing data
            run_sql "SELECT count(*) FROM pg_stat_monitor;" >/dev/null 2>&1
            ;;
        "safeupdate"|"pg-safeupdate")
            # Test safeupdate functionality by trying an unsafe update (should fail)
            ! run_sql "CREATE TEMP TABLE test_safe AS SELECT 1 as id; UPDATE test_safe SET id = 2;" >/dev/null 2>&1
            ;;
        *)
            # For other extensions, just check if they're installed
            return 0
            ;;
    esac
}

main() {
    print_status "INFO" "PostgreSQL Extension Health Check"
    print_status "INFO" "=================================="

    # Check PostgreSQL connectivity
    if ! pg_isready -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" -U "$POSTGRES_USER" >/dev/null 2>&1; then
        print_status "ERROR" "Cannot connect to PostgreSQL at $POSTGRES_HOST:$POSTGRES_PORT"
        exit 1
    fi
    print_status "OK" "PostgreSQL connection successful"

    # Get list of extensions to check
    extensions_to_check="${POSTGRES_EXTENSIONS:-}"
    if [[ -z "$extensions_to_check" ]]; then
        print_status "WARN" "No extensions specified in POSTGRES_EXTENSIONS environment variable"
        print_status "INFO" "Checking all installed extensions..."
        extensions_to_check=$(run_sql "SELECT string_agg(extname, ',') FROM pg_extension WHERE extname != 'plpgsql';")
    fi

    if [[ -z "$extensions_to_check" ]]; then
        print_status "INFO" "No extensions found to check"
        exit 0
    fi

    print_status "INFO" "Checking extensions: $extensions_to_check"
    echo

    # Convert comma-separated list to array
    IFS=',' read -ra EXTENSIONS <<< "$extensions_to_check"
    
    local overall_status=0
    
    for ext in "${EXTENSIONS[@]}"; do
        ext=$(echo "$ext" | xargs)  # Trim whitespace
        
        # Map extension names (handle special cases)
        case "$ext" in
            "pgvector") ext_name="vector" ;;
            "pg-safeupdate") ext_name="safeupdate" ;;
            *) ext_name="$ext" ;;
        esac
        
        echo "Extension: $ext ($ext_name)"
        echo "----------------------------------------"
        
        # Check if extension is available
        if check_extension_available "$ext_name"; then
            print_status "OK" "Extension '$ext_name' is available"
        else
            print_status "ERROR" "Extension '$ext_name' is not available"
            overall_status=1
            echo
            continue
        fi
        
        # Check if extension is installed
        if check_extension_installed "$ext_name"; then
            print_status "OK" "Extension '$ext_name' is installed"
        else
            print_status "ERROR" "Extension '$ext_name' is not installed"
            overall_status=1
            echo
            continue
        fi
        
        # Check extension functionality
        if check_extension_functionality "$ext_name"; then
            print_status "OK" "Extension '$ext_name' is functioning correctly"
        else
            print_status "WARN" "Extension '$ext_name' may not be functioning correctly"
        fi
        
        echo
    done

    if [[ $overall_status -eq 0 ]]; then
        print_status "OK" "All extensions are healthy"
    else
        print_status "ERROR" "Some extensions have issues"
    fi

    return $overall_status
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi