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
API_URL="api.abisalde.dev"


# Port Configuration
MYSQL_DEV_HOST_PORT=3388
MYSQL_DEV_PROD_HOST_PORT=3306
MYSQL_DEV_CONTAINER_PORT=3306
REDIS_DEV_HOST_PORT=6388
REDIS_DEV_CONTAINER_PORT=6379
APP_DEV_HOST_PORT=8080
APP_DEV_CONTAINER_PORT=8080
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587

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
  local base=$(< /dev/urandom tr -dc 'a-z' | head -c 10)
  local upper=$(< /dev/urandom tr -dc 'A-Z' | head -c 3)
  local number=$(< /dev/urandom tr -dc '0-9' | head -c 3)
  local special=$(< /dev/urandom tr -dc '!$#[]*' | head -c 3)
  
  # Shuffle the components
  echo -n "$base$upper$number$special" | fold -w1 | shuf | tr -d '\n'
  echo
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

# Write secrets without trailing newline
echo -n "$PROD_DB_PASSWORD" > "$SECRETS_DIR/.prod_db_password"
echo -n "$DEV_DB_PASSWORD" > "$SECRETS_DIR/.dev_db_password"
echo -n "$REDIS_PASSWORD" > "$SECRETS_DIR/.redis_password"

# Secure permissions
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
SMTP_HOST=$SMTP_HOST
SMTP_PORT=$SMTP_PORT
EOF


# Create init-db.sql for initial database setup
cat > "$DEPLOY_DIR/init-db.sql" <<EOF

CREATE DATABASE IF NOT EXISTS $PROD_DB_NAME CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
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
  mysql-prod:
    image: mysql:lts
    container_name: mysql-prod
    environment:
      MYSQL_ROOT_PASSWORD_FILE: /run/secrets/prod_db_password
      MYSQL_PASSWORD_FILE: /run/secrets/prod_db_password
      MYSQL_USER: "$DB_USER"
      MYSQL_DATABASE: "$PROD_DB_NAME"
    secrets:
      - prod_db_password
    volumes:
      - mysql_prod_data:/var/lib/mysql
      - ./init-db.sql:/docker-entrypoint-initdb.d/init.sql
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost", "-u", "root", "-p=$PROD_DB_PASSWORD"]
      interval: 5s
      timeout: 5s
      retries: 10
      start_period: 60s
    networks:
      - auth-prod-network
    restart: on-failure

  redis:
    image: redis/redis-stack:7.2.0-v17
    container_name: redis
    environment:
      REDIS_ARGS: "--save 1200 32" 
      REDIS_PASSWORD_FILE: /run/secrets/redis_password
    secrets:
      - redis_password
    command: ['/redis-entrypoint.sh']
    volumes:
      - ../scripts/start-redis.sh:/redis-entrypoint.sh:ro
      - redis_data:/data
    healthcheck:
      test:
        [
          'CMD',
          'redis-cli',
          '-a',
          '$$(cat /run/secrets/redis_password)',
          'ping',
        ]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - auth-prod-network
    restart: on-failure
  
  traefik:
    image: traefik:v3.4
    container_name: traefik
    command:
      - "--providers.docker=true"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.email=princeabisal@gmail.com"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
      - "--certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=web"
      - "--certificatesresolvers.letsencrypt.acme.tlschallenge=true"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./letsencrypt:/letsencrypt
      - /var/run/docker.sock:/var/run/docker.sock:ro
    healthcheck:
      test: ["CMD", "wget", "--spider", "http://localhost:8080/ping"]
      interval: 10s
      timeout: 5s
      retries: 3
      start_period: 30s
    networks:
      - auth-prod-network
    restart: always


  auth-service:
    build:
      context: ../
      dockerfile: Dockerfile
    depends_on:
      mysql-prod:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      APP_ENV: "production"
      ENVIRONMENT: "production"
      DB_HOST: mysql-prod
      DB_PORT: "$MYSQL_DEV_CONTAINER_PORT"
      DB_USER: "$DB_USER"
      DB_NAME: "$PROD_DB_NAME"
      DB_PASSWORD_FILE: /run/secrets/prod_db_password
      REDIS_PASSWORD_FILE: /run/secrets/redis_password
      REDIS_ARGS: --save 1200 32
      DB_SSL_MODE: require
      REDIS_URL: "redis://default:$REDIS_PASSWORD@redis:$REDIS_DEV_CONTAINER_PORT"
    secrets:
      - prod_db_password
      - redis_password
    volumes:
      - ../internal/configs:/home/appuser/internal/configs:ro
      - ../.env:/home/appuser/.env:ro
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.auth-service.rule=Host(\`$API_URL\`)"
      - "traefik.http.routers.auth-service.entrypoints=websecure"
      - "traefik.http.routers.auth-service.tls.certresolver=letsencrypt"
      - "traefik.http.services.auth-service.loadbalancer.server.port=8080"
      - "traefik.http.middlewares.redirect-to-https.redirectscheme.scheme=https"
      - "traefik.http.routers.auth-service-http.middlewares=redirect-to-https"
      - "traefik.http.routers.auth-service-http.entrypoints=web"
      - "traefik.http.services.auth-service.loadbalancer.passhostheader=true"
    networks:
      - auth-prod-network
    restart: on-failure

volumes:
  mysql_prod_data:
  redis_data:

secrets:
  prod_db_password:
    file: ../secrets/.prod_db_password
  redis_password:
    file: ../secrets/.redis_password

networks:
  auth-prod-network:
    name: auth-prod-network
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
  -e "s|^\([[:space:]]*mysql_dsn:\).*|\1 \"appuser:\${PROD_DB_PASSWORD:-prod_db_password}@tcp(mysql-prod:${MYSQL_DEV_PROD_HOST_PORT})/${PROD_DB_NAME}?parseTime=true\"|" \
  -e "s|^\([[:space:]]*host:\).*|\1 \"mysql-prod\"|" \
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