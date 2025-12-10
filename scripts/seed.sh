#!/bin/bash

set -e

echo "🌱 Starting user seed process..."

# Check if Docker containers are running
if ! docker ps | grep -q "mysql-dev"; then
    echo "❌ MySQL container is not running. Please start Docker containers first:"
    echo "   cd deployments && docker-compose up -d"
    exit 1
fi

# Navigate to the script directory
cd "$(dirname "$0")"

# Build and run the seed script inside the Docker container
echo "📦 Running seed script inside Docker container..."
cd ..
docker exec -i deployments-auth-service-1 go run /app/scripts/seed_users.go

echo "✨ Seed process completed!"
