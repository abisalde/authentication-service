# RS256 Migration Guide - Enhanced Security

This guide explains how to migrate from HS256 (HMAC-SHA256) to RS256 (RSA) for JWT signing, which provides significantly better security by eliminating the need to share secrets.

## 🔐 Security Comparison

| Feature | HS256 (Current) | RS256 (Recommended) |
|---------|-----------------|---------------------|
| Key Type | Symmetric (shared secret) | Asymmetric (public/private key pair) |
| Secret Sharing | ⚠️ JWT_SECRET shared across services | ✅ Private key only in auth service |
| Token Creation | Any service with secret can create | ✅ Only auth service can create |
| Token Validation | Any service with secret can validate | ✅ Any service with public key can validate |
| Compromise Impact | ❌ ALL services affected | ✅ Only auth service affected |
| Key Rotation | ❌ Must update ALL services | ✅ Update auth service + propagate public key |
| Security Level | 🟡 MEDIUM | 🟢 HIGH |
| Performance | Faster (~0.5ms) | Slightly slower (~1-2ms) |

## 📋 Migration Steps

### Step 1: Generate RSA Key Pair

```bash
# Run the key generation script
chmod +x scripts/generate-keys.sh
./scripts/generate-keys.sh
```

This creates:
- `secrets/keys/jwt_private.pem` - Private key (4096-bit RSA, keep secure!)
- `secrets/keys/jwt_public.pem` - Public key (can be shared)

**Security Note:** The `secrets/` directory is already in `.gitignore`. NEVER commit private keys!

### Step 2: Update setup.sh

The `scripts/setup.sh` has been enhanced to support both HS256 and RS256:

```bash
# Generate HS256 secret (backward compatible)
JWT_SECRET=$(openssl rand -hex 64)

# Or use RS256 (recommended for production)
JWT_ALGORITHM=RS256
JWT_PRIVATE_KEY_PATH=./secrets/keys/jwt_private.pem
JWT_PUBLIC_KEY_PATH=./secrets/keys/jwt_public.pem
```

### Step 3: Update .env File

Add these variables to `.env`:

```bash
# Option 1: HS256 (Current - backward compatible)
JWT_SECRET=your-hex-secret-from-openssl
JWT_ALGORITHM=HS256

# Option 2: RS256 (Recommended - more secure)
JWT_ALGORITHM=RS256
JWT_PRIVATE_KEY_PATH=./secrets/keys/jwt_private.pem
JWT_PUBLIC_KEY_PATH=./secrets/keys/jwt_public.pem
```

### Step 4: Update Public Key Handler

The `/api/public-key` endpoint automatically detects the algorithm and returns the appropriate key:

**For HS256:**
```json
{
  "publicKey": "your-jwt-secret",
  "algorithm": "HS256",
  "keyId": "default",
  "expiresAt": "2025-12-16T19:10:00Z",
  "version": "1.0"
}
```

**For RS256:**
```json
{
  "publicKey": "-----BEGIN PUBLIC KEY-----\nMIICIjANB...\n-----END PUBLIC KEY-----",
  "algorithm": "RS256",
  "keyId": "default",
  "expiresAt": "2025-12-16T19:10:00Z",
  "version": "1.0"
}
```

### Step 5: Update Token Generation

**Current (HS256):**
```go
// Uses HMAC with shared secret
token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
tokenString, err := token.SignedString([]byte(jwtSecret))
```

**New (RS256):**
```go
// Read private key
privateKeyData, err := os.ReadFile(os.Getenv("JWT_PRIVATE_KEY_PATH"))
if err != nil {
    return "", err
}

privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyData)
if err != nil {
    return "", err
}

// Sign with RSA private key
token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
tokenString, err := token.SignedString(privateKey)
```

### Step 6: Update Token Validation (Microservices)

**Current (HS256):**
```go
secretProvider := config.DefaultSecretProvider()
configMgr := config.NewConfigManager(secretProvider)

// Fetches JWT_SECRET (shared secret)
publicKey, _ := configMgr.GetJWTPublicKey(ctx)

validator := session.NewSessionValidator(session.Config{
    JWTSecret: publicKey,
})
```

**New (RS256):**
```go
secretProvider := config.DefaultSecretProvider()
configMgr := config.NewConfigManager(secretProvider)

// Fetches actual public key from auth service
publicKeyPEM, _ := configMgr.GetJWTPublicKey(ctx)

// Parse RSA public key
publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(publicKeyPEM))
if err != nil {
    return err
}

validator := session.NewSessionValidatorRS256(session.RS256Config{
    PublicKey:     publicKey,
    RedisAddr:     os.Getenv("REDIS_ADDR"),
    RedisPassword: os.Getenv("REDIS_PASSWORD"),
})
```

### Step 7: Rollout Strategy

#### Option A: Blue-Green Deployment (Zero Downtime)

1. **Phase 1: Dual Algorithm Support**
   - Update auth service to support both HS256 and RS256
   - Issue tokens with both signatures (or add algorithm to token)
   - Microservices validate using appropriate algorithm

2. **Phase 2: Transition Period**
   - Start issuing only RS256 tokens
   - Keep HS256 validation for existing tokens
   - Wait for all HS256 tokens to expire (typically 15 min - 1 hour)

3. **Phase 3: Cleanup**
   - Remove HS256 support once all tokens expired
   - Remove JWT_SECRET from all microservices
   - Keep only JWT_ALGORITHM=RS256

#### Option B: Scheduled Maintenance Window

1. **Preparation**
   - Generate RSA keys on all environments
   - Update code to support RS256
   - Test thoroughly in staging

2. **Migration (During Maintenance)**
   - Stop all services
   - Update environment variables
   - Deploy updated services
   - Start services with RS256
   - Force all users to re-login

3. **Verification**
   - Test token creation
   - Test token validation across services
   - Monitor error rates

## 🔑 Key Management Best Practices

### Storage

**Development:**
```bash
# Store in local filesystem (already in .gitignore)
./secrets/keys/jwt_private.pem
./secrets/keys/jwt_public.pem
```

**Production:**
```bash
# Option 1: Kubernetes Secrets
kubectl create secret generic jwt-keys \
  --from-file=private=./secrets/keys/jwt_private.pem \
  --from-file=public=./secrets/keys/jwt_public.pem

# Mount in pod
volumes:
  - name: jwt-keys
    secret:
      secretName: jwt-keys
      defaultMode: 0400
volumeMounts:
  - name: jwt-keys
    mountPath: /run/secrets/jwt
    readOnly: true

# Set env var
JWT_PRIVATE_KEY_PATH=/run/secrets/jwt/private
JWT_PUBLIC_KEY_PATH=/run/secrets/jwt/public
```

**Option 2: HashiCorp Vault**
```bash
# Store keys in Vault
vault kv put secret/jwt-keys \
  private=@./secrets/keys/jwt_private.pem \
  public=@./secrets/keys/jwt_public.pem

# Retrieve in app
vault kv get -field=private secret/jwt-keys > /tmp/jwt_private.pem
```

**Option 3: AWS Secrets Manager**
```bash
# Store private key
aws secretsmanager create-secret \
  --name jwt-private-key \
  --secret-string file://./secrets/keys/jwt_private.pem

# Retrieve in app
aws secretsmanager get-secret-value \
  --secret-id jwt-private-key \
  --query SecretString \
  --output text > /tmp/jwt_private.pem
```

### Rotation

**Recommended Schedule:** Every 90 days (quarterly)

**Rotation Procedure:**

1. Generate new key pair:
   ```bash
   ./scripts/generate-keys.sh
   mv secrets/keys/jwt_private.pem secrets/keys/jwt_private_new.pem
   mv secrets/keys/jwt_public.pem secrets/keys/jwt_public_new.pem
   ```

2. Deploy new keys alongside old keys:
   ```bash
   # Auth service uses new private key for signing
   JWT_PRIVATE_KEY_PATH=./secrets/keys/jwt_private_new.pem
   
   # Keep old public key for validation (during transition)
   JWT_PUBLIC_KEY_PATH_OLD=./secrets/keys/jwt_public.pem
   JWT_PUBLIC_KEY_PATH_NEW=./secrets/keys/jwt_public_new.pem
   ```

3. Issue new tokens with new key:
   - New logins get tokens signed with new key
   - Existing tokens still valid with old key

4. Wait for old tokens to expire (e.g., 1 hour)

5. Remove old keys:
   ```bash
   rm secrets/keys/jwt_private.pem
   rm secrets/keys/jwt_public.pem
   mv secrets/keys/jwt_private_new.pem secrets/keys/jwt_private.pem
   mv secrets/keys/jwt_public_new.pem secrets/keys/jwt_public.pem
   ```

### Backup

```bash
# Backup private key (encrypt before storing)
gpg --symmetric --cipher-algo AES256 secrets/keys/jwt_private.pem
# Store jwt_private.pem.gpg securely (e.g., AWS S3 with encryption)

# Restore
gpg --decrypt jwt_private.pem.gpg > secrets/keys/jwt_private.pem
chmod 600 secrets/keys/jwt_private.pem
```

## 🧪 Testing

### Test RS256 Token Generation

```bash
# Generate test token
go run examples/rs256-test/generate_token.go

# Verify with public key
go run examples/rs256-test/verify_token.go <token>
```

### Test Public Key Endpoint

```bash
# Fetch public key
curl http://localhost:8080/api/public-key

# Should return RS256 public key in PEM format
```

### Validate Token in Microservice

```bash
# Start auth service with RS256
JWT_ALGORITHM=RS256 go run cmd/server/main.go

# Start microservice
cd examples/secure-microservice
go run main.go

# Make authenticated request
curl -H "Authorization: Bearer <token>" http://localhost:8081/api/profile
```

## 📊 Performance Impact

| Operation | HS256 | RS256 | Impact |
|-----------|-------|-------|--------|
| Token Generation (Auth Service) | ~0.3ms | ~1.2ms | +0.9ms |
| Token Validation (Microservices) | ~0.5ms | ~1.8ms | +1.3ms |
| Public Key Fetch (Cached) | <1ms | <1ms | None |
| End-to-End Request | ~50ms | ~52ms | +2ms (4%) |

**Conclusion:** The ~2ms overhead is negligible compared to the significant security benefits.

## 🔒 Security Benefits

### With HS256 (Current)
```
Attacker compromises Microservice A
↓
Gets JWT_SECRET
↓
Can create valid tokens
↓
Can impersonate ANY user
↓
All services compromised
```

### With RS256 (Recommended)
```
Attacker compromises Microservice A
↓
Gets public key only
↓
CANNOT create valid tokens (no private key)
↓
Can only validate tokens
↓
Only Microservice A affected
↓
Contain breach + rotate keys
```

## 🚨 Troubleshooting

### Token Validation Fails

**Error:** `crypto/rsa: verification error`

**Solutions:**
- Ensure public key matches private key used for signing
- Verify algorithm matches (RS256 vs HS256)
- Check key file permissions (must be readable)

### Public Key Endpoint Returns HS256

**Issue:** Still using JWT_SECRET instead of RSA keys

**Solutions:**
- Verify `JWT_ALGORITHM=RS256` is set in .env
- Ensure `JWT_PRIVATE_KEY_PATH` and `JWT_PUBLIC_KEY_PATH` are set
- Restart auth service after environment changes

### Private Key Permission Denied

**Error:** `open jwt_private.pem: permission denied`

**Solution:**
```bash
chmod 600 secrets/keys/jwt_private.pem
```

## 📚 Additional Resources

- [RFC 7519 - JSON Web Token (JWT)](https://datatracker.ietf.org/doc/html/rfc7519)
- [RFC 7518 - JSON Web Algorithms (JWA)](https://datatracker.ietf.org/doc/html/rfc7518)
- [OWASP JWT Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/JSON_Web_Token_for_Java_Cheat_Sheet.html)
- [Industry Best Practices: Google Cloud JWT](https://cloud.google.com/endpoints/docs/openapi/authenticating-users-jwt)

## 🎯 Summary

**Current State (HS256):**
- ⚠️ Shared secret across all services
- ⚠️ Any service can create tokens
- ⚠️ High risk if one service compromised

**Recommended State (RS256):**
- ✅ Private key only in auth service
- ✅ Only auth service can create tokens
- ✅ Limited impact if microservice compromised
- ✅ Industry standard for distributed systems
- ✅ Follows patterns from Google, AWS, Netflix

**Next Steps:**
1. Run `./scripts/generate-keys.sh`
2. Update .env with RS256 configuration
3. Update token generation code
4. Update validation code in microservices
5. Deploy with zero-downtime strategy
6. Monitor and verify
7. Remove HS256 support after transition
