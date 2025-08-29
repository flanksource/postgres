-- PostgreSQL Task Logging Infrastructure
-- This serves as both the schema and initial seed data

CREATE TABLE IF NOT EXISTS postgres_task_log (
    id SERIAL PRIMARY KEY,
    task_name VARCHAR(100) NOT NULL,
    task_type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    duration_ms INTEGER,
    details JSONB,
    error_message TEXT
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_task_log_started ON postgres_task_log(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_log_type ON postgres_task_log(task_type);
CREATE INDEX IF NOT EXISTS idx_task_log_status ON postgres_task_log(status);
CREATE INDEX IF NOT EXISTS idx_task_log_name_status ON postgres_task_log(task_name, status);

-- Initial seed entry
INSERT INTO postgres_task_log (task_name, task_type, status, completed_at, details)
VALUES ('init-task-log', 'seed', 'completed', CURRENT_TIMESTAMP, '{"description": "Task logging infrastructure initialized"}')
ON CONFLICT DO NOTHING;

-- Create test data table (part of seed)
CREATE TABLE IF NOT EXISTS test_upgrade (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    value INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert test data
INSERT INTO test_upgrade (name, value) 
SELECT 'record-' || i, i * 100
FROM generate_series(1, 5) i
ON CONFLICT DO NOTHING;

-- Create indexes on test table
CREATE INDEX IF NOT EXISTS idx_test_upgrade_name ON test_upgrade(name);
CREATE INDEX IF NOT EXISTS idx_test_upgrade_value ON test_upgrade(value);

-- Create a view for test data
CREATE VIEW IF NOT EXISTS test_upgrade_summary AS
SELECT COUNT(*) as total_records, SUM(value) as total_value
FROM test_upgrade;

-- Log the seed operation
INSERT INTO postgres_task_log (task_name, task_type, status, completed_at, details)
VALUES ('seed-test-data', 'seed', 'completed', CURRENT_TIMESTAMP, '{"table": "test_upgrade", "records": 5}')
ON CONFLICT DO NOTHING;