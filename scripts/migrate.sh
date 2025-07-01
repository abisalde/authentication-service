#!/bin/bash
set -eo pipefail

# Configuration
export LC_ALL=C
PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DEPLOY_DIR="$PROJECT_ROOT/deployments"
CERTS_DIR="$PROJECT_ROOT/scripts/cockroach/certs"
ADMIN_USER="root"
APP_USER="appuser"
DB_NAME="authserviceprod"
TIMEOUT_SECONDS=120
MIGRATION_DIR="$PROJECT_ROOT/migrations"

# --------------------------
# Functions
# --------------------------

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

verify_certificates() {
  if [[ ! -f "$CERTS_DIR/ca.crt" ]] || [[ ! -f "$CERTS_DIR/client.$ADMIN_USER.crt" ]]; then
    log "❌ Certificate files not found in $CERTS_DIR"
    log "Please run ./scripts/setup.sh first"
    exit 1
  fi
}

wait_for_db() {
  local attempt=1
  local max_attempts=$((TIMEOUT_SECONDS/2))
  
  log "⏳ Waiting for CockroachDB to be ready..."
  
  until docker exec lb \
    ./cockroach sql \
    --certs-dir=/cockroach/certs \
    --host=lb \
    --user=$ADMIN_USER \
    --execute="SELECT 1" >/dev/null 2>&1; do
    
    if [ $attempt -ge $max_attempts ]; then
      log "❌ Timed out waiting for CockroachDB"
      exit 1
    fi
    
    log "Attempt $attempt/$max_attempts: Database not ready yet..."
    attempt=$((attempt + 1))
    sleep 2
  done
}

initialize_database() {
  log "🔧 Initializing database structure..."
  
  docker exec -i lb \
    ./cockroach sql \
    --certs-dir=/cockroach/certs \
    --host=lb \
    --user=$ADMIN_USER \
    --execute="$(cat $CERTS_DIR/init.sql)" || {
    log "❌ Database initialization failed"
    exit 1
  }
}

run_migrations() {
  log "🔧 Applying migrations from $MIGRATION_DIR..."
  
  for migration in "$MIGRATION_DIR"/*.up.sql; do
    if [[ -f "$migration" ]]; then
      log "Applying $(basename "$migration")"
      docker exec -i lb \
        ./cockroach sql \
        --certs-dir=/cockroach/certs \
        --host=lb \
        --user=$ADMIN_USER \
        --database=$DB_NAME \
        < "$migration" || {
        log "❌ Migration failed: $(basename "$migration")"
        exit 1
      }
    fi
  done
}

# --------------------------
# Main Script
# --------------------------

log "🚀 Starting database migration process"

# Verify execution context
if [[ ! -f "go.mod" ]]; then
  log "❌ Error: Run this script from your project root directory"
  exit 1
fi

verify_certificates
wait_for_db
initialize_database
run_migrations

log "✅ Database migration completed successfully"