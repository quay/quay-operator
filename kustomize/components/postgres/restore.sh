#!/bin/sh

# NOTE: The Postgres container will not run this init script if `$PGDATA` is populated, so no need to have extra checks here to prevent re-restoring.
RESTORE_FILE=/var/lib/postgresql/data/dump.sql

if [ -f "$RESTORE_FILE" ]; then
  echo "Restoring DB using $RESTORE_FILE"

  pg_restore -d $POSTGRES_DB -U $POSTGRES_USER --exit-on-error $RESTORE_FILE
else
  echo "No DB restore needed"
fi
