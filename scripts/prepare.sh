#!/bin/bash

set -ex

go mod tidy

# Early return if database is already created.
if [[ "$1" == "not_create_database" ]];
    then
        exit 0
    fi

PSQL_HOST="localhost"
PSQL_PORT="5432"
PSQL_USER="validator"
PSQL_PASSWORD="val1dat0r"
PSQL_DB_NAME="project-sem-1"
PSQL_DEFAULT_DB_NAME="postgres"
export PGPASSWORD="$PSQL_PASSWORD"

# Drop & create database (to start from scratch).
psql -U "$PSQL_USER" -h "$PSQL_HOST" -p "$PSQL_PORT" -d "$PSQL_DEFAULT_DB_NAME" -c "DROP DATABASE IF EXISTS \"$PSQL_DB_NAME\";"
psql -U "$PSQL_USER" -h "$PSQL_HOST" -p "$PSQL_PORT" -d "$PSQL_DEFAULT_DB_NAME" -c "CREATE DATABASE \"$PSQL_DB_NAME\";"

psql -U "$PSQL_USER" -h "$PSQL_HOST" -p "$PSQL_PORT" -d "$PSQL_DB_NAME" -c "
CREATE TABLE IF NOT EXISTS prices (
    id SERIAL PRIMARY KEY,
    name TEXT,
    category TEXT,
    price DECIMAL(12,2),
    create_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);"
