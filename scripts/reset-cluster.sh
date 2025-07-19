#!/bin/bash
set -eo pipefail

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DEPLOY_DIR="$PROJECT_ROOT/deployments"

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

log "🔴 Resetting cluster..."
docker-compose -f "$DEPLOY_DIR/docker-compose.yml" -f down -v


log "✅ Cluster stopped and volumes removed"

