#!/bin/bash

# PgBouncer Load Test Script
# Tests connection pooling under various load scenarios

set -e

POSTGRES_HOST=${POSTGRES_HOST:-postgres-test}
POSTGRES_PORT=${POSTGRES_PORT:-5432}
PGBOUNCER_PORT=${PGBOUNCER_PORT:-6432}
POSTGRES_USER=${POSTGRES_USER:-testuser}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-testpass}
POSTGRES_DB=${POSTGRES_DB:-testdb}

echo "🏊 Starting PgBouncer Load Tests"
echo "================================="

# Function to run concurrent connections
run_concurrent_test() {
    local host=$1
    local port=$2
    local connections=$3
    local queries_per_conn=$4
    local test_name=$5
    
    echo "Running $test_name: $connections concurrent connections, $queries_per_conn queries each"
    
    local start_time=$(date +%s)
    local pids=()
    
    for i in $(seq 1 $connections); do
        (
            PGPASSWORD=$POSTGRES_PASSWORD psql -h $host -p $port -U $POSTGRES_USER -d $POSTGRES_DB -q << EOF >/dev/null 2>&1
$(for j in $(seq 1 $queries_per_conn); do
    echo "SELECT pg_sleep(0.01), $i as connection_id, $j as query_id, now() as timestamp;"
done)
EOF
        ) &
        pids+=($!)
    done
    
    # Wait for all background processes
    for pid in "${pids[@]}"; do
        wait $pid
    done
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    local total_queries=$((connections * queries_per_conn))
    local qps=$((total_queries / duration))
    
    echo "  ✅ $test_name completed in ${duration}s (${qps} queries/sec)"
}

# Test 1: Direct PostgreSQL connection baseline
echo "Test 1: Direct PostgreSQL Connection Baseline"
run_concurrent_test $POSTGRES_HOST $POSTGRES_PORT 10 10 "Direct PostgreSQL (10 conn)"

# Test 2: PgBouncer connection pooling
echo "Test 2: PgBouncer Connection Pooling"
run_concurrent_test $POSTGRES_HOST $PGBOUNCER_PORT 10 10 "PgBouncer (10 conn)"

# Test 3: High concurrency test via PgBouncer
echo "Test 3: High Concurrency via PgBouncer"
run_concurrent_test $POSTGRES_HOST $PGBOUNCER_PORT 50 5 "PgBouncer High Concurrency (50 conn)"

# Test 4: Sustained load test
echo "Test 4: Sustained Load Test"
run_concurrent_test $POSTGRES_HOST $PGBOUNCER_PORT 20 20 "PgBouncer Sustained Load (20x20)"

# Test 5: Connection churn test (rapid connect/disconnect)
echo "Test 5: Connection Churn Test"
local start_time=$(date +%s)
for i in $(seq 1 100); do
    PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $PGBOUNCER_PORT -U $POSTGRES_USER -d $POSTGRES_DB -c "SELECT 1;" >/dev/null 2>&1
done
local end_time=$(date +%s)
local duration=$((end_time - start_time))
echo "  ✅ Connection churn test: 100 connections in ${duration}s"

# Check PgBouncer statistics
echo ""
echo "PgBouncer Statistics:"
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $PGBOUNCER_PORT -U $POSTGRES_USER -d pgbouncer -c "SHOW POOLS;" 2>/dev/null || echo "Could not retrieve PgBouncer stats"
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $PGBOUNCER_PORT -U $POSTGRES_USER -d pgbouncer -c "SHOW STATS;" 2>/dev/null || echo "Could not retrieve PgBouncer stats"

echo ""
echo "✅ PgBouncer load tests completed successfully!"
echo ""
echo "Interpretation:"
echo "- PgBouncer should handle high concurrency better than direct connections"
echo "- Connection churn should be fast due to connection reuse"
echo "- Pool statistics should show active connection management"