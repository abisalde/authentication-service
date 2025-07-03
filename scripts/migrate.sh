#!/bin/bash
set -eo pipefail

# Configuration
PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
MIGRATION_DIR="$PROJECT_ROOT/migrations"
CONFIG_DIR="$PROJECT_ROOT/internal/configs"
CERTS_DIR="$PROJECT_ROOT/scripts/cockroach/certs"
TIMEOUT_SECONDS=60  # Increased timeout

# --------------------------
# Functions
# --------------------------

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

load_config() {
  local env=${1:-dev}
  local config_file="$CONFIG_DIR/$env.yml"

  # Check if config file exists
  if [ ! -f "$config_file" ]; then
    log "❌ Config file not found: $config_file"
    exit 1
  fi


  RAW_PASSWORD=$(yq e '.database.password' "$config_file")

  if [[ "$RAW_PASSWORD" == \${* ]]; then
    # Extract variable name (e.g. DEV_DB_PASSWORD)
    VAR_NAME=$(echo "$RAW_PASSWORD" | sed -E 's/^\$\{([A-Za-z_][A-Za-z0-9_]*)[:-].*$/\1/')
    VAR_NAME_LOWER=$(echo "$VAR_NAME" | tr '[:upper:]' '[:lower:]')
    PASSWORD_FILE="$CERTS_DIR/.${VAR_NAME_LOWER}"

  if [ -f "$PASSWORD_FILE" ]; then
    export DB_PASSWORD=$(<"$PASSWORD_FILE")
  else
    # Extract default value (e.g. dev_db_password)
    DEFAULT_VALUE=$(echo "$RAW_PASSWORD" | sed -n 's/.*:-\([^}]*\)}/\1/p')
    export DB_PASSWORD="${DEFAULT_VALUE:-}"
  fi
  else
    export DB_PASSWORD="$RAW_PASSWORD"
  fi

 
    log "Looking for password file: $PASSWORD_FILE"



  
   # Extract database configuration
  export DB_HOST=$(yq e '.database.host' "$config_file")
  export DB_PORT=$(yq e '.database.port' "$config_file")
  export DB_USER=$(yq e '.database.user' "$config_file")
  export DB_NAME=$(yq e '.database.dbname' "$config_file")
  export DB_SSLMODE=$(yq e '.database.sslmode' "$config_file")

  # Handle environment variable substitution in password
  if [[ "$DB_PASSWORD" == \${* ]]; then
    export DB_PASSWORD=$(eval echo "$DB_PASSWORD")
  fi

  # Verify all required variables are set
  if [ -z "$DB_HOST" ] || [ -z "$DB_PORT" ] || [ -z "$DB_USER" ] || [ -z "$DB_NAME" ]; then
    log "❌ Missing required database configuration in $config_file"
    exit 1
  fi
}

wait_for_db() {
  local timeout=${1:-$TIMEOUT_SECONDS}
  local attempt=0
  
  log "⏳ Waiting for database to be ready (timeout: ${timeout}s)..."
  log "ℹ️ Trying connection to: host=$DB_HOST port=$DB_PORT user=$DB_USER dbname=$DB_NAME"

  until PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1" >/dev/null 2>&1 || \
        PGPASSWORD="$DB_PASSWORD" psql -h "127.0.0.1" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1" >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ $attempt -ge $timeout ]; then
      log "❌ Database connection timed out"
      log "💡 Troubleshooting tips:"
      log "1. Verify PostgreSQL ports are properly exposed (5432:5432)"
      log "2. Try connecting manually with:"
      log "   PGPASSWORD=\$(cat scripts/cockroach/certs/.dev_db_password) psql -h 127.0.0.1 -p 5432 -U root -d authservicelocal -c \"SELECT 1\""
      exit 1
    fi
    sleep 1
  done
}

run_migrations() {
  local migration_files=($(ls "$MIGRATION_DIR"/*.up.sql | sort))
  
  for file in "${migration_files[@]}"; do
    log "Applying $(basename "$file")"
    PGPASSWORD=$(cat scripts/cockroach/certs/.dev_db_password) psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
      -v ON_ERROR_STOP=1 \
      -f "$file" || {
      log "❌ Migration failed: $(basename "$file")"
      exit 1
    }
  done
}
# --------------------------
# Main Script
# --------------------------

log "🚀 Starting database migration"

# Determine environment
ENVIRONMENT="dev"
if [ "$1" = "--prod" ] || [ "$1" = "-p" ]; then
  ENVIRONMENT="prod"
  log "🔧 Production mode selected"
fi

# Load configuration
load_config "$ENVIRONMENT"


# Right before the wait_for_db call add:
log "Debug connection attempt:"
PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1" || true
PGPASSWORD="$DB_PASSWORD" psql -h "127.0.0.1" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1" || true

# Wait for database connection
wait_for_db

# Run migrations
run_migrations

log "✅ Migration completed successfully"