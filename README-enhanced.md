# Enhanced PostgreSQL Distribution

A production-ready PostgreSQL distribution inspired by Supabase, featuring PostgreSQL 17 with popular extensions, connection pooling via PgBouncer, automatic REST API generation with PostgREST, and backup capabilities with WAL-G. All services are managed by s6-overlay for robust process supervision.

## 🚀 Features

### Core Database
- **PostgreSQL 17** - Latest PostgreSQL with performance improvements
- **Popular Extensions** - Pre-installed extensions for AI/ML, JSON, scheduling, and more
- **Automatic Upgrades** - Seamless upgrades from PostgreSQL 14/15/16 to 17

### Connection Pooling
- **PgBouncer** - High-performance connection pooling
- **Transaction Pooling** - Optimized for web applications
- **Authentication Integration** - Seamless auth with PostgreSQL

### REST API
- **PostgREST** - Automatic REST API generation from database schema
- **JWT Authentication** - Secure API access with JSON Web Tokens
- **Role-based Access** - Fine-grained permissions via PostgreSQL roles

### Backup & Recovery
- **WAL-G** - Point-in-time recovery and automated backups
- **Multi-cloud Support** - S3, GCS, and Azure Blob Storage
- **Automated Scheduling** - CronJob-based backup scheduling

### Process Management
- **s6-overlay** - Lightweight process supervision
- **Service Dependencies** - Proper startup ordering and health monitoring
- **Graceful Shutdowns** - Clean service termination

## 📦 Available Images

### Enhanced Images
- `ghcr.io/flanksource/postgres-enhanced:17-latest` - Full-featured image
- `ghcr.io/flanksource/postgres-enhanced:17.6` - Specific version
- `ghcr.io/flanksource/postgres-enhanced:17.6-abc123f` - Version with commit SHA

### Standard Upgrade Images (Legacy)
- `ghcr.io/flanksource/postgres:17` - Standard PostgreSQL 17
- `ghcr.io/flanksource/postgres-upgrade:to-17` - Upgrade-focused image

## 🔧 Quick Start

### Docker Compose

```yaml
version: '3.8'
services:
  postgres-enhanced:
    image: ghcr.io/flanksource/postgres-enhanced:17-latest
    environment:
      POSTGRES_PASSWORD: your-secure-password
      PGBOUNCER_ENABLED: "true"
      POSTGREST_ENABLED: "true"
      POSTGREST_JWT_SECRET: "your-jwt-secret"
    ports:
      - "5432:5432"  # PostgreSQL
      - "6432:6432"  # PgBouncer
      - "3000:3000"  # PostgREST
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["/usr/local/bin/healthcheck.sh"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  postgres_data:
```

### Kubernetes with Helm

```bash
# Install with enhanced features enabled
helm install my-postgres ./chart \
  --values ./chart/values-enhanced.yaml \
  --set pgbouncer.enabled=true \
  --set postgrest.enabled=true \
  --set postgresql.database.password="your-secure-password"

# Access services
kubectl port-forward svc/my-postgres-postgresql 5432:5432     # PostgreSQL
kubectl port-forward svc/my-postgres-pgbouncer 6432:6432      # PgBouncer
kubectl port-forward svc/my-postgres-postgrest 3000:3000      # PostgREST API
```

## 🧩 Extensions Included

### AI/ML & Vector Search
- **pgvector** - Vector similarity search for AI/ML applications
- **pg_embedding** - Additional embedding functions

### Job Scheduling & Background Tasks
- **pg_cron** - Cron-based job scheduler within PostgreSQL
- **pg_partman** - Automated table partitioning

### Security & Authentication
- **pgjwt** - JSON Web Token functions
- **pgcrypto** - Cryptographic functions
- **pgaudit** - Session and object-level audit logging

### HTTP & Network
- **pgsql-http** - HTTP client for PostgreSQL
- **pg_net** - Async HTTP/webhook requests

### Development & Utilities
- **pg_hashids** - Generate short unique IDs like YouTube
- **pg_jsonschema** - JSON schema validation
- **hypopg** - Hypothetical indexes for query planning
- **pg_stat_monitor** - Enhanced query performance monitoring

### Maintenance & Performance
- **pg_repack** - Online table reorganization without locks
- **pg_plan_filter** - Filter and log query plans

### Replication & Logical Decoding
- **wal2json** - JSON output for logical replication
- **pg_tle** - Trusted Language Extensions framework

## 🏗️ Architecture

```
┌─────────────────────────────────────────┐
│                s6-overlay               │
│            Process Supervisor           │
├─────────────────────────────────────────┤
│  ┌─────────────┐ ┌─────────────┐       │
│  │ PostgreSQL  │ │  PgBouncer  │       │
│  │    :5432    │ │    :6432    │       │
│  └─────────────┘ └─────────────┘       │
│                                         │
│  ┌─────────────┐ ┌─────────────┐       │
│  │  PostgREST  │ │   WAL-G     │       │
│  │    :3000    │ │  (backups)  │       │
│  └─────────────┘ └─────────────┘       │
└─────────────────────────────────────────┘
```

## ⚙️ Configuration

### Environment Variables

#### PostgreSQL Core
```bash
POSTGRES_DB=postgres                    # Database name
POSTGRES_USER=postgres                  # Username
POSTGRES_PASSWORD=secure-password       # Password (required)
```

#### Service Control
```bash
PGBOUNCER_ENABLED=true                  # Enable PgBouncer
POSTGREST_ENABLED=true                  # Enable PostgREST
WALG_ENABLED=false                      # Enable WAL-G backups
```

#### PgBouncer Configuration
```bash
PGBOUNCER_PORT=6432                     # PgBouncer port
PGBOUNCER_MAX_CLIENT_CONN=100           # Max client connections
PGBOUNCER_DEFAULT_POOL_SIZE=25          # Default pool size
PGBOUNCER_POOL_MODE=transaction         # Pooling mode
```

#### PostgREST Configuration
```bash
POSTGREST_PORT=3000                     # PostgREST port
POSTGREST_DB_SCHEMA=public              # Schema to expose
POSTGREST_DB_ANON_ROLE=anon            # Anonymous role
POSTGREST_JWT_SECRET=your-jwt-secret    # JWT secret key
POSTGREST_MAX_ROWS=1000                 # Max rows per request
```

#### WAL-G Backup Configuration
```bash
WALG_COMPRESSION_METHOD=lz4             # Compression: lz4, lzma, brotli
WALG_S3_PREFIX=s3://bucket/path         # S3 storage path
AWS_ACCESS_KEY_ID=your-access-key       # AWS credentials
AWS_SECRET_ACCESS_KEY=your-secret-key   # AWS credentials
```

## 🔌 API Usage

### Health Check
```bash
# Via PostgreSQL function
psql -c "SELECT health_check();"

# Via PostgREST API
curl http://localhost:3000/rpc/health_check
```

### REST API Examples
```bash
# List all tables
curl http://localhost:3000/

# Query metrics table
curl http://localhost:3000/metrics

# Insert data (requires authentication)
curl -X POST http://localhost:3000/metrics \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-jwt-token" \
  -d '{"name": "cpu_usage", "value": 85.5}'

# Vector similarity search (with pgvector)
curl -X POST http://localhost:3000/rpc/vector_search \
  -H "Content-Type: application/json" \
  -d '{"query_vector": "[1,2,3]", "limit": 5}'
```

### Connection Examples
```bash
# Direct PostgreSQL connection
psql postgresql://postgres:password@localhost:5432/postgres

# Via PgBouncer (recommended for applications)
psql postgresql://postgres:password@localhost:6432/postgres

# Check PgBouncer stats
psql postgresql://postgres:password@localhost:6432/pgbouncer -c "SHOW POOLS;"
```

## 📊 Monitoring & Observability

### Built-in Health Checks
- PostgreSQL readiness probe
- Service-specific health endpoints
- Multi-service health check script

### Metrics & Monitoring
```sql
-- Query performance metrics
SELECT * FROM pg_stat_statements ORDER BY total_time DESC LIMIT 10;

-- Check extension usage
SELECT * FROM pg_extension;

-- Monitor connections via PgBouncer
SHOW POOLS;
SHOW CLIENTS;
```

### Log Analysis
```bash
# View service logs
docker logs container-name

# Kubernetes logs
kubectl logs -f statefulset/postgres-enhanced

# Service-specific logs in s6-overlay
docker exec container-name s6-rc -v2 -t 1000 -u change
```

## 🛠️ Development

### Building the Enhanced Image
```bash
# Build image
make -f Makefile.enhanced build

# Build and test locally
make -f Makefile.enhanced dev

# Run complete CI pipeline
make -f Makefile.enhanced ci
```

### Testing
```bash
# Local container testing
make -f Makefile.enhanced test-local

# Kubernetes integration tests
make -f Makefile.enhanced test-k8s

# Full Helm test cycle
make -f Makefile.enhanced test-helm
```

### Development Workflow
```bash
# Start local development environment
make -f Makefile.enhanced run

# Check container health
make -f Makefile.enhanced health

# View logs
make -f Makefile.enhanced logs

# Open shell for debugging
make -f Makefile.enhanced shell

# Clean up
make -f Makefile.enhanced clean
```

## 🔄 Migration & Upgrades

### From Standard PostgreSQL
1. **Backup your data** using pg_dump or WAL-G
2. Update your configuration to use the enhanced image
3. Enable desired services (PgBouncer, PostgREST)
4. Test thoroughly before production deployment

### From Previous Versions
The enhanced image includes automatic upgrade capabilities:

```bash
# Upgrade will be performed automatically on container start
docker run -v your-data:/var/lib/postgresql/data \
  ghcr.io/flanksource/postgres-enhanced:17-latest
```

## 🚨 Production Considerations

### Security
- Use strong passwords and JWT secrets
- Configure proper network policies
- Regular security updates
- Limit PostgREST schema exposure
- Use SSL/TLS for all connections

### Performance
- Configure connection pooling based on your workload
- Monitor and tune PostgreSQL settings
- Use appropriate extensions for your use case
- Regular VACUUM and ANALYZE operations

### Backup Strategy
```yaml
# Example backup configuration
backup:
  enabled: true
  schedule: "0 2 * * *"  # Daily at 2 AM
  walg:
    retainCount: 7       # Keep 7 backups
    storage:
      s3:
        enabled: true
        prefix: "s3://backups/postgres"
        region: "us-west-2"
```

### High Availability
- Use PostgreSQL streaming replication
- Configure proper resource limits
- Implement pod disruption budgets
- Use persistent volumes with backup

## 📈 Scaling

### Horizontal Scaling (Read Replicas)
```yaml
# Configure read replicas
postgresql:
  replicas:
    enabled: true
    count: 2
    readOnly: true
```

### Connection Pooling
```yaml
# Optimize PgBouncer for your workload
pgbouncer:
  config:
    maxClientConn: 200
    defaultPoolSize: 50
    poolMode: "transaction"
```

## 🐛 Troubleshooting

### Common Issues

**Service Won't Start**
```bash
# Check s6-overlay logs
docker exec container s6-rc -v2 -da change

# Check individual service status
docker exec container s6-svstat /run/service/postgresql
docker exec container s6-svstat /run/service/pgbouncer
docker exec container s6-svstat /run/service/postgrest
```

**Connection Issues**
```bash
# Test PostgreSQL directly
pg_isready -h localhost -p 5432

# Test PgBouncer
nc -z localhost 6432

# Test PostgREST
curl -f http://localhost:3000/
```

**Extension Problems**
```sql
-- Check installed extensions
SELECT * FROM pg_extension;

-- Check available extensions
SELECT * FROM pg_available_extensions WHERE name LIKE 'pg%';

-- Install missing extension
CREATE EXTENSION IF NOT EXISTS extension_name;
```

### Performance Issues
```sql
-- Check slow queries
SELECT query, total_time, calls, mean_time 
FROM pg_stat_statements 
ORDER BY total_time DESC LIMIT 10;

-- Check connection pooling stats
-- (Connect to PgBouncer admin)
SHOW POOLS;
SHOW STATS;
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Submit a pull request

### Development Setup
```bash
git clone https://github.com/flanksource/postgres
cd postgres
make -f Makefile.enhanced dev
```

## 📋 Roadmap

- [ ] Additional PostgreSQL extensions
- [ ] Grafana dashboard templates
- [ ] Multi-database support
- [ ] Enhanced security features
- [ ] Performance optimization guides
- [ ] Cloud-specific optimizations

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- GitHub Issues: [Report bugs and request features](https://github.com/flanksource/postgres/issues)
- Documentation: Check this README and inline comments
- Community: Join discussions in GitHub Discussions

---

## 📚 Additional Resources

- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [PgBouncer Documentation](https://www.pgbouncer.org/)
- [PostgREST Documentation](https://postgrest.org/)
- [WAL-G Documentation](https://wal-g.readthedocs.io/)
- [s6-overlay Documentation](https://github.com/just-containers/s6-overlay)

**Built with ❤️ by the Flanksource team, inspired by Supabase's excellent PostgreSQL distribution.**