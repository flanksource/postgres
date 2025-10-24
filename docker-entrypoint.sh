#!/bin/bash
set -e
echo "PGDATA is set to: $PGDATA"
echo "Contents of PGDATA directory:"
ls -al $PGDATA

postgres-cli auto-start --pg-tune --auto-upgrade --auto-init --data-dir $PGDATA -vvvv

exec $PGBIN/postgres "$@"
