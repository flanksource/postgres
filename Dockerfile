# PostgreSQL Docker Image with Extensions and Services
# PostgreSQL via apt + Extensions + Services via eget
# Supports both AMD64 and ARM64 architectures

FROM postgres:17-bookworm

# Get architecture information
ARG TARGETARCH
ARG TARGETOS
ENV TARGETARCH=${TARGETARCH}
ENV TARGETOS=${TARGETOS}

# Target version for upgrades (can be 15, 16, or 17)
ARG TARGET_VERSION=17
ENV TARGET_VERSION=${TARGET_VERSION}

# Task version for consistent builds
ARG TASK_VERSION=3.44.1

# Labels
LABEL maintainer="flanksource"
LABEL description="PostgreSQL with Nix-based extensions, PgBouncer, PostgREST, WAL-G, and s6-overlay"
LABEL architecture="multi-arch"
LABEL target_version="${TARGET_VERSION}"

# Conditionally add PostgreSQL repositories based on TARGET_VERSION
RUN set -eux; \
	if [ "$TARGET_VERSION" = "17" ]; then \
		sed -i 's/$/ 16 15 14/' /etc/apt/sources.list.d/pgdg.list; \
	elif [ "$TARGET_VERSION" = "16" ]; then \
		sed -i 's/$/ 15 14/' /etc/apt/sources.list.d/pgdg.list; \
	elif [ "$TARGET_VERSION" = "15" ]; then \
		sed -i 's/$/ 14/' /etc/apt/sources.list.d/pgdg.list; \
	fi

# Install basic packages, PostgreSQL tools, and supervisord using --no-cache approach
RUN apt-get update --allow-insecure-repositories && \
	apt-get install  -y --no-install-recommends --allow-unauthenticated \
		libsodium23 \
		netcat-openbsd \
        build-essential \
        libevent-dev \
        pkg-config \
        libssl-dev \
        libsystemd-dev \
		wget \
		ca-certificates \
		curl \
		openssl \
        supervisor \
        jq \
        socat \
        postgresql-14 postgresql-15 postgresql-16 \
        pgbouncer \
        postgresql-${TARGET_VERSION}-pgtap \
        postgresql-${TARGET_VERSION}-pgvector \
        postgresql-${TARGET_VERSION}-pgaudit \
        postgresql-${TARGET_VERSION}-repack \
        postgresql-${TARGET_VERSION}-cron \
        postgresql-${TARGET_VERSION}-wal2json \
        postgresql-${TARGET_VERSION}-hypopg \
        pgtop \
		&& \
	apt-get clean && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*


# Arguments for versions
ARG POSTGREST_VERSION=12.2.3
ARG WALG_VERSION=3.0.5



# Install binary tools using Taskfile
COPY Taskfile.binaries.yaml /tmp/Taskfile.binaries.yaml

# Bootstrap Task first, then install all binaries
RUN  cd /tmp && \
    # Bootstrap eget and Task first
    EGET_VERSION="1.3.4" && \
    ARCH="$(dpkg --print-architecture)" && \
    curl -fsSL "https://github.com/zyedidia/eget/releases/download/v${EGET_VERSION}/eget-${EGET_VERSION}-linux_${ARCH}.tar.gz" | tar -xzC /tmp && \
    find /tmp -name eget -type f -executable | head -1 | xargs -I {} install {} /usr/local/bin/eget && \
    /usr/local/bin/eget go-task/task --tag v3.44.1 -a ".tar.gz" --to /usr/local/bin && \
    # Now use Task to install everything else
    task --taskfile=Taskfile.binaries.yaml install-postgrest && \
    task --taskfile=Taskfile.binaries.yaml install-walg && \
    rm -f /tmp/eget

# Copy Taskfile for extension installation
COPY Taskfile.extensions.yaml /tmp/Taskfile.extensions.yaml

WORKDIR /tmp

# Install GitHub .deb packages
RUN task --taskfile=Taskfile.extensions.yaml install-github-extensions

# Install build dependencies and PGXN client
RUN task --taskfile=Taskfile.extensions.yaml install-build-deps && \
    task --taskfile=Taskfile.extensions.yaml install-pgxn-client

# Install source-based extensions
RUN task --taskfile=Taskfile.extensions.yaml install-source-extensions

# Create service users
RUN groupadd -g 70 pgbouncer && \
    useradd -r -u 70 -g pgbouncer -d /var/lib/pgbouncer -s /bin/false pgbouncer && \
    groupadd -g 71 postgrest && \
    useradd -r -u 71 -g postgrest -d /var/lib/postgrest -s /bin/false postgrest && \
    groupadd -g 72 walg && \
    useradd -r -u 72 -g walg -d /var/lib/walg -s /bin/false walg

# Create directories with proper permissions
RUN mkdir -p \
    /etc/pgbouncer \
    /etc/postgrest \
    /etc/wal-g \
    /var/lib/pgbouncer \
    /var/lib/postgrest \
    /var/lib/walg \
    /var/log/pgbouncer \
    /var/log/postgrest \
    /var/log/wal-g && \
    chown pgbouncer:pgbouncer /etc/pgbouncer /var/lib/pgbouncer /var/log/pgbouncer && \
    chown postgrest:postgrest /etc/postgrest /var/lib/postgrest /var/log/postgrest && \
    chown walg:walg /etc/wal-g /var/lib/walg /var/log/wal-g

# Ensure proper directory structure and ownership for extensions
RUN mkdir -p /usr/lib/postgresql/17/lib /usr/share/postgresql/17/extension && \
    chown -R postgres:postgres /usr/lib/postgresql/17 /usr/share/postgresql/17

# Set environment variables for all versions
ENV PG14BIN /usr/lib/postgresql/14/bin
ENV PG15BIN /usr/lib/postgresql/15/bin
ENV PG16BIN /usr/lib/postgresql/16/bin
ENV PG17BIN /usr/lib/postgresql/17/bin

# Data directories
ENV PGDATA /var/lib/postgresql/data

# Create data directory
RUN mkdir -p ${PGDATA} && \
	chown -R postgres:postgres /var/lib/postgresql


# Create supervisord directories and set permissions
RUN mkdir -p /var/log/supervisor /etc/supervisor/conf.d && \
    chown -R root:root /var/log/supervisor /etc/supervisor

# Extensions configuration (installed via Nix)
ENV POSTGRES_EXTENSIONS="pgvector,pgsodium,pgjwt,pgaudit,pg_stat_monitor,pg_repack,pg_plan_filter,pg_net,pg_jsonschema,pg_hashids,pg_cron,pg_safeupdate,index_advisor,wal2json,hypopg"

# Service control environment variables
ENV PGBOUNCER_ENABLED=true
ENV POSTGREST_ENABLED=true
ENV WALG_ENABLED=false
ENV HEALTH_SERVICE_ENABLED=false

# Enhanced logging configuration
ENV SERVICE_LOG_FORMAT=simple
ENV SERVICE_LOG_LEVEL=INFO
ENV SERVICE_LOG_TIMESTAMPS=true
ENV SERVICE_METRICS_ENABLED=true

# Health service configuration
ENV HEALTH_PORT=8080
ENV HEALTH_HOST=0.0.0.0

# PostgreSQL environment (backward compatibility)
ENV POSTGRES_DB=postgres
ENV POSTGRES_USER=postgres

# PgBouncer configuration
ENV PGBOUNCER_PORT=6432
ENV PGBOUNCER_MAX_CLIENT_CONN=100
ENV PGBOUNCER_DEFAULT_POOL_SIZE=25
ENV PGBOUNCER_POOL_MODE=transaction
ENV PGBOUNCER_AUTH_TYPE=md5

# PostgREST configuration
ENV POSTGREST_PORT=3000
ENV POSTGREST_DB_SCHEMA=public
ENV POSTGREST_DB_ANON_ROLE=anon
ENV POSTGREST_MAX_ROWS=1000

# WAL-G configuration
ENV WALG_COMPRESSION_METHOD=lz4
ENV WALG_S3_PREFIX=""
ENV PGHOST=/var/run/postgresql

# Ensure postgres user can access mounted volumes (common UID in GitHub Actions)
RUN set -eux; \
	usermod -u 1001 postgres; \
	groupmod -g 1001 postgres

# Expose ports
EXPOSE 5432 6432 3000 8080

WORKDIR /var/lib/postgresql

# Copy supervisord configuration and service scripts
COPY supervisord.conf /etc/supervisord.conf
COPY postgresql-service.sh pgbouncer-service.sh postgrest-service.sh walg-service.sh health-service.sh /usr/local/bin/
COPY scripts/service-logging.sh scripts/health-server.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/postgresql-service.sh /usr/local/bin/pgbouncer-service.sh \
               /usr/local/bin/postgrest-service.sh /usr/local/bin/walg-service.sh \
               /usr/local/bin/health-service.sh /usr/local/bin/service-logging.sh \
               /usr/local/bin/health-server.sh

# Copy Taskfiles and scripts
COPY Taskfile.yml Taskfile.*.yaml /var/lib/postgresql/
COPY docker-entrypoint.sh docker-upgrade-multi /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh /usr/local/bin/docker-upgrade-multi

USER root

# Add Docker health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD /usr/local/bin/health-server.sh check json | jq -e '.overall_status == "healthy"' > /dev/null || exit 1

# Use supervisord as init system
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

# Default command (services will be started by supervisord)
CMD ["supervisord", "-c", "/etc/supervisord.conf"]
