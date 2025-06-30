#!/bin/bash

# Configuration
VOLUME_PREFIX="auth-roach"
NETWORK_NAME="auth-net"
CERTS_DIR="./cockroach/certs"
NODE_COUNT=3

# 1. Create Docker resources
echo "Creating Docker volumes and network..."
for i in $(seq 1 $NODE_COUNT); do
    docker volume create ${VOLUME_PREFIX}-${i}-data
done
docker network create -d bridge $NETWORK_NAME

# 2. Generate certificates
echo "Generating certificates..."
mkdir -p $CERTS_DIR

# CA Certificate
docker run --rm -v $(pwd)/$CERTS_DIR:/certs cockroachdb/cockroach \
    cert create-ca --certs-dir=/certs --ca-key=/certs/ca.key

# Node Certificates (for all nodes and load balancer)
docker run --rm -v $(pwd)/$CERTS_DIR:/certs cockroachdb/cockroach \
    cert create-node localhost auth-roach-1 auth-roach-2 auth-roach-3 lb \
    --certs-dir=/certs --ca-key=/certs/ca.key

# Client Certificates
docker run --rm -v $(pwd)/$CERTS_DIR:/certs cockroachdb/cockroach \
    cert create-client root --certs-dir=/certs --ca-key=/certs/ca.key

docker run --rm -v $(pwd)/$CERTS_DIR:/certs cockroachdb/cockroach \
    cert create-client appuser --certs-dir=/certs --ca-key=/certs/ca.key

# 3. Set proper permissions
echo "Setting file permissions..."
chmod 600 $CERTS_DIR/*.key
chmod 644 $CERTS_DIR/*.crt

# 4. Create database initialization script
echo "Creating initialization SQL..."
cat > $CERTS_DIR/init.sql <<EOF
CREATE DATABASE IF NOT EXISTS authserviceprod;
CREATE USER IF NOT EXISTS appuser;
GRANT ALL ON DATABASE authserviceprod TO appuser;
EOF

# 5. Generate Docker Compose override for development
echo "Generating development compose file..."
cat > deployments/docker-compose.override.yml <<EOF
version: '3.8'

services:
  auth-roach-1:
    extends:
      file: docker-compose.yml
      service: cockroachdb
    command: start --join=auth-roach-1,auth-roach-2,auth-roach-3 --certs-dir=/cockroach/certs
    volumes:
      - ${VOLUME_PREFIX}-1-data:/cockroach/cockroach-data
      - ./cockroach/certs:/cockroach/certs
      - ./cockroach/certs/init.sql:/docker-entrypoint-initdb.d/init.sql

  auth-roach-2:
    extends:
      file: docker-compose.yml
      service: cockroachdb
    command: start --join=auth-roach-1,auth-roach-2,auth-roach-3 --certs-dir=/cockroach/certs
    volumes:
      - ${VOLUME_PREFIX}-2-data:/cockroach/cockroach-data
      - ./cockroach/certs:/cockroach/certs

  auth-roach-3:
    extends:
      file: docker-compose.yml
      service: cockroachdb
    command: start --join=auth-roach-1,auth-roach-2,auth-roach-3 --certs-dir=/cockroach/certs
    volumes:
      - ${VOLUME_PREFIX}-3-data:/cockroach/cockroach-data
      - ./cockroach/certs:/cockroach/certs

  lb:
    image: cockroachdb/cockroach
    command: start --join=auth-roach-1,auth-roach-2,auth-roach-3 --certs-dir=/cockroach/certs --http-addr=lb:8080 --sql-addr=lb:26257
    depends_on:
      - auth-roach-1
      - auth-roach-2
      - auth-roach-3
    volumes:
      - ./cockroach/certs:/cockroach/certs
    ports:
      - "26257:26257"
      - "8080:8080"

volumes:
  ${VOLUME_PREFIX}-1-data:
  ${VOLUME_PREFIX}-2-data:
  ${VOLUME_PREFIX}-3-data:
EOF

# 6. Update configuration files
echo "Updating configuration files..."
# For development (pointing to lb)
if [[ "$OSTYPE" == "darwin"* ]]; then
  sed -i '' 's/host:.*/host: "localhost"/' internal/configs/dev.yml
else
  sed -i 's/host:.*/host: "localhost"/' internal/configs/dev.yml
fi

# For production (pointing to lb)
sed -i '' 's/host:.*/host: "lb"/' internal/configs/prod.yml

# 7. Verify structure
echo -e "\nCreated resources:"
echo "Certificate files:"
ls -l $CERTS_DIR
echo -e "\nDocker volumes:"
docker volume ls | grep $VOLUME_PREFIX
echo -e "\nDocker network:"
docker network inspect $NETWORK_NAME

echo -e "\nSetup complete for 3-node Cockroach cluster!"

echo -e "✅ Next steps:\n"

echo "1. Start the cluster: ▶️"
echo "   docker-compose -f deployments/docker-compose.yml -f deployments/docker-compose.override.yml up -d"
echo ""

echo "2. Verify cluster status: ❤️‍🔥"
echo "   docker exec -it auth-roach-1 ./cockroach node status --certs-dir=/cockroach/certs"
echo ""

echo "3. Initialize cluster: ./scripts/migrate.sh"