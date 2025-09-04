#!/bin/bash

# Enhanced Service Logging Library
# Provides structured logging, health reporting, and metrics collection for PostgreSQL services
# Usage: source /usr/local/bin/service-logging.sh

# Default configuration
SERVICE_LOG_FORMAT="${SERVICE_LOG_FORMAT:-structured}"  # "structured" or "simple"
SERVICE_LOG_LEVEL="${SERVICE_LOG_LEVEL:-INFO}"          # DEBUG, INFO, WARN, ERROR
SERVICE_LOG_TIMESTAMPS="${SERVICE_LOG_TIMESTAMPS:-true}"
SERVICE_METRICS_ENABLED="${SERVICE_METRICS_ENABLED:-true}"

# Service context (should be set by the calling service)
SERVICE_NAME="${SERVICE_NAME:-unknown}"
SERVICE_VERSION="${SERVICE_VERSION:-unknown}"
SERVICE_PID="$$"

# Log level numeric values for comparison
declare -A LOG_LEVELS=([DEBUG]=0 [INFO]=1 [WARN]=2 [ERROR]=3)
CURRENT_LOG_LEVEL=${LOG_LEVELS[${SERVICE_LOG_LEVEL}]:-1}

# Metrics storage
declare -A SERVICE_METRICS=()
declare -A SERVICE_COUNTERS=()

# Health status tracking
SERVICE_HEALTH_STATUS="STARTING"
SERVICE_HEALTH_DETAILS=""
declare -A SERVICE_HEALTH_CHECKS=()

# Colors for simple format
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to get current timestamp
get_timestamp() {
    if [[ "${SERVICE_LOG_TIMESTAMPS}" == "true" ]]; then
        date -u +"%Y-%m-%dT%H:%M:%S.%3NZ"
    else
        echo ""
    fi
}

# Function to escape JSON strings
json_escape() {
    printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g; s/\n/\\n/g; s/\r/\\r/g'
}

# Core logging function
log_message() {
    local level="$1"
    local message="$2"
    local extra_fields="$3"
    
    # Check if level should be logged
    local level_num=${LOG_LEVELS[$level]:-1}
    if [[ $level_num -lt $CURRENT_LOG_LEVEL ]]; then
        return 0
    fi
    
    local timestamp=$(get_timestamp)
    
    if [[ "${SERVICE_LOG_FORMAT}" == "structured" ]]; then
        # JSON structured logging
        local json_msg="{\"timestamp\":\"$timestamp\""
        json_msg+=",\"level\":\"$level\""
        json_msg+=",\"service\":\"$SERVICE_NAME\""
        json_msg+=",\"version\":\"$SERVICE_VERSION\""
        json_msg+=",\"pid\":$SERVICE_PID"
        json_msg+=",\"message\":\"$(json_escape "$message")\""
        
        # Add health status
        json_msg+=",\"health_status\":\"$SERVICE_HEALTH_STATUS\""
        
        # Add extra fields if provided
        if [[ -n "$extra_fields" ]]; then
            json_msg+=",${extra_fields}"
        fi
        
        json_msg+="}"
        echo "$json_msg" >&2
    else
        # Simple format with colors
        local color=""
        local prefix=""
        case $level in
            "DEBUG") color="$BLUE"; prefix="🔍" ;;
            "INFO")  color="$GREEN"; prefix="ℹ️" ;;
            "WARN")  color="$YELLOW"; prefix="⚠️" ;;
            "ERROR") color="$RED"; prefix="❌" ;;
        esac
        
        local log_line=""
        if [[ -n "$timestamp" ]]; then
            log_line="$timestamp "
        fi
        log_line+="[$SERVICE_NAME] ${color}${prefix} ${level}${NC}: $message"
        
        echo -e "$log_line" >&2
    fi
}

# Convenience logging functions
log_debug() {
    log_message "DEBUG" "$1" "$2"
}

log_info() {
    log_message "INFO" "$1" "$2"
}

log_warn() {
    log_message "WARN" "$1" "$2"
}

log_error() {
    log_message "ERROR" "$1" "$2"
}

# Progress tracking for startup sequences
log_progress() {
    local step="$1"
    local total="$2"
    local description="$3"
    
    local progress_pct=$((step * 100 / total))
    local extra_fields="\"step\":$step,\"total_steps\":$total,\"progress_percent\":$progress_pct"
    
    log_info "Startup progress: $description ($step/$total - ${progress_pct}%)" "$extra_fields"
}

# Health status management
set_health_status() {
    local status="$1"
    local details="$2"
    
    SERVICE_HEALTH_STATUS="$status"
    SERVICE_HEALTH_DETAILS="$details"
    
    local extra_fields="\"health_details\":\"$(json_escape "$details")\""
    log_info "Health status changed to: $status" "$extra_fields"
}

# Health check registration and execution
register_health_check() {
    local name="$1"
    local command="$2"
    local timeout="${3:-10}"
    
    SERVICE_HEALTH_CHECKS["$name"]="$command|$timeout"
}

run_health_checks() {
    local overall_status="HEALTHY"
    local failed_checks=()
    
    for check_name in "${!SERVICE_HEALTH_CHECKS[@]}"; do
        local check_data="${SERVICE_HEALTH_CHECKS[$check_name]}"
        local command=$(echo "$check_data" | cut -d'|' -f1)
        local timeout=$(echo "$check_data" | cut -d'|' -f2)
        
        if timeout "$timeout" bash -c "$command" >/dev/null 2>&1; then
            log_debug "Health check '$check_name' passed"
        else
            log_warn "Health check '$check_name' failed"
            overall_status="UNHEALTHY"
            failed_checks+=("$check_name")
        fi
    done
    
    if [[ "$overall_status" == "HEALTHY" ]]; then
        set_health_status "HEALTHY" "All health checks passed"
        return 0
    else
        local failed_list=$(IFS=,; echo "${failed_checks[*]}")
        set_health_status "UNHEALTHY" "Failed checks: $failed_list"
        return 1
    fi
}

# Metrics collection
set_metric() {
    local name="$1"
    local value="$2"
    local labels="$3"
    
    local key="$name"
    if [[ -n "$labels" ]]; then
        key="${name}{${labels}}"
    fi
    
    SERVICE_METRICS["$key"]="$value"
    
    if [[ "${SERVICE_LOG_LEVEL}" == "DEBUG" ]]; then
        log_debug "Metric set: $key = $value"
    fi
}

increment_counter() {
    local name="$1"
    local labels="$2"
    local increment="${3:-1}"
    
    local key="$name"
    if [[ -n "$labels" ]]; then
        key="${name}{${labels}}"
    fi
    
    local current_value=${SERVICE_COUNTERS["$key"]:-0}
    SERVICE_COUNTERS["$key"]=$((current_value + increment))
    
    if [[ "${SERVICE_LOG_LEVEL}" == "DEBUG" ]]; then
        log_debug "Counter incremented: $key = ${SERVICE_COUNTERS[$key]}"
    fi
}

# Configuration validation logging
log_config() {
    local config_name="$1"
    local config_value="$2"
    local is_sensitive="${3:-false}"
    
    if [[ "$is_sensitive" == "true" ]]; then
        config_value="[REDACTED]"
    fi
    
    local extra_fields="\"config_name\":\"$config_name\",\"config_value\":\"$(json_escape "$config_value")\""
    log_debug "Configuration: $config_name = $config_value" "$extra_fields"
}

# Service lifecycle logging
log_startup() {
    local extra_fields="\"lifecycle\":\"startup\""
    log_info "Service starting up" "$extra_fields"
    set_health_status "STARTING" "Service initialization in progress"
}

log_ready() {
    local extra_fields="\"lifecycle\":\"ready\""
    log_info "Service is ready to accept connections" "$extra_fields"
    set_health_status "HEALTHY" "Service is fully operational"
}

log_shutdown() {
    local reason="${1:-shutdown_requested}"
    local extra_fields="\"lifecycle\":\"shutdown\",\"reason\":\"$reason\""
    log_info "Service shutting down: $reason" "$extra_fields"
    set_health_status "STOPPING" "Service shutdown in progress"
}

# Error context logging
log_error_with_context() {
    local error_message="$1"
    local error_code="${2:-unknown}"
    local context="$3"
    
    local extra_fields="\"error_code\":\"$error_code\""
    if [[ -n "$context" ]]; then
        extra_fields+=",\"error_context\":\"$(json_escape "$context")\""
    fi
    
    log_error "$error_message" "$extra_fields"
    increment_counter "service_errors" "error_code=$error_code"
}

# Connection tracking
log_connection_event() {
    local event_type="$1"  # connect, disconnect, auth_success, auth_failure
    local client_info="$2"
    local database="${3:-}"
    
    local extra_fields="\"event_type\":\"$event_type\",\"client\":\"$(json_escape "$client_info")\""
    if [[ -n "$database" ]]; then
        extra_fields+=",\"database\":\"$database\""
    fi
    
    case "$event_type" in
        "connect")
            log_info "Client connected: $client_info" "$extra_fields"
            increment_counter "client_connections" "type=connect"
            ;;
        "disconnect")
            log_info "Client disconnected: $client_info" "$extra_fields"
            increment_counter "client_connections" "type=disconnect"
            ;;
        "auth_success")
            log_info "Authentication successful: $client_info" "$extra_fields"
            increment_counter "auth_events" "type=success"
            ;;
        "auth_failure")
            log_warn "Authentication failed: $client_info" "$extra_fields"
            increment_counter "auth_events" "type=failure"
            ;;
    esac
}

# Performance logging
log_performance() {
    local operation="$1"
    local duration_ms="$2"
    local additional_info="$3"
    
    local extra_fields="\"operation\":\"$operation\",\"duration_ms\":$duration_ms"
    if [[ -n "$additional_info" ]]; then
        extra_fields+=",\"details\":\"$(json_escape "$additional_info")\""
    fi
    extra_fields+="\""
    
    # Log as INFO for significant operations, DEBUG for routine operations
    if [[ $duration_ms -gt 1000 ]]; then
        log_warn "Slow operation: $operation took ${duration_ms}ms" "$extra_fields"
    elif [[ $duration_ms -gt 100 ]]; then
        log_info "Operation completed: $operation took ${duration_ms}ms" "$extra_fields"
    else
        log_debug "Operation completed: $operation took ${duration_ms}ms" "$extra_fields"
    fi
    
    set_metric "operation_duration_ms" "$duration_ms" "operation=$operation"
}

# Function to dump all metrics in Prometheus format
dump_metrics() {
    echo "# Service metrics for $SERVICE_NAME"
    echo "# TYPE service_info gauge"
    echo "service_info{service=\"$SERVICE_NAME\",version=\"$SERVICE_VERSION\",status=\"$SERVICE_HEALTH_STATUS\"} 1"
    
    # Dump regular metrics
    for key in "${!SERVICE_METRICS[@]}"; do
        echo "# TYPE ${key%{*} gauge"
        echo "$key ${SERVICE_METRICS[$key]}"
    done
    
    # Dump counters
    for key in "${!SERVICE_COUNTERS[@]}"; do
        echo "# TYPE ${key%{*} counter"
        echo "$key ${SERVICE_COUNTERS[$key]}"
    done
}

# Function to create a simple health check endpoint response
get_health_status() {
    if [[ "${SERVICE_LOG_FORMAT}" == "structured" ]]; then
        local json_response="{\"status\":\"$SERVICE_HEALTH_STATUS\""
        json_response+=",\"service\":\"$SERVICE_NAME\""
        json_response+=",\"version\":\"$SERVICE_VERSION\""
        json_response+=",\"pid\":$SERVICE_PID"
        if [[ -n "$SERVICE_HEALTH_DETAILS" ]]; then
            json_response+=",\"details\":\"$(json_escape "$SERVICE_HEALTH_DETAILS")\""
        fi
        json_response+="}"
        echo "$json_response"
    else
        echo "Service: $SERVICE_NAME"
        echo "Status: $SERVICE_HEALTH_STATUS"
        echo "Version: $SERVICE_VERSION"
        echo "PID: $SERVICE_PID"
        if [[ -n "$SERVICE_HEALTH_DETAILS" ]]; then
            echo "Details: $SERVICE_HEALTH_DETAILS"
        fi
    fi
}

# Trap handler for graceful shutdown logging
handle_shutdown() {
    local signal="$1"
    log_shutdown "received_signal_$signal"
    exit 0
}

# Set up signal handlers
trap 'handle_shutdown TERM' TERM
trap 'handle_shutdown INT' INT
trap 'handle_shutdown QUIT' QUIT

# Function to initialize logging for a service
init_service_logging() {
    local service_name="$1"
    local service_version="${2:-1.0}"
    
    SERVICE_NAME="$service_name"
    SERVICE_VERSION="$service_version"
    
    log_startup
}