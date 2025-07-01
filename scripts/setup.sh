#!/bin/bash
set -eo pipefail

# Configuration
export LC_ALL=C
PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
VOLUME_PREFIX="auth-roach"
NETWORK_NAME="auth-net"
DEPLOY_DIR="$PROJECT_ROOT/deployments"
CERTS_DIR="$PROJECT_ROOT/scripts/cockroach/certs"
NODE_COUNT=3
COCKROACH_VERSION="v23.2.0"
CERT_LIFETIME="8750h"
ADMIN_USER="root"
APP_USER="appuser"
DB_NAME="authserviceprod"

# --------------------------
# Functions
# --------------------------

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

generate_password() {
  < /dev/urandom tr -dc 'A-Za-z0-9!#$%&()*+,-./:;<=>?@[\]^_{|}~' | head -c 32 2>/dev/null || true
}

verify_directories() {
  log "📂 Verifying directory structure..."
  mkdir -p "$CERTS_DIR" || {
    log "❌ Failed to create certificates directory"
    exit 1
  }
}

verify_project_root() {
  if [[ ! -f "go.mod" ]]; then
    log "❌ Error: Run this script from your project root directory"
    exit 1
  fi
}

# --------------------------
# Main Script
# --------------------------

log "🚀 Starting CockroachDB cluster setup"

# Verify execution context
verify_project_root
verify_directories

# 1. Create Docker resources
log "🔧 Creating Docker volumes and network..."
for i in $(seq 1 $NODE_COUNT); do
  docker volume create "${VOLUME_PREFIX}-${i}-data" >/dev/null || {
    log "❌ Failed to create volume ${VOLUME_PREFIX}-${i}-data"
    exit 1
  }
done

docker network create -d bridge "$NETWORK_NAME" >/dev/null || {
  log "❌ Failed to create network $NETWORK_NAME"
  exit 1
}

# 2. Generate certificates
log "🔐 Generating certificates..."

# CA Certificate
# Create certificates directly on host first
docker run --rm -v "$CERTS_DIR:/certs" cockroachdb/cockroach:$COCKROACH_VERSION \
  cert create-ca --certs-dir=/certs --ca-key=/certs/ca.key --allow-ca-key-reuse || {
  log "❌ Failed to generate CA certificate"
  exit 1
}

# Node Certificates
docker run --rm -v "$CERTS_DIR:/certs" cockroachdb/cockroach:$COCKROACH_VERSION \
  cert create-node localhost 127.0.0.1 auth-roach-1 auth-roach-2 auth-roach-3 lb $(hostname) *.auth-net auth-net \
  --certs-dir=/certs --ca-key=/certs/ca.key || {
  log "❌ Failed to generate node certificates"
  exit 1
}

# Client Certificates
docker run --rm -v "$CERTS_DIR:/certs" cockroachdb/cockroach:$COCKROACH_VERSION \
  cert create-client "$ADMIN_USER" --certs-dir=/certs --ca-key=/certs/ca.key || {
  log "❌ Failed to generate admin certificate"
  exit 1
}

docker run --rm -v "$CERTS_DIR:/certs" cockroachdb/cockroach:$COCKROACH_VERSION \
  cert create-client "$APP_USER" --certs-dir=/certs --ca-key=/certs/ca.key || {
  log "❌ Failed to generate app user certificate"
  exit 1
}

# 3. Set proper permissions
log "🔒 Setting file permissions..."
chmod 600 "$CERTS_DIR"/*.key
chmod 644 "$CERTS_DIR"/*.crt

# 4. Create database initialization script
log "📝 Generating initialization SQL..."
APP_USER_PASSWORD=$(generate_password)
cat > "$CERTS_DIR/init.sql" <<EOF
CREATE DATABASE IF NOT EXISTS $DB_NAME;
CREATE USER IF NOT EXISTS $APP_USER WITH PASSWORD '$APP_USER_PASSWORD';
GRANT ALL ON DATABASE $DB_NAME TO $APP_USER;
EOF

# Store password securely
echo "$APP_USER_PASSWORD" > "$CERTS_DIR/.db_password"
chmod 400 "$CERTS_DIR/.db_password"

# 5. Generate Docker Compose override
log "🐳 Generating Docker Compose override..."
mkdir -p "$DEPLOY_DIR"
cat > "$DEPLOY_DIR/docker-compose.override.yml" <<EOF
services:
  auth-roach-1:
    extends:
      file: docker-compose.yml
      service: cockroachdb
    command:
      - start
      - --certs-dir=/cockroach/certs
      - --listen-addr=:36257
      - --sql-addr=:26257
      - --advertise-sql-addr=auth-roach-1:26257
      - --cache=25%
      - --join=auth-roach-1,auth-roach-2,auth-roach-3
    volumes:
      - ${VOLUME_PREFIX}-1-data:/cockroach/cockroach-data
      - $CERTS_DIR:/cockroach/certs
      - $CERTS_DIR/init.sql:/docker-entrypoint-initdb.d/init.sql
    

  auth-roach-2:
    extends:
      file: docker-compose.yml
      service: cockroachdb
    image: cockroachdb/cockroach:$COCKROACH_VERSION
    command: 
      - start 
      - --certs-dir=/cockroach/certs
      - --listen-addr=:36257
      - --sql-addr=:26257
      - --advertise-sql-addr=auth-roach-2:26257
      - --join=auth-roach-1,auth-roach-2,auth-roach-3 
      - --cache=25%
    volumes:
      - ${VOLUME_PREFIX}-2-data:/cockroach/cockroach-data
      - $CERTS_DIR:/cockroach/certs

  auth-roach-3:
    extends:
      file: docker-compose.yml
      service: cockroachdb
    command: 
      - start 
      - --certs-dir=/cockroach/certs
      - --listen-addr=:36257
      - --sql-addr=:26257
      - --advertise-sql-addr=auth-roach-3:26257
      - --join=auth-roach-1,auth-roach-2,auth-roach-3
      - --cache=25%
    volumes:
      - ${VOLUME_PREFIX}-3-data:/cockroach/cockroach-data
      - $CERTS_DIR:/cockroach/certs

  lb:
    image: cockroachdb/cockroach:$COCKROACH_VERSION
    command: 
      - start
      - --join=auth-roach-1,auth-roach-2,auth-roach-3
      - --certs-dir=/cockroach/certs
      - --http-addr=0.0.0.0:8080
      - --sql-addr=lb:26257
      - --advertise-addr=lb
      - --cache=25%
      - --max-sql-memory=25%
    environment:
      - COCKROACH_CONNECT_TIMEOUT=15s 
    depends_on:
      auth-roach-1:
        condition: service_healthy
      auth-roach-2:
        condition: service_healthy
      auth-roach-3:
        condition: service_healthy
    volumes:
      - $CERTS_DIR:/cockroach/certs:ro
    ports:
      - "26257:26257"
      - "8080:8080"
    networks:
      - auth-net
    healthcheck:
      test: ["CMD-SHELL", "curl -f http://localhost:8080/health?ready=1 || exit 1"]
      interval: 10s
      timeout: 5s
      retries: 5
    deploy:
      replicas: 1
      resources:
        limits:
          memory: 1G

volumes:
  ${VOLUME_PREFIX}-1-data:
  ${VOLUME_PREFIX}-2-data:
  ${VOLUME_PREFIX}-3-data:
EOF

# 6. Update configuration files
log "⚙️ Updating configuration files..."
if [[ "$OSTYPE" == "darwin"* ]]; then
  sed -i '' "s/host:.*/host: \"localhost\"/" internal/configs/dev.yml
  sed -i '' "s/host:.*/host: \"lb\"/" internal/configs/prod.yml
else
  sed -i "s/host:.*/host: \"localhost\"/" internal/configs/dev.yml
  sed -i "s/host:.*/host: \"lb\"/" internal/configs/prod.yml
fi

log "✅ Setup completed successfully!"
echo -e "\nNext steps:"
echo "1. Start the cluster:"
echo "   docker-compose -f $DEPLOY_DIR/docker-compose.yml -f $DEPLOY_DIR/docker-compose.override.yml up -d"
echo ""
echo "2. Initialize database:"
echo "   ./scripts/migrate.sh"