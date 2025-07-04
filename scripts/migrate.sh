#!/bin/bash
set -eo pipefail

# Configuration
PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
MIGRATION_DIR="$PROJECT_ROOT/migrations"
CONFIG_DIR="$PROJECT_ROOT/internal/configs"
SECRETS_DIR="$PROJECT_ROOT/secrets"
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

  # Load password from file if not set
  if [ "$env" = "dev" ] && [ -z "${DEV_DB_PASSWORD}" ] && [ -f "$SECRETS_DIR/.dev_db_password" ]; then
    export DEV_DB_PASSWORD=$(cat "$SECRETS_DIR/.dev_db_password")
  elif [ "$env" = "prod" ] && [ -z "${PROD_DB_PASSWORD}" ] && [ -f "$SECRETS_DIR/.prod_db_password" ]; then
    export PROD_DB_PASSWORD=$(cat "$SECRETS_DIR/.prod_db_password")
  fi

  # Extract MySQL configuration directly (safer than parsing DSN)
  export DB_HOST=$(yq e '.database.host' "$config_file" | tr -d '"')
  export DB_PORT=$(yq e '.database.port' "$config_file" | tr -d '"')
  export DB_USER=$(yq e '.database.user' "$config_file" | tr -d '"')
  export DB_NAME=$(yq e '.database.dbname' "$config_file" | tr -d '"')

  # Use password from environment or config
  if [ "$env" = "dev" ]; then
    export DB_PASSWORD="${DEV_DB_PASSWORD:-$(yq e '.database.password' "$config_file" | sed "s/\${DEV_DB_PASSWORD:-dev_db_password}/dev_db_password/")}"
  else
    export DB_PASSWORD="${PROD_DB_PASSWORD:-$(yq e '.database.password' "$config_file" | sed "s/\${PROD_DB_PASSWORD:-prod_db_password}/prod_db_password/")}"
  fi

  # Force DB_HOST to 127.0.0.1 if it's set to localhost (fixes MySQL user@host issue)
  if [ "$DB_HOST" = "localhost" ]; then
    export DB_HOST="127.0.0.1"
  fi

  log "Using connection: mysql -h $DB_HOST -P $DB_PORT -u $DB_USER -p[hidden] $DB_NAME"

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

  until mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" -e "SELECT 1" >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ $attempt -ge $timeout ]; then
      log "❌ Database connection timed out"
      log "💡 Troubleshooting tips:"
      log "1. Verify MySQL ports are properly exposed (e.g., 3388:3306)"
      log "2. Check if MySQL is running: docker ps"
      log "3. Try connecting manually with:"
      log "   mysql -h $DB_HOST -P $DB_PORT -u $DB_USER -p\$DB_PASSWORD -e \"SELECT 1\""
      exit 1
    fi
    sleep 1
  done
}

run_migrations() {
  local migration_files=($(ls "$MIGRATION_DIR"/*.up.sql | sort))
  for file in "${migration_files[@]}"; do
    log "Applying $(basename "$file")"
    mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" "$DB_NAME" < "$file" || {
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

# Debug connection attempt
log "Debug connection attempt:"
mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASSWORD" -e "SELECT 1" || true

# Wait for database connection
wait_for_db

# Run migrations
run_migrations

log "✅ Migration completed successfully"
