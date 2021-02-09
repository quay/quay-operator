#!/bin/sh

echo "attempting to create pg_trgm extension if it does not exist"

until psql -d $POSTGRESQL_DATABASE -h $POSTGRESQL_DATABASE -U postgres -c 'CREATE EXTENSION IF NOT EXISTS pg_trgm;'
do
  echo "..."
  sleep 1
done

echo "succesfully created pg_trgm extension"
