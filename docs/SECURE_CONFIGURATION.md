# Secure Configuration Management

This guide addresses the security concerns around sharing JWT secrets and Redis passwords across multiple microservices and provides production-ready solutions.

## Security Concerns

### ❌ Problems with Shared Secrets

The original approach of passing `JWT_SECRET` and `REDIS_PASSWORD` to every microservice has several security issues:

1. **Secret Proliferation**: Secrets are duplicated across many services
2. **Exposure Risk**: Each service that knows the secret is a potential leak point
3. **Rotation Difficulty**: Changing secrets requires updating all services
4. **No Audit Trail**: Can't track which service used a secret
5. **Single Point of Failure**: If one service is compromised, all services are at risk

### ✅ Industry Best Practices

Leading companies (Google, Amazon, Netflix, Spotify) use these approaches:

1. **Public Key Infrastructure (PKI)**: Auth service signs with private key, microservices verify with public key
2. **Centralized Secret Management**: Vault, AWS Secrets Manager, Google Secret Manager
3. **Kubernetes Secrets**: Mount secrets as files in containers
4. **Service Mesh**: Istio, Linkerd handle authentication at network layer

## Solution: Secure Configuration Package

The `pkg/config` package provides multiple secure ways to manage secrets.

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│                Authentication Service                   │
│                                                         │
│  - Holds private key (RS256)                           │
│  - Signs JWT tokens with private key                   │
│  - Exposes public key endpoint: /api/public-key        │
│  - Has Redis password (single source of truth)         │
└─────────────────────────────────────────────────────────┘
                          │
                          │ Public Key (Safe to share)
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
        ▼                 ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ Microservice │  │ Microservice │  │ Microservice │
│      A       │  │      B       │  │      C       │
│              │  │              │  │              │
│ - Gets public│  │ - Gets public│  │ - Gets public│
│   key from   │  │   key from   │  │   key from   │
│   auth       │  │   auth       │  │   auth       │
│ - Validates  │  │ - Validates  │  │ - Validates  │
│   JWTs       │  │   JWTs       │  │   JWTs       │
│ - No secret! │  │ - No secret! │  │ - No secret! │
└──────────────┘  └──────────────┘  └──────────────┘
```

## Implementation Options

### Option 1: Public Key Infrastructure (RS256) - **RECOMMENDED FOR PRODUCTION**

This is the most secure approach. The authentication service signs tokens with a private key, and microservices validate with the public key.

#### Benefits
✅ Microservices don't need the secret key
✅ Public key can be shared openly
✅ Can't forge tokens even if microservice is compromised
✅ Easy key rotation (just update public key endpoint)
✅ Industry standard (Google, GitHub, Auth0 use this)

#### Setup

**Authentication Service:**
```go
// Generate RSA key pair (do this once, save securely)
privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
publicKey := &privateKey.PublicKey

// Sign tokens with private key
token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
tokenString, err := token.SignedString(privateKey)

// Expose public key endpoint
http.HandleFunc("/api/public-key", func(w http.ResponseWriter, r *http.Request) {
    pubKeyPEM := exportPublicKeyAsPEM(publicKey)
    w.Header().Set("Content-Type", "application/x-pem-file")
    w.Write(pubKeyPEM)
})
```

**Microservices:**
```go
import "github.com/abisalde/authentication-service/pkg/config"

// Initialize with public key provider
provider := config.NewPublicKeyProvider("https://auth.example.com")
configMgr := config.NewConfigManager(provider)

// Get public key (cached for 1 hour)
publicKey, err := configMgr.GetJWTPublicKey(ctx)

// Validate tokens
token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
        return nil, fmt.Errorf("unexpected signing method")
    }
    return publicKey, nil
})
```

### Option 2: Kubernetes Secrets - **RECOMMENDED FOR K8S DEPLOYMENTS**

Mount secrets as files in containers. Secrets are managed by Kubernetes and not exposed in environment variables.

#### Benefits
✅ Secrets stored encrypted in etcd
✅ RBAC controls who can access secrets
✅ Secrets rotated without redeploying
✅ Not visible in process lists
✅ Native Kubernetes integration

#### Setup

**Create Kubernetes Secrets:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: auth-secrets
type: Opaque
stringData:
  JWT_SECRET: "your-super-secret-key-here"
  REDIS_PASSWORD: "your-redis-password"
  REDIS_ADDR: "redis.default.svc.cluster.local:6379"
```

**Mount in Deployment:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: microservice-a
spec:
  template:
    spec:
      containers:
      - name: app
        image: microservice-a:latest
        volumeMounts:
        - name: secrets
          mountPath: /var/run/secrets/app
          readOnly: true
      volumes:
      - name: secrets
        secret:
          secretName: auth-secrets
```

**Application Code:**
```go
import "github.com/abisalde/authentication-service/pkg/config"

// Automatically detects K8s environment and uses file-based secrets
provider := config.DefaultSecretProvider()
configMgr := config.NewConfigManager(provider)

// Get secrets from mounted files
jwtSecret, err := configMgr.GetJWTSecret(ctx)
redisAddr, redisPass, err := configMgr.GetRedisConfig(ctx)
```

### Option 3: HashiCorp Vault - **ENTERPRISE SOLUTION**

Centralized secret management with encryption, auditing, and dynamic secrets.

#### Benefits
✅ Centralized secret storage
✅ Automatic secret rotation
✅ Audit logging
✅ Dynamic secrets (generated on demand)
✅ Fine-grained access control
✅ Encryption at rest and in transit

#### Setup

**Vault Configuration:**
```bash
# Store secrets in Vault
vault kv put secret/auth JWT_SECRET="your-secret"
vault kv put secret/redis PASSWORD="redis-pass" ADDR="redis:6379"

# Create policy for microservices
vault policy write microservice-policy - <<EOF
path "secret/data/auth" {
  capabilities = ["read"]
}
path "secret/data/redis" {
  capabilities = ["read"]
}
EOF
```

**Application Code:**
```go
import (
    "github.com/hashicorp/vault/api"
    "github.com/abisalde/authentication-service/pkg/config"
)

// Custom Vault provider
type VaultSecretProvider struct {
    client *api.Client
}

func (v *VaultSecretProvider) GetSecret(ctx context.Context, key string) (string, error) {
    secret, err := v.client.Logical().Read("secret/data/auth")
    if err != nil {
        return "", err
    }
    
    data := secret.Data["data"].(map[string]interface{})
    value, ok := data[key].(string)
    if !ok {
        return "", config.ErrSecretNotFound
    }
    return value, nil
}

// Use in application
vaultClient, _ := api.NewClient(api.DefaultConfig())
provider := &VaultSecretProvider{client: vaultClient}
configMgr := config.NewConfigManager(provider)
```

### Option 4: AWS Secrets Manager - **AWS CLOUD SOLUTION**

For applications running on AWS, use Secrets Manager for automatic rotation and IAM integration.

#### Benefits
✅ Automatic secret rotation
✅ IAM-based access control
✅ Integration with AWS services
✅ Encryption with AWS KMS
✅ Cross-region replication
✅ Audit with CloudTrail

#### Setup

**Store secrets:**
```bash
aws secretsmanager create-secret \
    --name auth/jwt-secret \
    --secret-string "your-super-secret-key"

aws secretsmanager create-secret \
    --name auth/redis \
    --secret-string '{"password":"redis-pass","addr":"redis:6379"}'
```

**IAM Policy:**
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue"
      ],
      "Resource": [
        "arn:aws:secretsmanager:*:*:secret:auth/*"
      ]
    }
  ]
}
```

**Application Code:**
```go
import (
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/secretsmanager"
)

type AWSSecretProvider struct {
    client *secretsmanager.SecretsManager
}

func (a *AWSSecretProvider) GetSecret(ctx context.Context, key string) (string, error) {
    input := &secretsmanager.GetSecretValueInput{
        SecretId: aws.String("auth/" + key),
    }
    
    result, err := a.client.GetSecretValue(input)
    if err != nil {
        return "", err
    }
    
    return *result.SecretString, nil
}

// Use with ConfigManager
sess := session.Must(session.NewSession())
provider := &AWSSecretProvider{
    client: secretsmanager.New(sess),
}
configMgr := config.NewConfigManager(provider)
```

## Migration Guide

### Step 1: Choose Your Approach

| Environment | Recommended Solution |
|-------------|---------------------|
| Production (any cloud) | **Public Key Infrastructure (RS256)** |
| Kubernetes | **K8s Secrets + Public Key** |
| AWS | **AWS Secrets Manager + Public Key** |
| GCP | **Google Secret Manager + Public Key** |
| Azure | **Azure Key Vault + Public Key** |
| Development | **Environment Variables** (temporary only) |

### Step 2: Implement Public Key Infrastructure

This is the most important security improvement:

1. **Generate RSA Key Pair** (authentication service only)
2. **Update JWT signing** to use RS256 instead of HS256
3. **Add public key endpoint** at `/api/public-key`
4. **Update microservices** to fetch and use public key
5. **Remove JWT_SECRET** from microservice configs

### Step 3: Implement Secret Management

Choose based on your infrastructure:

**For Kubernetes:**
```bash
# Create secrets
kubectl create secret generic auth-secrets \
  --from-literal=REDIS_ADDR=redis:6379 \
  --from-literal=REDIS_PASSWORD=your-password

# Update deployments to mount secrets
# Use config.DefaultSecretProvider() - auto-detects K8s
```

**For AWS:**
```bash
# Store in Secrets Manager
aws secretsmanager create-secret ...

# Update IAM roles for EC2/ECS/EKS
# Use AWSSecretProvider
```

**For other environments:**
- Use Vault for centralized management
- Or file-based secrets with proper permissions

### Step 4: Test in Staging

1. Deploy authentication service with public key endpoint
2. Deploy one microservice with new config
3. Verify token validation works
4. Monitor for errors
5. Gradually roll out to all services

### Step 5: Remove Shared Secrets

Once all services use public key:
1. Remove JWT_SECRET from microservice configs
2. Keep only in authentication service
3. Document the architecture
4. Set up monitoring and alerts

## Best Practices

### 1. Use Public Key Infrastructure (PKI)

✅ **DO**: Use RS256 (RSA) for JWT signing
❌ **DON'T**: Use HS256 (HMAC) in production

```go
// ✅ GOOD: Only auth service has private key
privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
tokenString, _ := token.SignedString(privateKey)

// ❌ BAD: Every service has the secret
token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
tokenString, _ := token.SignedString([]byte("shared-secret"))
```

### 2. Centralize Secret Management

✅ **DO**: Use a centralized secret store
❌ **DON'T**: Scatter secrets across configs

```go
// ✅ GOOD: Fetch from central store
configMgr := config.NewConfigManager(provider)
secret, _ := configMgr.GetJWTSecret(ctx)

// ❌ BAD: Hardcoded or in environment variables
secret := os.Getenv("JWT_SECRET")
```

### 3. Implement Secret Rotation

✅ **DO**: Plan for regular rotation
❌ **DON'T**: Use secrets indefinitely

```go
// ✅ GOOD: Fetch fresh keys periodically
publicKey, _ := configMgr.GetJWTPublicKey(ctx) // Cached for 1 hour

// ❌ BAD: Load once at startup
publicKey := loadPublicKeyOnce()
```

### 4. Use Least Privilege

✅ **DO**: Give services only what they need
❌ **DON'T**: Give all services all secrets

```yaml
# ✅ GOOD: Microservices only get Redis config
secrets:
  - REDIS_ADDR
  - REDIS_PASSWORD

# ❌ BAD: Microservices get auth service secrets
secrets:
  - JWT_SECRET
  - DATABASE_PASSWORD
  - ADMIN_API_KEY
```

### 5. Monitor Secret Access

✅ **DO**: Log and audit secret access
❌ **DON'T**: Allow untracked access

```go
// ✅ GOOD: Log secret retrieval
log.Printf("Service %s retrieved secret %s", serviceName, secretKey)
secret, _ := provider.GetSecret(ctx, secretKey)

// ❌ BAD: Silent retrieval
secret := os.Getenv(secretKey)
```

## Security Checklist

Before deploying to production:

- [ ] Switched from HS256 to RS256 for JWT signing
- [ ] Authentication service has private key only
- [ ] Public key endpoint is accessible
- [ ] Microservices use public key for validation
- [ ] JWT_SECRET removed from microservice configs
- [ ] Secrets stored in secure backend (Vault, K8s Secrets, AWS Secrets Manager)
- [ ] Secrets mounted as files, not environment variables
- [ ] RBAC/IAM policies configured for secret access
- [ ] Secret rotation strategy documented
- [ ] Monitoring and alerting for secret access
- [ ] Audit logs enabled
- [ ] Backup and recovery plan for secrets
- [ ] Documentation updated

## Performance Impact

The secure configuration approach has minimal performance impact:

| Operation | Time | Notes |
|-----------|------|-------|
| Public key fetch | 10-50ms | Cached for 1 hour |
| RS256 validation | 0.5-1ms | Slightly slower than HS256 |
| File-based secret read | < 1ms | Cached in memory |
| Vault secret fetch | 5-20ms | Cached for configured TTL |

**Overall impact: < 5ms per request** (mostly one-time at startup)

## Troubleshooting

### Public Key Not Found

```
Error: failed to fetch public key: status 404
```

**Solution**: Ensure authentication service exposes `/api/public-key` endpoint

### Secret Not Found in K8s

```
Error: failed to read secret: no such file or directory
```

**Solution**: Check secret is created and mounted:
```bash
kubectl get secrets
kubectl describe pod <pod-name>
```

### Vault Connection Failed

```
Error: failed to connect to Vault
```

**Solution**: Check Vault address and token:
```bash
export VAULT_ADDR="https://vault.example.com"
export VAULT_TOKEN="your-token"
```

## Related Documentation

- [Multi-Device Session Management](./MULTI_DEVICE_SESSION_MANAGEMENT.md)
- [Deployment Architecture](./DEPLOYMENT_ARCHITECTURE.md)
- [Session Integration Guide](./SESSION_INTEGRATION_GUIDE.md)

## References

- [JWT Best Practices](https://tools.ietf.org/html/rfc8725)
- [OWASP Secrets Management](https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html)
- [Google Cloud Security Best Practices](https://cloud.google.com/security/best-practices)
- [AWS Secrets Manager Best Practices](https://docs.aws.amazon.com/secretsmanager/latest/userguide/best-practices.html)
