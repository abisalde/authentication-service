# Secure Microservice Example

This example demonstrates how to securely configure a microservice without hardcoding secrets or sharing JWT signing keys.

## Security Features

✅ **Automatic Secret Detection**: Detects environment and uses appropriate secret provider
✅ **Multiple Secret Sources**: Supports environment vars, Kubernetes secrets, public key provider
✅ **No Hardcoded Secrets**: All secrets loaded from secure sources
✅ **Production Ready**: Designed for Kubernetes, AWS, and other cloud platforms

## Running the Example

### Development Mode (Environment Variables)

```bash
# Set environment variables
export JWT_SECRET="your-development-secret"
export REDIS_ADDR="localhost:6379"
export REDIS_PASSWORD="" # Optional
export PORT="8081"

# Run the service
go run main.go
```

**Security Level**: 🟡 MEDIUM (Development Only)

### Production Mode (Public Key Provider)

```bash
# Set auth service URL - microservice will fetch public key
export AUTH_SERVICE_URL="https://auth.example.com"
export REDIS_ADDR="redis.production.svc.cluster.local:6379"
export REDIS_PASSWORD="$(cat /var/run/secrets/redis/password)"
export PORT="8081"

# Run the service
go run main.go
```

**Security Level**: 🟢 HIGH (Production Ready)

### Kubernetes Deployment

Create secrets:
```bash
kubectl create secret generic auth-secrets \
  --from-literal=JWT_SECRET=your-secret \
  --from-literal=REDIS_ADDR=redis:6379 \
  --from-literal=REDIS_PASSWORD=redis-password
```

Deploy:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: secure-microservice
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: app
        image: secure-microservice:latest
        volumeMounts:
        - name: secrets
          mountPath: /var/run/secrets/app
          readOnly: true
        env:
        - name: PORT
          value: "8080"
      volumes:
      - name: secrets
        secret:
          secretName: auth-secrets
```

**Security Level**: 🟢 HIGH (Production Ready)

## Testing

### Public Endpoint (No Auth)
```bash
curl http://localhost:8081/api/public
```

### Protected Endpoint (Requires Auth)
```bash
# Get token from auth service first
TOKEN="your-jwt-token"

curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8081/api/protected
```

### Security Info
```bash
curl http://localhost:8081/api/security-info
```

## Security Comparison

| Method | Security Level | Use Case |
|--------|---------------|----------|
| Environment Variables | 🟡 MEDIUM | Development only |
| Kubernetes Secrets | 🟢 HIGH | Production (K8s) |
| Public Key Provider | 🟢 HIGH | Any production |
| Vault/AWS Secrets | 🟢 HIGH | Enterprise |

## Best Practices

1. **Never commit secrets** to version control
2. **Use public key infrastructure** (RS256) in production
3. **Rotate secrets regularly** (at least every 90 days)
4. **Monitor secret access** with audit logs
5. **Use least privilege** - give services only what they need

## Related Documentation

- [Secure Configuration Guide](../../docs/SECURE_CONFIGURATION.md)
- [Session Integration Guide](../../docs/SESSION_INTEGRATION_GUIDE.md)
- [Deployment Architecture](../../docs/DEPLOYMENT_ARCHITECTURE.md)
