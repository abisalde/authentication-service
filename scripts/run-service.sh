#!/bin/bash


# Configuration
export LC_ALL=C
PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# Default values
PORT=${PORT:-8080}
ENVIRONMENT=${ENVIRONMENT:-development}
BUILD_DIR="$PROJECT_DIR/bin"
APP_NAME="authentication-service"
USE_DOCKER=${USE_DOCKER:-false}

# Load .env file if it exists
if [ -f "${PROJECT_ROOT}/.env" ]; then
  echo "🌱 Loading environment variables from .env"
  export $(grep -v '^#' ${PROJECT_ROOT}/.env | xargs)
fi



# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        -p|--port) PORT="$2"; shift 2 ;;
        -e|--env) ENVIRONMENT="$2"; shift 2 ;;
        -d|--docker) USE_DOCKER=true; shift ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done




if [ "$USE_DOCKER" = true ]; then
    echo "🐳 Running with Docker Compose..."
    docker-compose up --build
else
    echo "📦 Building locally..."
# Create build directory if it doesn't exist
    mkdir -p "$BUILD_DIR"
    
    echo "📦 Building $APP_NAME (env: $ENVIRONMENT, port: $PORT)"
    echo "----------------------------------------"

    echo "🔨 Building application..."
    cd "${PROJECT_ROOT}" && go build -o "${BUILD_DIR}/${APP_NAME}" ./cmd/server/main.go
    
    if [ $? -ne 0 ]; then
        echo "❌ Build failed"
        exit 1
    fi
    
    chmod +x "${BUILD_DIR}/${APP_NAME}"
    
    echo "🚀 Starting ${APP_NAME} on port ${PORT}..."
    export PORT=$PORT
    export ENVIRONMENT=$ENVIRONMENT
    "${BUILD_DIR}/${APP_NAME}"
fi