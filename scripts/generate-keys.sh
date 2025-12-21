#!/bin/bash
set -eo pipefail

# Script to generate RSA key pairs for JWT signing (RS256)
# This is more secure than HS256 as private key never leaves auth service

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
KEYS_DIR="$PROJECT_ROOT/secrets/keys"

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Create keys directory
mkdir -p "$KEYS_DIR"
chmod 700 "$KEYS_DIR"

log "🔐 Generating RSA key pair for JWT signing..."

# Generate private key (4096 bits for enhanced security)
openssl genpkey -algorithm RSA -out "$KEYS_DIR/jwt_private.pem" -pkeyopt rsa_keygen_bits:4096

# Extract public key from private key
openssl rsa -pubout -in "$KEYS_DIR/jwt_private.pem" -out "$KEYS_DIR/jwt_public.pem"

# Secure permissions
chmod 600 "$KEYS_DIR/jwt_private.pem"  # Private key - owner read/write only
chmod 644 "$KEYS_DIR/jwt_public.pem"   # Public key - everyone can read

log "✅ RSA key pair generated successfully!"
log "   Private key: $KEYS_DIR/jwt_private.pem (keep secure!)"
log "   Public key:  $KEYS_DIR/jwt_public.pem (can be distributed)"

echo ""
echo "Next steps:"
echo "1. Update .env to use RS256:"
echo "   JWT_ALGORITHM=RS256"
echo "   JWT_PRIVATE_KEY_PATH=./secrets/keys/jwt_private.pem"
echo "   JWT_PUBLIC_KEY_PATH=./secrets/keys/jwt_public.pem"
echo ""
echo "2. For HS256 (current method), update JWT_SECRET:"
echo "   JWT_SECRET=\$(openssl rand -hex 64)"
echo ""
echo "3. Backup private key securely (never commit to git!)"
