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
    ('record5', 500);

-- Create some additional test objects
CREATE INDEX IF NOT EXISTS idx_test_upgrade_name ON test_upgrade(name);
CREATE INDEX IF NOT EXISTS idx_test_upgrade_value ON test_upgrade(value);

-- Create a view (using DROP/CREATE pattern for compatibility)  
DROP VIEW IF EXISTS test_upgrade_summary;
CREATE VIEW test_upgrade_summary AS
SELECT COUNT(*) as total_records, SUM(value) as total_value
FROM test_upgrade;

-- Add completion notice
DO $$
BEGIN
    RAISE NOTICE 'Test seed data script completed successfully';
END $$;