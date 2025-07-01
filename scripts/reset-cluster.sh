#!/bin/bash
set -eo pipefail

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DEPLOY_DIR="$PROJECT_ROOT/deployments"
CERTS_DIR="$PROJECT_ROOT/scripts/cockroach/certs"
VOLUME_PREFIX="auth-roach"
NETWORK_NAME="auth-net"
NODE_COUNT=3

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

cleanup() {
  log "🧹 Cleaning up existing Docker resources..."
  
  # Stop and remove containers if compose files exist
  if [[ -f "$DEPLOY_DIR/docker-compose.yml" ]]; then
    docker-compose -f "$DEPLOY_DIR/docker-compose.yml" down -v 2>/dev/null || true
  fi
  
  # Remove specific volumes
  for i in $(seq 1 $NODE_COUNT); do
    docker volume rm "${VOLUME_PREFIX}-${i}-data" 2>/dev/null || true
  done
  
  # Remove network
  docker network rm "$NETWORK_NAME" 2>/dev/null || true
  
  # Clean up files
  rm -rf "$CERTS_DIR" 2>/dev/null || true
  rm -f "$DEPLOY_DIR/docker-compose.override.yml" 2>/dev/null || true
  
  # Ensure directories exist
  mkdir -p "$DEPLOY_DIR" "$CERTS_DIR"
}

verify_environment() {
  log "🔍 Verifying environment..."
  if ! command -v docker &> /dev/null; then
    log "❌ Docker is not installed"
    exit 1
  fi
  
  if ! docker info &> /dev/null; then
    log "❌ Docker daemon is not running"
    exit 1
  fi
  
  if [[ ! -f "$PROJECT_ROOT/go.mod" ]]; then
    log "❌ Not in project root directory"
    exit 1
  fi
}

log "🔄 Starting complete cluster reset"
verify_environment
cleanup


log "✅ Cleanup complete. Now run:"
echo ""
echo "./scripts/setup.sh"