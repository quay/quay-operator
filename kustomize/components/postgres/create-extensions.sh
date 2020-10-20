#!/bin/sh

echo "attempting to create pg_trgm extension"
psql -d $POSTGRESQL_DATABASE -h $POSTGRESQL_DATABASE -U postgres -c 'CREATE EXTENSION pg_trgm;' || true
echo "succesfully created pg_trgm extension"
break
