-- Template for creating test user (uses text/template)
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '{{.TestUser}}') THEN
        CREATE USER {{.TestUser}} WITH PASSWORD '{{.TestPassword}}';
    END IF;
END$$;

GRANT ALL PRIVILEGES ON DATABASE {{.TestDatabase}} TO {{.TestUser}};
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO {{.TestUser}};
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO {{.TestUser}};

-- Add completion notice
DO $$
BEGIN
    RAISE NOTICE 'Test user creation completed successfully';
END $$;