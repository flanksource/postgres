#!/bin/bash
# PostgreSQL command wrapper that runs as postgres user when root

if [ "$(id -u)" = "0" ]; then
    # Running as root - switch to postgres user for this command
    exec gosu postgres "$@"
else
    # Already running as non-root user
    exec "$@"
fi