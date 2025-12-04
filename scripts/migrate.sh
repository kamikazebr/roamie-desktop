#!/bin/bash

set -e

# Load environment variables
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

if [ -z "$DATABASE_URL" ]; then
    echo "Error: DATABASE_URL not set"
    exit 1
fi

echo "Running migrations..."

for migration in cmd/server/migrations/*.sql; do
    echo "Applying: $migration"
    psql "$DATABASE_URL" -f "$migration"
done

echo "Migrations complete!"
