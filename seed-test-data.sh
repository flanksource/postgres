#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Seeding PostgreSQL test volumes with data..."

# Function to seed a PostgreSQL volume with test data
seed_postgres_volume() {
    local version=$1
    local volume_name=$2
    local container_name="postgres-seed-${version}"
    
    echo -e "${YELLOW}Seeding volume ${volume_name} with PostgreSQL ${version} data...${NC}"
    
    # Remove any existing container
    docker rm -f ${container_name} 2>/dev/null || true
    
    # Create and start PostgreSQL container with the volume
    docker run -d \
        --name ${container_name} \
        -e POSTGRES_PASSWORD=testpass \
        -e POSTGRES_DB=testdb \
        -v ${volume_name}:/var/lib/postgresql/data \
        postgres:${version}
    
    # Wait for PostgreSQL to be ready
    echo -n "Waiting for PostgreSQL ${version} to be ready..."
    for i in {1..30}; do
        if docker exec ${container_name} pg_isready -U postgres >/dev/null 2>&1; then
            echo -e " ${GREEN}Ready!${NC}"
            break
        fi
        echo -n "."
        sleep 1
    done
    
    # Create test data
    echo "Creating test data..."
    docker exec ${container_name} psql -U postgres -d testdb -c "
        CREATE USER testuser WITH PASSWORD 'testpass';
        GRANT ALL PRIVILEGES ON DATABASE testdb TO testuser;
    "
    docker exec ${container_name} psql -U postgres -d testdb -c "
        CREATE TABLE test_table (
            id SERIAL PRIMARY KEY,
            name VARCHAR(100),
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            version VARCHAR(10) DEFAULT '${version}'
        );
        
        INSERT INTO test_table (name) VALUES 
            ('Test Record 1 from PG${version}'),
            ('Test Record 2 from PG${version}'),
            ('Test Record 3 from PG${version}');
        
        CREATE TABLE version_info (
            id INTEGER PRIMARY KEY,
            original_version VARCHAR(10),
            seed_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
        
        INSERT INTO version_info (id, original_version) VALUES (1, '${version}');
    "
    
    # Verify data was created
    echo "Verifying test data..."
    docker exec ${container_name} psql -U postgres -d testdb -c "
        SELECT 'Tables created:' as info;
        SELECT tablename FROM pg_tables WHERE schemaname = 'public';
        SELECT 'Test data count:' as info;
        SELECT COUNT(*) as record_count FROM test_table;
        SELECT 'Version info:' as info;
        SELECT * FROM version_info;
    "
    
    # Stop and remove the container (data persists in volume)
    echo "Stopping seed container..."
    docker stop ${container_name}
    docker rm ${container_name}
    
    echo -e "${GREEN}Volume ${volume_name} seeded successfully with PostgreSQL ${version} data${NC}\n"
}

# Create volumes if they don't exist
echo "Creating Docker volumes..."
docker volume create pg14-test-data
docker volume create pg15-test-data
docker volume create pg16-test-data

# Seed each volume with the appropriate PostgreSQL version
seed_postgres_volume "14" "pg14-test-data"
seed_postgres_volume "15" "pg15-test-data"
seed_postgres_volume "16" "pg16-test-data"

echo -e "${GREEN}All test volumes seeded successfully!${NC}"