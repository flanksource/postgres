-- Test seed data for PostgreSQL upgrade testing
CREATE TABLE IF NOT EXISTS test_upgrade (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    value INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO test_upgrade (name, value) VALUES
    ('record1', 100),
    ('record2', 200),
    ('record3', 300),
    ('record4', 400),
    ('record5', 500)
ON CONFLICT DO NOTHING;

-- Create some additional test objects
CREATE INDEX IF NOT EXISTS idx_test_upgrade_name ON test_upgrade(name);
CREATE INDEX IF NOT EXISTS idx_test_upgrade_value ON test_upgrade(value);

-- Create a view (using DROP/CREATE pattern for compatibility)
DROP VIEW IF EXISTS test_upgrade_summary;
CREATE VIEW test_upgrade_summary AS
SELECT COUNT(*) as total_records, SUM(value) as total_value
FROM test_upgrade;

-- Create a test user if not exists (will be replaced with actual values)
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'testuser') THEN
        CREATE USER testuser WITH PASSWORD 'testpass';
    END IF;
END$$;

GRANT ALL PRIVILEGES ON DATABASE postgres TO testuser;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO testuser;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO testuser;