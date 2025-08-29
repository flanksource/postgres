FROM postgres:17-bookworm

# Target version for upgrades (can be 15, 16, or 17)
ARG TARGET_VERSION=17
ENV TARGET_VERSION=${TARGET_VERSION}

# Conditionally add PostgreSQL repositories based on TARGET_VERSION
RUN set -eux; \
	if [ "$TARGET_VERSION" = "17" ]; then \
		sed -i 's/$/ 16 15 14/' /etc/apt/sources.list.d/pgdg.list; \
	elif [ "$TARGET_VERSION" = "16" ]; then \
		sed -i 's/$/ 15 14/' /etc/apt/sources.list.d/pgdg.list; \
	elif [ "$TARGET_VERSION" = "15" ]; then \
		sed -i 's/$/ 14/' /etc/apt/sources.list.d/pgdg.list; \
	fi

# Install only necessary PostgreSQL versions based on TARGET_VERSION
RUN set -eux; \
	apt-get update; \
	apt-get install -y --no-install-recommends \
		gosu \
		curl \
		ca-certificates \
	; \
	if [ "$TARGET_VERSION" = "17" ] || [ "$TARGET_VERSION" = "16" ] || [ "$TARGET_VERSION" = "15" ]; then \
		apt-get install -y --no-install-recommends postgresql-14; \
	fi; \
	if [ "$TARGET_VERSION" = "17" ] || [ "$TARGET_VERSION" = "16" ]; then \
		apt-get install -y --no-install-recommends postgresql-15; \
	fi; \
	if [ "$TARGET_VERSION" = "17" ]; then \
		apt-get install -y --no-install-recommends postgresql-16; \
	fi; \
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

# Create only necessary data directories based on TARGET_VERSION
RUN set -eux; \
	if [ "$TARGET_VERSION" = "17" ] || [ "$TARGET_VERSION" = "16" ] || [ "$TARGET_VERSION" = "15" ]; then \
		mkdir -p "$PG14DATA"; \
	fi; \
	if [ "$TARGET_VERSION" = "17" ] || [ "$TARGET_VERSION" = "16" ]; then \
		mkdir -p "$PG15DATA"; \
	fi; \
	if [ "$TARGET_VERSION" = "17" ]; then \
		mkdir -p "$PG16DATA"; \
	fi; \
	mkdir -p "/var/lib/postgresql/${TARGET_VERSION}/data"; \
	chown -R postgres:postgres /var/lib/postgresql

# Install task for running pre/post sanity tests
RUN set -eux; \
	curl -fsSL https://taskfile.dev/install.sh | sh -s -- -d -b /usr/local/bin

WORKDIR /var/lib/postgresql

COPY Taskfile.yml Taskfile.*.yaml /var/lib/postgresql/
COPY docker-entrypoint.sh docker-upgrade-multi /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh /usr/local/bin/docker-upgrade-multi

ENTRYPOINT ["docker-entrypoint.sh"]

# Default: auto-detect version and upgrade to TARGET_VERSION
CMD []
