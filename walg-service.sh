#!/bin/bash

# WAL-G backup service script for supervisord
set -e

# Function to log with WAL-G prefix
log() {
    echo "[wal-g] $*" >&2
}

# Check if WAL-G backup is enabled
if [ "${WALG_ENABLED:-false}" != "true" ]; then
    log "WAL-G backup is disabled, exiting..."
    exit 0
fi

log "🗃️ Starting WAL-G backup monitoring service..."

# Wait for PostgreSQL to be ready with retry logic
log "Waiting for PostgreSQL to be ready..."
MAX_RETRIES=30
RETRY_COUNT=0
until pg_isready -h localhost -p 5432 -U ${POSTGRES_USER:-postgres} > /dev/null 2>&1; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        log "ERROR: PostgreSQL did not become ready after $MAX_RETRIES attempts"
        exit 1
    fi
    log "PostgreSQL not ready (attempt $RETRY_COUNT/$MAX_RETRIES), waiting..."
    sleep 5
done
log "PostgreSQL is ready"

# Set default WAL-G environment variables
export WALG_POSTGRESQL_DATA_DIR="${PGDATA}"
export WALG_STREAM_CREATE_COMMAND="${WALG_STREAM_CREATE_COMMAND:-pg_receivewal -h localhost -p 5432 -U ${POSTGRES_USER:-postgres} -D - --synchronous}"
export WALG_STREAM_RESTORE_COMMAND="${WALG_STREAM_RESTORE_COMMAND:-pg_receivewal -h localhost -p 5432 -U ${POSTGRES_USER:-postgres} -D - --synchronous}"

# Storage configuration (required)
if [ -z "$WALG_S3_PREFIX" ] && [ -z "$WALG_GS_PREFIX" ] && [ -z "$WALG_AZ_PREFIX" ] && [ -z "$WALG_FILE_PREFIX" ]; then
    log "ERROR: WAL-G storage configuration required. Set one of: WALG_S3_PREFIX, WALG_GS_PREFIX, WALG_AZ_PREFIX, or WALG_FILE_PREFIX"
    exit 1
fi

# Backup schedule configuration
BACKUP_SCHEDULE="${WALG_BACKUP_SCHEDULE:-0 2 * * *}"  # Default: daily at 2 AM
BACKUP_RETAIN_COUNT="${WALG_BACKUP_RETAIN_COUNT:-7}"  # Keep 7 backups by default
BACKUP_COMPRESS="${WALG_BACKUP_COMPRESS:-lz4}"

echo "WAL-G configuration:"
echo "  Data directory: $WALG_POSTGRESQL_DATA_DIR"
echo "  Backup schedule: $BACKUP_SCHEDULE"
echo "  Retain backups: $BACKUP_RETAIN_COUNT"
echo "  Compression: $BACKUP_COMPRESS"

# Function to perform backup
perform_backup() {
    echo "$(date): Starting WAL-G backup..."
    
    # Create base backup
    if wal-g backup-push "$WALG_POSTGRESQL_DATA_DIR"; then
        echo "$(date): Backup completed successfully"
        
        # Clean up old backups
        if [ "$BACKUP_RETAIN_COUNT" -gt 0 ]; then
            echo "$(date): Cleaning up old backups (keeping $BACKUP_RETAIN_COUNT)"
            wal-g delete retain "$BACKUP_RETAIN_COUNT" --confirm
        fi
    else
        echo "$(date): ERROR: Backup failed"
        return 1
    fi
}

# Function to perform WAL archiving test
test_wal_archiving() {
    echo "$(date): Testing WAL archiving..."
    
    # Force a checkpoint and switch WAL file to test archiving
    PGPASSWORD="${POSTGRES_PASSWORD:-}" psql -h localhost -p 5432 -U ${POSTGRES_USER:-postgres} -d ${POSTGRES_DB:-postgres} -c "SELECT pg_switch_wal();" || true
    
    echo "$(date): WAL archiving test completed"
}

# Perform initial backup if requested
if [ "${WALG_INITIAL_BACKUP:-false}" = "true" ]; then
    echo "Performing initial backup..."
    sleep 30  # Give PostgreSQL time to fully start
    perform_backup
fi

# Test WAL archiving setup
test_wal_archiving

# Main monitoring loop
echo "Starting backup scheduler..."
while true; do
    current_time=$(date +"%M %H %d %m %w")  # minute hour day month weekday
    
    # Simple cron-like scheduler
    if echo "$BACKUP_SCHEDULE" | grep -q "$(echo $current_time | cut -d' ' -f1) $(echo $current_time | cut -d' ' -f2) $(echo $current_time | cut -d' ' -f3) $(echo $current_time | cut -d' ' -f4) $(echo $current_time | cut -d' ' -f5)"; then
        perform_backup
    fi
    
    # Check every minute
    sleep 60
done