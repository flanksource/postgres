#!/bin/bash

# PostgreSQL Extension Initialization Script
# This script initializes PostgreSQL extensions from environment variables or command line

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

# Extension name mapping (handle special cases)
declare -A EXTENSION_MAP=(
    ["pgvector"]="vector"
    ["pg_safeupdate"]="safeupdate"
    ["pg-safeupdate"]="safeupdate"
)

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
    PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "$sql"
}

# Function to check if extension is available
check_extension_available() {
    local ext_name="$1"
    local result=$(PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A -c "SELECT 1 FROM pg_available_extensions WHERE name = '$ext_name';" 2>/dev/null)
    [[ "$result" == "1" ]]
}

# Function to check if extension is already installed
check_extension_installed() {
    local ext_name="$1"
    local result=$(PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A -c "SELECT 1 FROM pg_extension WHERE extname = '$ext_name';" 2>/dev/null)
    [[ "$result" == "1" ]]
}

# Function to install a single extension
install_extension() {
    local ext="$1"
    local ext_name="${EXTENSION_MAP[$ext]:-$ext}"
    
    print_status "INFO" "Installing extension: $ext ($ext_name)"
    
    # Check if extension is available
    if ! check_extension_available "$ext_name"; then
        print_status "ERROR" "Extension '$ext_name' is not available"
        return 1
    fi
    
    # Check if already installed
    if check_extension_installed "$ext_name"; then
        print_status "WARN" "Extension '$ext_name' is already installed"
        return 0
    fi
    
    # Install the extension
    if run_sql "CREATE EXTENSION IF NOT EXISTS $ext_name CASCADE;" >/dev/null 2>&1; then
        print_status "OK" "Extension '$ext_name' installed successfully"
        
        # Post-installation setup for specific extensions
        case "$ext" in
            "pg_cron")
                print_status "INFO" "Setting up pg_cron permissions..."
                run_sql "GRANT USAGE ON SCHEMA cron TO $POSTGRES_USER;" >/dev/null 2>&1 || true
                ;;
            "pgaudit")
                print_status "INFO" "pgaudit installed - configure pgaudit.* settings in postgresql.conf"
                ;;
            "pg_stat_monitor")
                print_status "INFO" "pg_stat_monitor installed - restart PostgreSQL to activate"
                ;;
        esac
        
        return 0
    else
        print_status "ERROR" "Failed to install extension '$ext_name'"
        return 1
    fi
}

# Function to get extension list from various sources
get_extension_list() {
    local extensions=""
    
    # Priority 1: Command line argument
    if [[ $# -gt 0 ]]; then
        extensions="$1"
    # Priority 2: POSTGRES_EXTENSIONS environment variable
    elif [[ -n "${POSTGRES_EXTENSIONS:-}" ]]; then
        extensions="$POSTGRES_EXTENSIONS"
    # Priority 3: Extensions marker file (created by docker-entrypoint.sh)
    elif [[ -f "/var/lib/postgresql/.extensions_to_install" ]]; then
        extensions=$(cat /var/lib/postgresql/.extensions_to_install)
    fi
    
    echo "$extensions"
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [extension1,extension2,...]"
    echo
    echo "Initialize PostgreSQL extensions from:"
    echo "  1. Command line argument (comma-separated list)"
    echo "  2. POSTGRES_EXTENSIONS environment variable"
    echo "  3. /var/lib/postgresql/.extensions_to_install file"
    echo
    echo "Available extensions:"
    echo "  pgvector, pgsodium, pgjwt, pgaudit, pg_tle, pg_stat_monitor,"
    echo "  pg_repack, pg_plan_filter, pg_net, pg_jsonschema, pg_hashids,"
    echo "  pg_cron, pg-safeupdate, index_advisor, wal2json"
    echo
    echo "Environment variables:"
    echo "  POSTGRES_HOST=${POSTGRES_HOST}"
    echo "  POSTGRES_PORT=${POSTGRES_PORT}"
    echo "  POSTGRES_USER=${POSTGRES_USER}"
    echo "  POSTGRES_DB=${POSTGRES_DB}"
    echo "  POSTGRES_PASSWORD=<hidden>"
}

main() {
    print_status "INFO" "PostgreSQL Extension Initialization"
    print_status "INFO" "==================================="
    
    # Check if help is requested
    if [[ "$1" == "-h" || "$1" == "--help" ]]; then
        show_usage
        exit 0
    fi
    
    # Get extension list
    local extensions=$(get_extension_list "$@")
    
    if [[ -z "$extensions" ]]; then
        print_status "WARN" "No extensions specified"
        show_usage
        exit 0
    fi
    
    print_status "INFO" "Extensions to install: $extensions"
    
    # Check PostgreSQL connectivity
    if ! pg_isready -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" -U "$POSTGRES_USER" >/dev/null 2>&1; then
        print_status "ERROR" "Cannot connect to PostgreSQL at $POSTGRES_HOST:$POSTGRES_PORT"
        print_status "INFO" "Make sure PostgreSQL is running and accessible"
        exit 1
    fi
    print_status "OK" "PostgreSQL connection successful"
    echo
    
    # Convert comma-separated list to array
    IFS=',' read -ra EXTENSIONS <<< "$extensions"
    
    local success_count=0
    local error_count=0
    
    for ext in "${EXTENSIONS[@]}"; do
        ext=$(echo "$ext" | xargs)  # Trim whitespace
        
        if [[ -z "$ext" ]]; then
            continue
        fi
        
        if install_extension "$ext"; then
            ((success_count++))
        else
            ((error_count++))
        fi
        echo
    done
    
    # Summary
    print_status "INFO" "Installation Summary:"
    print_status "INFO" "  Successfully installed: $success_count extensions"
    if [[ $error_count -gt 0 ]]; then
        print_status "ERROR" "  Failed to install: $error_count extensions"
    fi
    
    # Clean up marker file if it exists
    if [[ -f "/var/lib/postgresql/.extensions_to_install" ]]; then
        rm -f /var/lib/postgresql/.extensions_to_install
        print_status "INFO" "Extension marker file cleaned up"
    fi
    
    if [[ $error_count -eq 0 ]]; then
        print_status "OK" "All extensions installed successfully!"
        exit 0
    else
        print_status "ERROR" "Some extensions failed to install"
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi