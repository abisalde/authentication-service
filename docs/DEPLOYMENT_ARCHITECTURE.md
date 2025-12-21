# Deployment Architecture for Distributed Session Management

## Overview

This document describes the deployment architecture for the authentication service and microservices with distributed session management.

## Architecture Diagram

```
                                    ┌─────────────────┐
                                    │   Load Balancer │
                                    │    (NGINX/ALB)  │
                                    └────────┬────────┘
                                             │
                    ┌────────────────────────┼────────────────────────┐
                    │                        │                        │
           ┌────────▼────────┐      ┌───────▼────────┐      ┌───────▼────────┐
           │  Auth Service   │      │  Microservice  │      │  Microservice  │
           │   Instance 1    │      │   A - Inst 1   │      │   B - Inst 1   │
           └────────┬────────┘      └───────┬────────┘      └───────┬────────┘
                    │                       │                        │
           ┌────────▼────────┐      ┌───────▼────────┐      ┌───────▼────────┐
           │  Auth Service   │      │  Microservice  │      │  Microservice  │
           │   Instance 2    │      │   A - Inst 2   │      │   B - Inst 2   │
           └────────┬────────┘      └───────┬────────┘      └───────┬────────┘
                    │                       │                        │
                    └───────────────────────┼────────────────────────┘
                                            │
                                   ┌────────▼────────┐
                                   │  Redis Cluster  │
                                   │   (Pub/Sub +    │
                                   │   Blacklist)    │
                                   └────────┬────────┘
                                            │
                                   ┌────────▼────────┐
                                   │  MySQL Cluster  │
                                   │  (User Data)    │
                                   └─────────────────┘
```

## Components

### 1. Load Balancer (NGINX/AWS ALB)

**Configuration:**
```nginx
upstream auth_service {
    least_conn;  # Use least connections for better distribution
    server auth-service-1:8080 max_fails=3 fail_timeout=30s;
    server auth-service-2:8080 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream microservice_a {
    least_conn;
    server microservice-a-1:8081 max_fails=3 fail_timeout=30s;
    server microservice-a-2:8081 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

server {
    listen 80;
    server_name api.example.com;

    # Auth service endpoints
    location /auth/ {
        proxy_pass http://auth_service/;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        
        # Health check
        proxy_next_upstream error timeout http_502 http_503 http_504;
    }

    # Microservice A endpoints
    location /api/orders/ {
        proxy_pass http://microservice_a/api/;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Authorization $http_authorization;
    }

    # Health check endpoint
    location /health {
        access_log off;
        return 200 "healthy\n";
    }
}
```

**Key Features:**
- No sticky sessions required (stateless design)
- Round-robin or least-connections load balancing
- Health checks to remove unhealthy instances
- Connection pooling with keepalive

### 2. Authentication Service

**Deployment Specs:**
```yaml
# kubernetes-auth-service.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: auth-service
  template:
    metadata:
      labels:
        app: auth-service
    spec:
      containers:
      - name: auth-service
        image: auth-service:latest
        ports:
        - containerPort: 8080
        env:
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: auth-secrets
              key: jwt-secret
        - name: REDIS_ADDR
          value: "redis-cluster:6379"
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-secrets
              key: password
        - name: DB_HOST
          value: "mysql-cluster"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: auth-service
spec:
  selector:
    app: auth-service
  ports:
  - protocol: TCP
    port: 8080
    targetPort: 8080
  type: ClusterIP
```

**Responsibilities:**
- User authentication (login/register)
- Token generation (access + refresh)
- Token blacklisting
- Publishing invalidation events
- User management

### 3. Microservices

**Deployment Specs:**
```yaml
# kubernetes-microservice-a.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: microservice-a
spec:
  replicas: 3
  selector:
    matchLabels:
      app: microservice-a
  template:
    metadata:
      labels:
        app: microservice-a
    spec:
      containers:
      - name: microservice-a
        image: microservice-a:latest
        ports:
        - containerPort: 8081
        env:
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: auth-secrets
              key: jwt-secret
        - name: REDIS_ADDR
          value: "redis-cluster:6379"
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-secrets
              key: password
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "250m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8081
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: microservice-a
spec:
  selector:
    app: microservice-a
  ports:
  - protocol: TCP
    port: 8081
    targetPort: 8081
  type: ClusterIP
```

**Responsibilities:**
- Validate JWT tokens locally
- Subscribe to token invalidation events
- Business logic (orders, payments, etc.)
- NO calls to auth service for validation

### 4. Redis Cluster

**Deployment:**
```yaml
# kubernetes-redis.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis
spec:
  serviceName: redis
  replicas: 3
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
        command:
          - redis-server
          - "--requirepass"
          - "$(REDIS_PASSWORD)"
          - "--maxmemory"
          - "256mb"
          - "--maxmemory-policy"
          - "allkeys-lru"
        env:
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-secrets
              key: password
        volumeMounts:
        - name: redis-data
          mountPath: /data
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
  volumeClaimTemplates:
  - metadata:
      name: redis-data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 1Gi
---
apiVersion: v1
kind: Service
metadata:
  name: redis-cluster
spec:
  selector:
    app: redis
  ports:
  - protocol: TCP
    port: 6379
    targetPort: 6379
  type: ClusterIP
```

**Usage:**
- Token blacklist storage
- Pub/sub for invalidation events
- Refresh token storage
- Rate limiting counters

**Configuration:**
- Enable persistence (RDB or AOF)
- Configure maxmemory policy: `allkeys-lru`
- Set reasonable TTLs for blacklist entries
- Monitor memory usage

### 5. MySQL Cluster

**Deployment:**
```yaml
# kubernetes-mysql.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mysql
spec:
  serviceName: mysql
  replicas: 1  # Or 3 for HA with replication
  selector:
    matchLabels:
      app: mysql
  template:
    metadata:
      labels:
        app: mysql
    spec:
      containers:
      - name: mysql
        image: mysql:8.0
        ports:
        - containerPort: 3306
        env:
        - name: MYSQL_ROOT_PASSWORD
          valueFrom:
            secretKeyRef:
              name: mysql-secrets
              key: root-password
        - name: MYSQL_DATABASE
          value: authservice
        volumeMounts:
        - name: mysql-data
          mountPath: /var/lib/mysql
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
  volumeClaimTemplates:
  - metadata:
      name: mysql-data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 10Gi
---
apiVersion: v1
kind: Service
metadata:
  name: mysql-cluster
spec:
  selector:
    app: mysql
  ports:
  - protocol: TCP
    port: 3306
    targetPort: 3306
  type: ClusterIP
```

**Usage:**
- User account storage
- User profile data
- OAuth provider mappings
- Audit logs

## Scaling Strategy

### Horizontal Scaling

**Auth Service:**
```bash
kubectl scale deployment auth-service --replicas=5
```

**Microservices:**
```bash
kubectl scale deployment microservice-a --replicas=10
```

**Auto-scaling with HPA:**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: auth-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: auth-service
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### Vertical Scaling

Adjust resource limits based on monitoring:

```yaml
resources:
  requests:
    memory: "512Mi"  # Increase for high-memory workloads
    cpu: "500m"      # Increase for CPU-intensive operations
  limits:
    memory: "1Gi"
    cpu: "1000m"
```

## Multi-Region Deployment

```
Region 1 (US-East)              Region 2 (EU-West)
┌─────────────────┐             ┌─────────────────┐
│  Auth Service   │             │  Auth Service   │
│  Microservices  │             │  Microservices  │
└────────┬────────┘             └────────┬────────┘
         │                               │
┌────────▼────────┐             ┌────────▼────────┐
│  Redis Cluster  │◄────────────►  Redis Cluster  │
│  (Replication)  │             │  (Replication)  │
└────────┬────────┘             └────────┬────────┘
         │                               │
         └───────────────┬───────────────┘
                         │
                 ┌───────▼───────┐
                 │ MySQL Primary │
                 │ (Multi-AZ)    │
                 └───────────────┘
```

**Key Considerations:**
- Redis replication for pub/sub across regions
- Accept eventual consistency (< 100ms)
- Use global load balancer (AWS Global Accelerator, Cloudflare)
- Replicate MySQL with read replicas in each region

## Security Configuration

### 1. Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: auth-service-network-policy
spec:
  podSelector:
    matchLabels:
      app: auth-service
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: redis
    ports:
    - protocol: TCP
      port: 6379
  - to:
    - podSelector:
        matchLabels:
          app: mysql
    ports:
    - protocol: TCP
      port: 3306
```

### 2. Secrets Management

**Using Kubernetes Secrets:**
```bash
# Create JWT secret
kubectl create secret generic auth-secrets \
  --from-literal=jwt-secret=$(openssl rand -base64 32)

# Create Redis password
kubectl create secret generic redis-secrets \
  --from-literal=password=$(openssl rand -base64 32)

# Create MySQL password
kubectl create secret generic mysql-secrets \
  --from-literal=root-password=$(openssl rand -base64 32)
```

**Or use external secret managers:**
- AWS Secrets Manager
- HashiCorp Vault
- Google Secret Manager
- Azure Key Vault

### 3. TLS/SSL

```yaml
apiVersion: v1
kind: Service
metadata:
  name: auth-service
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-ssl-cert: "arn:aws:acm:..."
    service.beta.kubernetes.io/aws-load-balancer-backend-protocol: "http"
spec:
  type: LoadBalancer
  ports:
  - port: 443
    targetPort: 8080
    protocol: TCP
```

## Monitoring and Observability

### Metrics to Monitor

**Auth Service:**
- Token generation rate
- Token validation rate
- Login success/failure rate
- Blacklist operation latency
- Redis pub/sub lag

**Microservices:**
- Token validation success/failure rate
- Token validation latency
- Redis connection health
- Blacklist cache hit rate
- Invalidation event processing time

**Redis:**
- Memory usage
- Pub/sub channel subscribers
- Command latency
- Connection count

**MySQL:**
- Query latency
- Connection pool usage
- Slow query log
- Replication lag (if applicable)

### Prometheus Configuration

```yaml
# prometheus-config.yaml
scrape_configs:
  - job_name: 'auth-service'
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: auth-service
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: instance

  - job_name: 'microservices'
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: microservice-.*
```

### Grafana Dashboards

Create dashboards for:
1. Authentication service overview
2. Token validation metrics
3. Redis performance
4. Microservice health
5. Session management flow

### Alerting Rules

```yaml
groups:
  - name: auth-service-alerts
    rules:
      - alert: HighTokenValidationFailureRate
        expr: rate(token_validation_failures[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High token validation failure rate"
          
      - alert: RedisConnectionFailure
        expr: redis_up == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Redis connection failed"
          
      - alert: HighAuthServiceLatency
        expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "95th percentile latency > 500ms"
```

## Disaster Recovery

### Backup Strategy

**Redis:**
- Enable RDB snapshots every 5 minutes
- Enable AOF for point-in-time recovery
- Replicate to backup region

**MySQL:**
- Daily full backups
- Binary log backup for PITR
- Test restore procedures monthly

### Recovery Procedures

**Redis Failure:**
1. Microservices continue to work (degraded mode)
2. JWT validation works, blacklist checks skipped
3. Restore Redis from backup
4. Resume pub/sub subscription

**Auth Service Failure:**
1. Microservices continue to validate tokens
2. New logins/registrations unavailable
3. Scale up remaining instances
4. Route traffic to healthy instances

**Database Failure:**
1. Token validation continues to work
2. User lookups fail
3. Failover to read replica
4. Restore from backup if needed

## Cost Optimization

### Resource Allocation

**Development:**
- Auth Service: 2 replicas, 128Mi memory
- Microservices: 1 replica each, 64Mi memory
- Redis: Single instance, 128Mi memory

**Production:**
- Auth Service: 3-5 replicas, 256-512Mi memory
- Microservices: 3-10 replicas, 128-256Mi memory
- Redis: 3 nodes cluster, 512Mi memory

### Cost-Saving Tips

1. Use Horizontal Pod Autoscaler to scale down during low traffic
2. Implement pod disruption budgets for graceful scaling
3. Use spot/preemptible instances for non-critical workloads
4. Set appropriate resource requests/limits to avoid over-provisioning
5. Use Redis for caching to reduce database queries

## Conclusion

This architecture provides:
- ✅ High availability and fault tolerance
- ✅ Horizontal scalability for all components
- ✅ Zero-downtime deployments
- ✅ Proper security isolation
- ✅ Comprehensive monitoring and alerting
- ✅ Disaster recovery capabilities
- ✅ Cost optimization opportunities
