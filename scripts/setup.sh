#!/bin/bash
set -eo pipefail

# Configuration
export LC_ALL=C
PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DEPLOY_DIR="$PROJECT_ROOT/deployments"
SECRETS_DIR="$PROJECT_ROOT/secrets"
CONFIG_ENV="$PROJECT_ROOT/internal/configs"
DB_USER="appuser"
DEV_DB_NAME="authservicelocal"
PROD_DB_NAME="authserviceprod"
REDIS_PASSWORD=$(openssl rand -hex 32)
JWT_SECRET=$(openssl rand -hex 64)


# Port Configuration
MYSQL_DEV_HOST_PORT=3388
MYSQL_DEV_PROD_HOST_PORT=3308
MYSQL_DEV_CONTAINER_PORT=3306
REDIS_DEV_HOST_PORT=6388
REDIS_DEV_CONTAINER_PORT=6379
APP_DEV_HOST_PORT=8080
APP_DEV_CONTAINER_PORT=8080

# --------------------------
# Functions
# --------------------------

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

generate_dev_password() {
  < /dev/urandom tr -dc 'A-Za-z0-9!#$%&()*+,-./;<=>?[]^_{|}~' | head -c 32 2>/dev/null || true
}

generate_prod_password() {
  openssl rand -hex 32
}

verify_directories() {
  log "📂 Verifying directory structure..."
  mkdir -p "$SECRETS_DIR" || {
    log "❌ Failed to create secrets directory"
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
log "🚀 Setting up MYSQL + Redis environment"

# Create directories
mkdir -p "$SECRETS_DIR"
chmod 700 "$SECRETS_DIR"

# Generate passwords
PROD_DB_PASSWORD=$(generate_prod_password)
DEV_DB_PASSWORD=$(generate_dev_password)
echo "$PROD_DB_PASSWORD" > "$SECRETS_DIR/.prod_db_password"
echo "$DEV_DB_PASSWORD" > "$SECRETS_DIR/.dev_db_password"
echo "$REDIS_PASSWORD" > "$SECRETS_DIR/.redis_password"
chmod 600 "$SECRETS_DIR"/.*password

# Create .env file for Docker Compose
log "🚀 Starts to create ENV"
cat > "$PROJECT_ROOT/.env" <<EOF
REDIS_PASSWORD=$REDIS_PASSWORD
DEV_DB_PASSWORD=$DEV_DB_PASSWORD
PROD_DB_PASSWORD=$PROD_DB_PASSWORD
MYSQL_DEV_HOST_PORT=$MYSQL_DEV_HOST_PORT
MYSQL_DEV_PROD_HOST_PORT=$MYSQL_DEV_PROD_HOST_PORT
REDIS_DEV_HOST_PORT=$REDIS_DEV_HOST_PORT
APP_DEV_HOST_PORT=$APP_DEV_HOST_PORT
PORT=$APP_DEV_HOST_PORT
APP_ENV=development
JWT_SECRET=$JWT_SECRET
EOF


# Create init-db.sql for initial database setup
cat > "$SECRETS_DIR/init-db.sql" <<EOF
CREATE DATABASE IF NOT EXISTS $PROD_DB_NAME;
CREATE USER IF NOT EXISTS '$DB_USER'@'%' IDENTIFIED BY '$PROD_DB_PASSWORD';
GRANT ALL PRIVILEGES ON $PROD_DB_NAME.* TO '$DB_USER'@'%';
FLUSH PRIVILEGES;

CREATE DATABASE IF NOT EXISTS $DEV_DB_NAME;
CREATE USER IF NOT EXISTS '$DB_USER'@'%' IDENTIFIED BY '$DEV_DB_PASSWORD';
GRANT ALL PRIVILEGES ON $DEV_DB_NAME.* TO '$DB_USER'@'%';
FLUSH PRIVILEGES;
EOF


# 5. Generate Docker Compose override
log "🐳 Generating Docker Compose override prod..."
mkdir -p "$DEPLOY_DIR"
cat > "$DEPLOY_DIR/docker-compose.prod.yml" <<EOF
services:
  mysql:
    image: mysql:lts
    container_name: mysql-prod
    environment:
      MYSQL_ROOT_PASSWORD: $PROD_DB_PASSWORD
      MYSQL_USER: $DB_USER
      MYSQL_PASSWORD: $PROD_DB_PASSWORD
      MYSQL_DATABASE: $PROD_DB_NAME
    ports:
      - "$MYSQL_DEV_PROD_HOST_PORT:$MYSQL_DEV_CONTAINER_PORT"
    volumes:
      - mysql_data:/var/lib/mysql
      - $SECRETS_DIR/init-db.sql:/docker-entrypoint-initdb.d/init.sql
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "$PROD_DB_NAME"]
      interval: 5s
      timeout: 5s
      retries: 10
    networks:
      - auth-net

  redis:
    image: redis/redis-stack:7.2.0-v17
    command: ['/redis-entrypoint.sh']
    container_name: redis
    environment:
      - REDIS_ARGS=--save 1200 32
      - REDIS_PASSWORD=$REDIS_PASSWORD
    volumes:
      - ../scripts/start-redis.sh:/redis-entrypoint.sh:ro
      - redis_data:/data
    healthcheck:
      test:
        [
          'CMD',
          'redis-cli',
          '-a',
          $REDIS_PASSWORD,
          'ping',
        ]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - auth-net
    ports:
      - "$APP_DEV_HOST_PORT:$APP_DEV_CONTAINER_PORT"
    deploy:
      replicas: 1
      restart_policy:
        condition: on-failure

  auth-service:
    build:
      context: ../
      dockerfile: Dockerfile
    depends_on:
      mysql:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      DB_HOST: mysql
      DB_PORT: $MYSQL_DEV_PROD_HOST_PORT
      DB_USER: $DB_USER
      DB_PASSWORD: $PROD_DB_PASSWORD
      DB_NAME: $PROD_DB_NAME
      DB_SSL_MODE: require
      REDIS_URL: "redis://default:$REDIS_PASSWORD@redis:$REDIS_DEV_CONTAINER_PORT"

volumes:
  mysql_data:
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

# Ensure variables are set
: ${MYSQL_DEV_HOST_PORT:=3388}
: ${REDIS_DEV_HOST_PORT:=6388}
: ${MYSQL_DEV_PROD_HOST_PORT:=3308}
: ${REDIS_DEV_CONTAINER_PORT:=6379}
: ${DEV_DB_NAME:=authservicelocal}
: ${PROD_DB_NAME:=authserviceprod}
: ${DB_USER:=appuser}

# Update development config
sed "${SED_INPLACE[@]}" \
  -e "s|^\([[:space:]]*mysql_dsn:\).*|\1 \"appuser:\${DEV_DB_PASSWORD:-dev_db_password}@tcp(mysql-dev:${MYSQL_DEV_HOST_PORT})/${DEV_DB_NAME}?parseTime=true\"|" \
  -e "s|^\([[:space:]]*host:\).*|\1 \"mysql-dev\"|" \
  -e "s|^\([[:space:]]*port:\).*|\1 ${MYSQL_DEV_HOST_PORT}|" \
  -e "s|^\([[:space:]]*user:\).*|\1 \"${DB_USER}\"|" \
  -e "s|^\([[:space:]]*password:\).*|\1 \${DEV_DB_PASSWORD:-dev_db_password}|" \
  -e "s|^\([[:space:]]*dbname:\).*|\1 \"${DEV_DB_NAME}\"|" \
  -e "s|^\([[:space:]]*sslmode:\).*|\1 disable|" \
  -e "s|^\([[:space:]]*redis_addr:\).*|\1 \"localhost:${REDIS_DEV_HOST_PORT}\"|" \
  -e "s|^\([[:space:]]*redis_password:\).*|\1 \"\${REDIS_PASSWORD:-redis_password}\"|" \
  "$CONFIG_ENV/dev.yml"

# Update production config  
sed "${SED_INPLACE[@]}" \
  -e "s|^\([[:space:]]*mysql_dsn:\).*|\1 \"appuser:\${PROD_DB_PASSWORD:-prod_db_password}@tcp(mysql:${MYSQL_DEV_PROD_HOST_PORT})/${PROD_DB_NAME}?parseTime=true\"|" \
  -e "s|^\([[:space:]]*host:\).*|\1 \"mysql\"|" \
  -e "s|^\([[:space:]]*port:\).*|\1 ${MYSQL_DEV_PROD_HOST_PORT}|" \
  -e "s|^\([[:space:]]*user:\).*|\1 \"${DB_USER}\"|" \
  -e "s|^\([[:space:]]*password:\).*|\1 \${PROD_DB_PASSWORD:-prod_db_password}|" \
  -e "s|^\([[:space:]]*dbname:\).*|\1 \"${PROD_DB_NAME}\"|" \
  -e "s|^\([[:space:]]*sslmode:\).*|\1 require|" \
  -e "s|^\([[:space:]]*redis_addr:\).*|\1 \"redis:${REDIS_DEV_CONTAINER_PORT}\"|" \
  -e "s|^\([[:space:]]*redis_password:\).*|\1 \"\${REDIS_PASSWORD:-redis_password}\"|" \
  "$CONFIG_ENV/prod.yml"

log "✅ Configuration files updated"

log "✅ Setup completed successfully!"


echo -e "\nNext steps:"
echo ""
echo "1. 📡 Initialize containers:"
echo "   ./scripts/init.sh"
echo ""
echo "2. 🛠️ Initialize database:"
echo "   ./scripts/migrate.sh"
echo ""
echo "3. 🚀 Run the application:"
echo "   The service will be available at http://localhost:$APP_DEV_HOST_PORT"