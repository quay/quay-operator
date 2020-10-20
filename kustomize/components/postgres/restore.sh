#!/bin/sh

# FIXME(alecmerdler): Ensure this only runs once...

# FIXME(alecmerdler): This file path is wrong for SCL Postgres...
RESTORE_FILE=/var/lib/postgresql/data/dump.sql

if [ -f "$RESTORE_FILE" ]; then
  echo "Restoring DB using $RESTORE_FILE"

  pg_restore -d $POSTGRES_DB -U $POSTGRES_USER --exit-on-error $RESTORE_FILE
else
  echo "No DB restore needed"
fi
