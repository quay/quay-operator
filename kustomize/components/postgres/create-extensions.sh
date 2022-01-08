#!/bin/bash

echo "attempting to create pg_trgm extension if it does not exist"

_psql () { psql --set ON_ERROR_STOP=1 "$@" ; }

_psql -d $POSTGRESQL_DATABASE <<<"CREATE EXTENSION IF NOT EXISTS pg_trgm;"
