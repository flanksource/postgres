# PostgreSQL Docker Image with pgconfig CLI
# Multi-version PostgreSQL (14-17) with auto-upgrade and tuning capabilities
# Supports both AMD64 and ARM64 architectures

# Build stage for pgconfig binary
FROM golang:1.25-bookworm AS pgconfig-builder

# Copy source code
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build pgconfig binary
RUN CGO_ENABLED=0 GOOS=linux go build -o postgres-cli ./cmd

# Main stage
FROM debian:bookworm-slim

# Get architecture information
ARG TARGETARCH
ARG TARGETOS
ENV TARGETARCH=${TARGETARCH}
ENV TARGETOS=${TARGETOS}

# Default PostgreSQL version (can be 14, 15, 16, or 17)
ARG PG_VERSION=17
ENV PG_VERSION=${PG_VERSION}

# Labels
LABEL maintainer="flanksource"
LABEL description="PostgreSQL with pgconfig for auto-upgrades and tuning"
LABEL architecture="multi-arch"

# Add PostgreSQL repository and configure locales
RUN set -eux; \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        wget \
        gnupg \
        lsb-release \
        locales && \
    # Generate en_US.UTF-8 locale for database compatibility
    sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen && \
    locale-gen en_US.UTF-8 && \
    wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /usr/share/keyrings/postgresql-archive-keyring.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/postgresql-archive-keyring.gpg] http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list

# Install PostgreSQL versions 14-17 and essential tools
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        postgresql-14 \
        postgresql-15 \
        postgresql-16 \
        postgresql-17 \
        postgresql-client-14 \
        postgresql-client-15 \
        postgresql-client-16 \
        postgresql-client-17 \
        postgresql-contrib-14 \
        postgresql-contrib-15 \
        postgresql-contrib-16 \
        postgresql-contrib-17 \
        gosu \
        jq \
        curl \
        procps \
        xz-utils \
        zstd && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*

# Copy postgres-cli binary from builder stage
COPY --from=pgconfig-builder /src/postgres-cli /usr/local/bin/postgres-cli
RUN chmod +x /usr/local/bin/postgres-cli

# Ensure postgres user has UID 999 and GID 999 for consistency
RUN usermod -u 999 postgres && groupmod -g 999 postgres && \
    find / -user 100 -exec chown -h postgres {} + 2>/dev/null || true && \
    find / -group 102 -exec chgrp -h postgres {} + 2>/dev/null || true

# Set environment variables for all PostgreSQL versions
ENV PG14BIN=/usr/lib/postgresql/14/bin
ENV PG15BIN=/usr/lib/postgresql/15/bin
ENV PG16BIN=/usr/lib/postgresql/16/bin
ENV PG17BIN=/usr/lib/postgresql/17/bin

# Data directory
ENV PGDATA=/var/lib/postgresql/data
ENV PGBIN=${PG17BIN}

# PostgreSQL default configuration
ENV POSTGRES_DB=postgres
ENV POSTGRES_USER=postgres
ENV POSTGRES_PASSWORD=

# pgconfig configuration
ENV PGCONFIG_CONFIG_DIR=/var/lib/postgresql/config
ENV PGCONFIG_AUTO_UPGRADE=true
ENV PGCONFIG_AUTO_TUNE=true

# Create postgres user and directories
RUN set -eux; \
    # useradd -r -g postgres --uid=999 --home-dir=/var/lib/postgresql --shell=/bin/bash postgres; \
    mkdir -p /var/lib/postgresql ${PGDATA} ${PGCONFIG_CONFIG_DIR} /docker-entrypoint-initdb.d /var/run/postgresql; \
    chown -R postgres:postgres /var/lib/postgresql /var/run/postgresql

COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Make volumes for data and init scripts
VOLUME /var/lib/postgresql/data

# Expose PostgreSQL port
EXPOSE 5432

# Stop signal for graceful shutdown
STOPSIGNAL SIGINT

# Set working directory
WORKDIR /var/lib/postgresql

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD pg_isready -U postgres || exit 1

# Run as postgres user for security (can override with --user root if needed for permission fixes)
USER postgres

# Set entrypoint
ENTRYPOINT ["docker-entrypoint.sh"]

# Default command - run PostgreSQL
# CMD ["postgres"]
