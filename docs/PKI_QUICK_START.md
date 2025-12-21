# Public Key Infrastructure (PKI) Quick Start Guide

This is a condensed guide to get you started with the Public Key Infrastructure for secure session management. For detailed information, see [IMPLEMENTATION_CHECKLIST.md](./IMPLEMENTATION_CHECKLIST.md).

## 🚀 5-Minute Setup

### 1. Auth Service (Already Done ✅)

The public key endpoint is already integrated! Just start your service:

```bash
JWT_SECRET=your-secret-key go run cmd/server/main.go
```

Test it:

```bash
curl http://localhost:8080/api/public-key
```

### 2. Update Your Microservices

Replace this:

```go
// ❌ OLD WAY (Insecure - shared secrets)
validator := session.NewSessionValidator(session.Config{
    JWTSecret: os.Getenv("JWT_SECRET"),
})
```

With this:

```go
// ✅ NEW WAY (Secure - public key only)
import "github.com/abisalde/authentication-service/pkg/config"

secretProvider := config.DefaultSecretProvider()
configMgr := config.NewConfigManager(secretProvider)

publicKey, _ := configMgr.GetJWTPublicKey(context.Background())

validator := session.NewSessionValidator(session.Config{
    JWTSecret: publicKey, // Fetched from auth service!
})
```

### 3. Remove JWT_SECRET from Microservices

```bash
# ❌ DELETE from microservice .env
JWT_SECRET=secret

# ✅ ADD this instead
AUTH_SERVICE_URL=http://auth-service:8080
```

### 4. Test End-to-End

```bash
# 1. Login (get token)
TOKEN=$(curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"pass"}' \
  | jq -r '.token')

# 2. Use token with microservice
curl http://localhost:8081/api/protected \
  -H "Authorization: Bearer $TOKEN"

# Should work! ✅
```

## 📊 Implementation Checklist

- [x] Auth service exposes `/api/public-key` ✅
- [x] Tests created for public key endpoint ✅
- [x] `pkg/config` created with secure configuration ✅
- [x] Documentation created ✅
- [x] Examples provided ✅
- [ ] Integrate into your microservices (← **Your task**)
- [ ] Remove JWT_SECRET from microservices (← **Your task**)
- [ ] Set up monitoring (← **Your task**)
- [ ] Plan key rotation (← **Your task**)

## 🔄 What Happens Now?

1. **Auth Service**: Holds `JWT_SECRET` (single source of truth)
2. **Public Key Endpoint**: Exposes key via `/api/public-key`
3. **Microservices**: Fetch public key automatically
4. **Token Validation**: Works without sharing secrets!

## 📚 Next Steps

- Read [IMPLEMENTATION_CHECKLIST.md](./IMPLEMENTATION_CHECKLIST.md) for detailed guide
- Check [SECURE_CONFIG_INTEGRATION_GUIDE.md](./SECURE_CONFIG_INTEGRATION_GUIDE.md) for integration patterns
- Review [examples/secure-microservice/](../examples/secure-microservice/) for working code

## 🆘 Need Help?

- **Can't fetch public key?** Check `AUTH_SERVICE_URL` environment variable
- **Token validation fails?** Wait 5 minutes for cache to refresh
- **Build errors?** Run `go mod tidy` to update dependencies

## ✅ Success Criteria

You're done when:
- ✅ Microservices fetch public key automatically
- ✅ JWT_SECRET removed from all microservices
- ✅ Tokens validate successfully
- ✅ No shared secrets across services

🎉 **Congratulations!** You now have enterprise-grade security for your microservices!
