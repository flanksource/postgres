#!/usr/bin/env bash
set -Eeo pipefail
# TODO swap to -Eeuo pipefail above (after handling all potentially-unset variables)

# usage: file_env VAR [DEFAULT]
#    ie: file_env 'XYZ_DB_PASSWORD' 'example'
# (will allow for "$XYZ_DB_PASSWORD_FILE" to fill in the value of
#  "$XYZ_DB_PASSWORD" from a file, especially for Docker's secrets feature)
file_env() {
	local var="$1"
	local fileVar="${var}_FILE"
	local def="${2:-}"
	if [ "${!var:-}" ] && [ "${!fileVar:-}" ]; then
		echo >&2 "error: both $var and $fileVar are set (but are exclusive)"
		exit 1
	fi
	local val="$def"
	if [ "${!var:-}" ]; then
		val="${!var}"
	elif [ "${!fileVar:-}" ]; then
		val="$(< "${!fileVar}")"
	fi
	export "$var"="$val"
	unset "$fileVar"
}

# check to see if this file is being run or sourced from another script
_is_sourced() {
	# https://unix.stackexchange.com/a/215279
	[ "${#BASH_SOURCE[@]}" -ge 2 ] \
		&& [ "${BASH_SOURCE[0]}" != "${BASH_SOURCE[1]}" ]
}

# used to create initial postgres directories and if run as root, ensure ownership to the "postgres" user
docker_create_db_directories() {
	local user; user="$(id -u)"

	mkdir -p "$PGDATA"
	# ignore failure since there are cases where we can't chmod (and PostgreSQL won't need it anyhow)
	chmod 00700 "$PGDATA" || :

	# ignore failure since it will be fine when using the image provided user
	mkdir -p /var/run/postgresql || :
	chmod 03775 /var/run/postgresql || :

	# Create configuration directory for pgconfig
	mkdir -p "$PGCONFIG_CONFIG_DIR" || :

	# if running as root, ensure ownership belongs to postgres user
	if [ "$user" = '0' ]; then
		find "$PGDATA" \! -user postgres -exec chown postgres '{}' +
		find /var/run/postgresql \! -user postgres -exec chown postgres '{}' +
		find "$PGCONFIG_CONFIG_DIR" \! -user postgres -exec chown postgres '{}' +
	fi
}

# initialize empty PGDATA directory with new database via 'initdb'
# arguments to `initdb` can be passed via POSTGRES_INITDB_ARGS or as arguments to this function
# `initdb` automatically creates the "postgres", "template0", and "template1" dbnames
docker_init_database_dir() {
	# "initdb" is particular about the current user existing in "/etc/passwd", so we use "nss_wrapper" to fake that if necessary
	local uid; uid="$(id -u)"
	if ! getent passwd "$uid" &> /dev/null; then
		# see if we can find a suitable "libnss_wrapper.so" (https://salsa.debian.org/sssd-team/nss-wrapper)
		local wrapper
		for wrapper in {/usr,}/lib{/*,}/libnss_wrapper.so; do
			if [ -s "$wrapper" ]; then
				NSS_WRAPPER_PASSWD="$(mktemp)"
				NSS_WRAPPER_GROUP="$(mktemp)"
				export LD_PRELOAD="$wrapper" NSS_WRAPPER_PASSWD NSS_WRAPPER_GROUP
				local gid; gid="$(id -g)"
				echo "postgres:x:$uid:$gid:PostgreSQL:$PGDATA:/bin/false" > "$NSS_WRAPPER_PASSWD"
				echo "postgres:x:$gid:" > "$NSS_WRAPPER_GROUP"
				break
			fi
		done
	fi

	if [ -n "$POSTGRES_INITDB_WALDIR" ]; then
		set -- --waldir "$POSTGRES_INITDB_WALDIR" "$@"
	fi

	# --pwfile refuses to handle a properly-empty file (hence the "\n"): https://github.com/docker-library/postgres/issues/1025
	eval 'initdb --username="$POSTGRES_USER" --pwfile=<(printf "%s\n" "$POSTGRES_PASSWORD") '"$POSTGRES_INITDB_ARGS"' "$@"'

	# unset/cleanup "nss_wrapper" bits
	if [[ "${LD_PRELOAD:-}" == */libnss_wrapper.so ]]; then
		rm -f "$NSS_WRAPPER_PASSWD" "$NSS_WRAPPER_GROUP"
		unset LD_PRELOAD NSS_WRAPPER_PASSWD NSS_WRAPPER_GROUP
	fi
}

# print large warning if POSTGRES_PASSWORD is long
# error if both POSTGRES_PASSWORD is empty and POSTGRES_HOST_AUTH_METHOD is not 'trust'
docker_verify_minimum_env() {
	if [ "${#POSTGRES_PASSWORD}" -ge 100 ]; then
		cat >&2 <<-'EOWARN'
			WARNING: The supplied POSTGRES_PASSWORD is 100+ characters.

			  This will not work if used via PGPASSWORD with "psql".

			  https://www.postgresql.org/message-id/flat/E1Rqxp2-0004Qt-PL%40wrigleys.postgresql.org (BUG #6412)
			  https://github.com/docker-library/postgres/issues/507

		EOWARN
	fi
	if [ -z "$POSTGRES_PASSWORD" ] && [ 'trust' != "$POSTGRES_HOST_AUTH_METHOD" ]; then
		# The - option suppresses leading tabs but *not* spaces. :)
		cat >&2 <<-'EOE'
			Error: Database is uninitialized and superuser password is not specified.
			       You must specify POSTGRES_PASSWORD to a non-empty value for the
			       superuser. For example, "-e POSTGRES_PASSWORD=password" on "docker run".

			       You may also use "POSTGRES_HOST_AUTH_METHOD=trust" to allow all
			       connections without a password. This is *not* recommended.

			       See PostgreSQL documentation about "trust":
			       https://www.postgresql.org/docs/current/auth-trust.html
		EOE
		exit 1
	fi
	if [ 'trust' = "$POSTGRES_HOST_AUTH_METHOD" ]; then
		cat >&2 <<-'EOWARN'
			********************************************************************************
			WARNING: POSTGRES_HOST_AUTH_METHOD has been set to "trust". This will allow
			         anyone with access to the Postgres port to access your database without
			         a password, even if POSTGRES_PASSWORD is set. See PostgreSQL
			         documentation about "trust":
			         https://www.postgresql.org/docs/current/auth-trust.html
			         In Docker's default configuration, this is effectively any other
			         container on the same system.

			         It is not recommended to use POSTGRES_HOST_AUTH_METHOD=trust. Replace
			         it with "-e POSTGRES_PASSWORD=password" instead to set a password in
			         "docker run".
			********************************************************************************
		EOWARN
	fi
}

# usage: docker_process_init_files [file [file [...]]]
#    ie: docker_process_init_files /docker-entrypoint-initdb.d/*
# process initializer files, based on file extensions
docker_process_init_files() {
	# psql here for backwards compatibility "${psql[@]}"
	psql=( docker_process_sql )

	echo
	local f
	for f; do
		case "$f" in
			*.sh)
				# https://github.com/docker-library/postgres/issues/450#issuecomment-393167936
				# https://github.com/docker-library/postgres/pull/452
				if [ -x "$f" ]; then
					echo "$0: running $f"
					"$f"
				else
					echo "$0: sourcing $f"
					# shellcheck disable=SC1090
					. "$f"
				fi
				;;
			*.sql)     echo "$0: running $f"; docker_process_sql -f "$f"; echo ;;
			*.sql.gz)  echo "$0: running $f"; gunzip -c "$f" | docker_process_sql; echo ;;
			*.sql.xz)  echo "$0: running $f"; xzcat "$f" | docker_process_sql; echo ;;
			*.sql.zst) echo "$0: running $f"; zstd -dc "$f" | docker_process_sql; echo ;;
			*)         echo "$0: ignoring $f" ;;
		esac
		echo
	done
}

# Execute sql script, passed via stdin (or -f flag of psql)
# usage: docker_process_sql [psql-cli-args]
#    ie: docker_process_sql --dbname=mydb <<<'INSERT ...'
#    ie: docker_process_sql -f my-file.sql
#    ie: docker_process_sql <my-file.sql
docker_process_sql() {
	local query_runner=( psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --no-password --no-psqlrc )
	if [ -n "$POSTGRES_DB" ]; then
		query_runner+=( --dbname "$POSTGRES_DB" )
	fi

	PGUSER="${PGUSER:-$POSTGRES_USER}" \
	"${query_runner[@]}" "$@"
}

# create initial database
# uses environment variables for input: POSTGRES_DB
docker_setup_db() {
	local dbAlreadyExists
	dbAlreadyExists="$(
		POSTGRES_DB= docker_process_sql --dbname postgres --set db="$POSTGRES_DB" --tuples-only <<-'EOSQL'
			SELECT 1 FROM pg_database WHERE datname = :'db' ;
		EOSQL
	)"
	if [ -z "$dbAlreadyExists" ]; then
		POSTGRES_DB= docker_process_sql --dbname postgres --set db="$POSTGRES_DB" <<-'EOSQL'
			CREATE DATABASE :"db" ;
		EOSQL
		echo
	fi
}

# Append POSTGRES_HOST_AUTH_METHOD to pg_hba.conf for "host" connections
# all arguments will be passed along as arguments to `postgres` for getting the setting configured
docker_setup_hba_conf() {
	# allow connections from anywhere with password by default
	{
		echo
		if [ 'trust' = "$POSTGRES_HOST_AUTH_METHOD" ]; then
			echo '# warning trust is enabled for all connections'
			echo '# see https://www.postgresql.org/docs/current/auth-trust.html'
		fi
		echo "host all all all $POSTGRES_HOST_AUTH_METHOD"
	} >> "$PGDATA/pg_hba.conf"
}

# start socket-only postgresql server for setting up or running scripts
# all arguments will be passed along as arguments to `postgres` (via pg_ctl)
docker_temp_server_start() {
	if [ "$1" = 'postgres' ]; then
		shift
	fi

	# internal start of server in order to allow setup using psql client
	# does not listen on external TCP/IP and waits until start finishes
	set -- "$@" -c listen_addresses='' -p "${PGPORT:-5432}"

	PGUSER="${PGUSER:-$POSTGRES_USER}" \
	pg_ctl -D "$PGDATA" \
		-o "$(printf '%q ' "$@")" \
		-w start
}

# stop postgresql server after done setting up user and running scripts
docker_temp_server_stop() {
	PGUSER="${PGUSER:-postgres}" \
	pg_ctl -D "$PGDATA" -m fast -w stop
}

# Detect cgroups memory limit
docker_detect_memory_limit() {
	local mem_limit=""

	# Try cgroup v2 first
	if [ -f /sys/fs/cgroup/memory.max ]; then
		mem_limit=$(cat /sys/fs/cgroup/memory.max)
		if [ "$mem_limit" != "max" ]; then
			echo "$mem_limit"
			return
		fi
	fi

	# Try cgroup v1
	if [ -f /sys/fs/cgroup/memory/memory.limit_in_bytes ]; then
		mem_limit=$(cat /sys/fs/cgroup/memory/memory.limit_in_bytes)
		# Check if it's not the default max value (very large number)
		if [ "$mem_limit" -lt 9223372036854771712 ]; then
			echo "$mem_limit"
			return
		fi
	fi

	# Fallback to total system memory
	if [ -f /proc/meminfo ]; then
		mem_kb=$(grep MemTotal /proc/meminfo | awk '{print $2}')
		echo $((mem_kb * 1024))
	fi
}

# Convert bytes to MB
docker_bytes_to_mb() {
	echo $(($1 / 1024 / 1024))
}

# Auto-tune PostgreSQL based on available memory
docker_pgconfig_autotune() {
	echo
	echo 'PostgreSQL pgconfig: Auto-tuning configuration'

	local mem_bytes mem_mb
	mem_bytes=$(docker_detect_memory_limit)
	mem_mb=$(docker_bytes_to_mb "$mem_bytes")

	echo "PostgreSQL pgconfig: Detected memory limit: ${mem_mb} MB"

	# Calculate PostgreSQL settings based on available memory
	# These are conservative estimates suitable for mixed workloads
	local shared_buffers effective_cache_size maintenance_work_mem work_mem
	shared_buffers=$((mem_mb / 4))
	effective_cache_size=$((mem_mb * 3 / 4))
	maintenance_work_mem=$((mem_mb / 16))
	work_mem=$((mem_mb / 64))

	# Ensure minimum values
	[ "$shared_buffers" -lt 128 ] && shared_buffers=128
	[ "$maintenance_work_mem" -lt 64 ] && maintenance_work_mem=64
	[ "$work_mem" -lt 4 ] && work_mem=4

	echo "PostgreSQL pgconfig: Applying tuning: shared_buffers=${shared_buffers}MB, effective_cache_size=${effective_cache_size}MB"

	# Use pgconfig CLI to apply tuning if available
	if command -v pgconfig >/dev/null 2>&1; then
		echo "PostgreSQL pgconfig: Using pgconfig CLI to tune database"
		pgconfig tune \
			--memory-mb="$mem_mb" \
			--db-type="${PGCONFIG_DB_TYPE:-mixed}" \
			--max-connections="${PGCONFIG_MAX_CONNECTIONS:-100}" 2>/dev/null || {
			echo "PostgreSQL pgconfig: pgconfig tune failed, using manual settings"
		}
	fi

	# Append tuning to postgresql.conf
	cat >> "$PGDATA/postgresql.conf" <<-EOCONF

	# pgconfig: Auto-tuning based on detected memory (${mem_mb} MB)
	shared_buffers = ${shared_buffers}MB
	effective_cache_size = ${effective_cache_size}MB
	maintenance_work_mem = ${maintenance_work_mem}MB
	work_mem = ${work_mem}MB
	max_connections = ${PGCONFIG_MAX_CONNECTIONS:-100}

	# pgconfig: Additional performance settings
	wal_buffers = 16MB
	checkpoint_completion_target = 0.9
	random_page_cost = 1.1
	effective_io_concurrency = 200
	EOCONF

	echo
}

# Perform PostgreSQL version upgrade
docker_pgconfig_upgrade() {
	local old_version="$1"
	local new_version="$2"

	echo
	echo "PostgreSQL pgconfig: Upgrading from version $old_version to $new_version"

	local old_bindir="/usr/lib/postgresql/$old_version/bin"
	local new_bindir="/usr/lib/postgresql/$new_version/bin"

	# Create backup of old data
	local backup_dir="${PGDATA}_backup_v${old_version}"
	if [ ! -d "$backup_dir" ]; then
		echo "PostgreSQL pgconfig: Creating backup at $backup_dir"
		cp -a "$PGDATA" "$backup_dir"
	fi

	# Perform upgrade using pg_upgrade
	local new_pgdata="${PGDATA}_new"
	mkdir -p "$new_pgdata"
	chmod 700 "$new_pgdata"

	echo "PostgreSQL pgconfig: Initializing new cluster for version $new_version"
	PGDATA="$new_pgdata" docker_init_database_dir

	echo "PostgreSQL pgconfig: Running pg_upgrade check"
	cd /tmp
	"$new_bindir/pg_upgrade" \
		--old-datadir="$PGDATA" \
		--new-datadir="$new_pgdata" \
		--old-bindir="$old_bindir" \
		--new-bindir="$new_bindir" \
		--username="$POSTGRES_USER" \
		--link \
		--check || {
			echo >&2 "PostgreSQL pgconfig: pg_upgrade check failed"
			exit 1
		}

	echo "PostgreSQL pgconfig: Running pg_upgrade"
	"$new_bindir/pg_upgrade" \
		--old-datadir="$PGDATA" \
		--new-datadir="$new_pgdata" \
		--old-bindir="$old_bindir" \
		--new-bindir="$new_bindir" \
		--username="$POSTGRES_USER" \
		--link || {
			echo >&2 "PostgreSQL pgconfig: pg_upgrade failed"
			exit 1
		}

	# Move old data and replace with new
	local old_data_dir="${PGDATA}_old"
	mv "$PGDATA" "$old_data_dir"
	mv "$new_pgdata" "$PGDATA"

	echo "PostgreSQL pgconfig: Upgrade completed successfully"
	echo "PostgreSQL pgconfig: Old data backed up to $old_data_dir"
	echo
}

# Reset admin password
docker_pgconfig_reset_password() {
	echo
	echo 'PostgreSQL pgconfig: Resetting admin password'

	# Start temporary server
	docker_temp_server_start "$@"

	# Reset password
	docker_process_sql --dbname postgres --set passwd="$POSTGRES_PASSWORD" <<-'EOSQL'
		ALTER USER postgres WITH PASSWORD :'passwd' ;
	EOSQL

	# Stop temporary server
	docker_temp_server_stop

	echo
}

_main() {
	# if first arg looks like a flag, assume we want to run postgres server
	if [ "${1:0:1}" = '-' ]; then
		set -- postgres "$@"
	fi

	if [ "$1" = 'postgres' ] && ! _is_sourced; then
		# Load various environment variables from files
		file_env 'POSTGRES_PASSWORD'
		file_env 'POSTGRES_USER' 'postgres'
		file_env 'POSTGRES_DB' "$POSTGRES_USER"
		file_env 'POSTGRES_INITDB_ARGS'
		file_env 'POSTGRES_INITDB_WALDIR'
		file_env 'POSTGRES_HOST_AUTH_METHOD'
		file_env 'PGCONFIG_AUTO_UPGRADE' 'true'
		file_env 'PGCONFIG_AUTO_TUNE' 'true'
		file_env 'PGCONFIG_RESET_PASSWORD' 'false'
		# shellcheck disable=SC2034
		declare -g DATABASE_ALREADY_EXISTS
		# look specifically for PG_VERSION, as it is expected in the DB dir
		if [ -s "$PGDATA/PG_VERSION" ]; then
			DATABASE_ALREADY_EXISTS='true'
		fi

		# Determine PostgreSQL version to use
		local pg_version="${PG_VERSION:-17}"
		local pg_bindir="/usr/lib/postgresql/$pg_version/bin"

		# Update PATH to include selected PostgreSQL version
		export PATH="$pg_bindir:$PATH"

		# allow the container to be started with `--user`
		if [ "$(id -u)" = '0' ]; then
			docker_create_db_directories
			exec gosu postgres "$BASH_SOURCE" "$@"
		fi

		docker_create_db_directories

		# only run initialization on an empty data directory
		if [ -z "$DATABASE_ALREADY_EXISTS" ]; then
			docker_verify_minimum_env

			# check dir permissions to reduce likelihood of half-initialized database
			ls /docker-entrypoint-initdb.d/ > /dev/null

			echo
			echo 'PostgreSQL init process in progress...'
			echo

			docker_init_database_dir
			docker_setup_hba_conf

			# PGPASSWORD is required for psql when authentication is required for 'local' connections via pg_hba.conf and is otherwise harmless
			# e.g. when '--auth=md5' or '--auth-local=md5' is used in POSTGRES_INITDB_ARGS
			export PGPASSWORD="${PGPASSWORD:-$POSTGRES_PASSWORD}"
			docker_temp_server_start "$@"

			docker_setup_db
			docker_process_init_files /docker-entrypoint-initdb.d/*

			# Apply auto-tuning if enabled
			if [ "$PGCONFIG_AUTO_TUNE" = "true" ]; then
				docker_temp_server_stop
				docker_pgconfig_autotune
				docker_temp_server_start "$@"
			fi

			docker_temp_server_stop

			unset PGPASSWORD

			echo
			echo 'PostgreSQL init process complete; ready for start up.'
			echo
		else
			# Database already exists
			local installed_version
			installed_version=$(cat "$PGDATA/PG_VERSION")
			echo
			echo "PostgreSQL Database directory appears to contain a database; Version: $installed_version"

			# Check if upgrade is needed
			if [ "$PGCONFIG_AUTO_UPGRADE" = "true" ] && [ "$installed_version" != "$pg_version" ]; then
				docker_verify_minimum_env
				export PGPASSWORD="${PGPASSWORD:-$POSTGRES_PASSWORD}"

				docker_pgconfig_upgrade "$installed_version" "$pg_version"

				# Update PATH to use new version
				pg_bindir="/usr/lib/postgresql/$pg_version/bin"
				export PATH="$pg_bindir:$PATH"

				# Apply auto-tuning after upgrade if enabled
				if [ "$PGCONFIG_AUTO_TUNE" = "true" ]; then
					docker_pgconfig_autotune
				fi

				unset PGPASSWORD
			fi

			# Reset password if requested
			if [ -n "$POSTGRES_PASSWORD" ] && [ "$PGCONFIG_RESET_PASSWORD" = "true" ]; then
				docker_verify_minimum_env
				export PGPASSWORD="${PGPASSWORD:-$POSTGRES_PASSWORD}"

				docker_pgconfig_reset_password "$@"

				unset PGPASSWORD
			fi
		fi
	fi

	exec "$@"
}

# If we are sourced from elsewhere, don't perform any further actions
if ! _is_sourced; then
	_main "$@"
fi
