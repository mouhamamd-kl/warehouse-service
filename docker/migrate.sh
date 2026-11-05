#!/bin/sh
set -eu

psql -v ON_ERROR_STOP=1 -c 'CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW());'

for migration in /migrations/*.sql; do
  version=$(basename "$migration")
  applied=$(psql -At -v ON_ERROR_STOP=1 -c "SELECT 1 FROM schema_migrations WHERE version = '$version';")
  if [ "$applied" = "1" ]; then
    echo "Already applied: $version"
    continue
  fi
  echo "Applying: $version"
  # Goose files contain both Up and Down SQL. Execute only the Up section.
  {
    awk '/^-- \+goose Up/{up=1; next} /^-- \+goose Down/{up=0} up{print}' "$migration"
    printf "\nINSERT INTO schema_migrations(version) VALUES ('%s');\n" "$version"
  } | psql -v ON_ERROR_STOP=1 --single-transaction
done
