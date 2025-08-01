FROM postgres:17-bookworm

# Target version for upgrades (can be 15, 16, or 17)
ARG TARGET_VERSION=17
ENV TARGET_VERSION=${TARGET_VERSION}

# Add PostgreSQL 14, 15, and 16 repositories
RUN sed -i 's/$/ 16 15 14/' /etc/apt/sources.list.d/pgdg.list

# Install PostgreSQL versions 14, 15, and 16
RUN set -eux; \
	apt-get update; \
	apt-get install -y --no-install-recommends \
		postgresql-14='14.18-1.pgdg120+1' \
		postgresql-15='15.13-1.pgdg120+1' \
		postgresql-16='16.9-1.pgdg120+1' \
		gosu \
		curl \
		ca-certificates \
	; \
	rm -rf /var/lib/apt/lists/*

# Set environment variables for all versions
ENV PG14BIN /usr/lib/postgresql/14/bin
ENV PG15BIN /usr/lib/postgresql/15/bin
ENV PG16BIN /usr/lib/postgresql/16/bin
ENV PG17BIN /usr/lib/postgresql/17/bin

# Data directories
ENV PG14DATA /var/lib/postgresql/14/data
ENV PG15DATA /var/lib/postgresql/15/data
ENV PG16DATA /var/lib/postgresql/16/data
ENV PG17DATA /var/lib/postgresql/17/data

# Create data directories
RUN set -eux; \
	mkdir -p "$PG14DATA" "$PG15DATA" "$PG16DATA" "$PG17DATA"; \
	chown -R postgres:postgres /var/lib/postgresql

# Install task for running pre/post sanity tests
RUN set -eux; \
	curl -fsSL https://taskfile.dev/install.sh | sh -s -- -d -b /usr/local/bin

WORKDIR /var/lib/postgresql

COPY Taskfile.yml /var/lib/postgresql/
COPY docker-upgrade-multi /usr/local/bin/
COPY docker-entrypoint.sh /usr/local/bin/

ENTRYPOINT ["docker-entrypoint.sh"]

# Default: auto-detect version and upgrade to TARGET_VERSION
CMD []