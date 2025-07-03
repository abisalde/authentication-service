#!/bin/bash
set -eo pipefail

# Configuration
export LC_ALL=C
PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DEPLOY_DIR="$PROJECT_ROOT/deployments"
CERTS_DIR="$PROJECT_ROOT/scripts/cockroach/certs"
CONFIG_ENV="$PROJECT_ROOT/internal/configs"
DEV_DB_USER="root"
DEV_DB_NAME="authservicelocal"
PROD_DB_USER="appuser"
PROD_DB_NAME="authserviceprod"
REDIS_PASSWORD=$(openssl rand -hex 32)

# --------------------------
# Functions
# --------------------------

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

generate_dev_password() {
  < /dev/urandom tr -dc 'A-Za-z0-9!#$%&()*+,-./:;<=>?@[\]^_{|}~' | head -c 32 2>/dev/null || true
}

generate_prod_password() {
  openssl rand -hex 32
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

# Verify execution context
verify_project_root
verify_directories


# Store password securely
log "🚀 Setting up PostgreSQL + Redis environment"

# Create directories
mkdir -p "$CERTS_DIR"
chmod 700 "$CERTS_DIR"

# Generate passwords
PROD_DB_PASSWORD=$(generate_prod_password)
DEV_DB_PASSWORD=$(generate_dev_password)
echo "$PROD_DB_PASSWORD" > "$CERTS_DIR/.prod_db_password"
echo "$DEV_DB_PASSWORD" > "$CERTS_DIR/.dev_db_password"
echo "$REDIS_PASSWORD" > "$CERTS_DIR/.redis_password"
chmod 600 "$CERTS_DIR"/.*password

# Create .env file for Docker Compose
log "🚀 Starts to create ENV"
cat > "$PROJECT_ROOT/.env" <<EOF
REDIS_PASSWORD=$REDIS_PASSWORD
DB_PASSWORD=$DEV_DB_PASSWORD
EOF


# Create init-db.sql for initial database setup
cat > "$CERTS_DIR/init-db.sql" <<EOF
CREATE USER IF NOT EXISTS $PROD_DB_USER WITH PASSWORD '$PROD_DB_PASSWORD';
CREATE DATABASE $PROD_DB_NAME;
GRANT ALL PRIVILEGES ON DATABASE $PROD_DB_NAME TO $PROD_DB_USER;

CREATE USER IF NOT EXISTS $DEV_DB_USER WITH PASSWORD '$DEV_DB_PASSWORD';
CREATE DATABASE $DEV_DB_NAME;
GRANT ALL PRIVILEGES ON DATABASE $DEV_DB_NAME TO $DEV_DB_USER;
EOF


# 5. Generate Docker Compose override
log "🐳 Generating Docker Compose override..."
mkdir -p "$DEPLOY_DIR"
cat > "$DEPLOY_DIR/docker-compose.override.yml" <<EOF
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: $PROD_DB_PASSWORD
      POSTGRES_DB: $PROD_DB_NAME
    volumes:
      - $PROJECT_ROOT/scripts/init-db.sql:/docker-entrypoint-initdb.d/init.sql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d $PROD_DB_NAME"]
      interval: 5s
      timeout: 5s
      retries: 10
    networks:
      - auth-net

  redis:
    image: redis/redis-stack:7.2.0-v17
    command: redis-server --requirepass $REDIS_PASSWORD
    environment:
      - REDIS_ARGS=--save 1200 32
      - REDIS_PASSWORD=$REDIS_PASSWORD
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - auth-net
    ports:
      - 6379:6379
    deploy:
      replicas: 1
      restart_policy:
        condition: on-failure

  auth-service:
    build:
      context: ../
      dockerfile: Dockerfile
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      DB_HOST: postgres
      DB_PORT: 5432
      DB_USER: $PROD_DB_USER
      DB_PASSWORD: $PROD_DB_PASSWORD
      DB_NAME: $PROD_DB_NAME
      DB_SSL_MODE: require
      REDIS_URL: "redis://default:$REDIS_PASSWORD@redis:6379"

volumes:
  postgres_data:
  redis_data:

networks:
  auth-net:
    driver: bridge
EOF

# 6. Update configuration files
log "⚙️ Updating configuration files..."

# Platform detection
SED_INPLACE=()
if [[ "$OSTYPE" == "darwin"* ]]; then
  SED_INPLACE=(-i '')
else
  SED_INPLACE=(-i)
fi

# Update development config
sed "${SED_INPLACE[@]}" \
  -e "s|host:.*|host: \"localhost\"|" \
  -e "s|port:.*|port: 5432|" \
  -e "s|user:.*|user: \"$DEV_DB_USER\"|" \
  -e "s|password:.*|password: \${DEV_DB_PASSWORD:-dev_db_password}|" \
  -e "s|dbname:.*|dbname: \"$DEV_DB_NAME\"|" \
  -e "s|sslmode:.*|sslmode: disable|" \
  "$CONFIG_ENV/dev.yml"

# Update production config  
sed "${SED_INPLACE[@]}" \
  -e "s|host:.*|host: \"postgres\"|" \
  -e "s|port:.*|port: 5432|" \
  -e "s|user:.*|user: \"$PROD_DB_USER\"|" \
  -e "s|password:.*|password: \${PROD_DB_PASSWORD:-prod_db_password}|" \
  -e "s|dbname:.*|dbname: \"$PROD_DB_NAME\"|" \
  -e "s|sslmode:.*|sslmode: require|" \
  "$CONFIG_ENV/prod.yml"

log "✅ Configuration files updated"

log "✅ Setup completed successfully!"


echo -e "\nNext steps:"
echo ""
echo "1. 📡 Initialize docker-compose"
echo "   ./scripts/init.sh"
echo ""
echo "1. Initialize database:"
echo "   ./scripts/migrate.sh"