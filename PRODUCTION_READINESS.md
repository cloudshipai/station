# Station Production Readiness Guide

This document outlines the requirements and recommendations for deploying Station in production environments.

## 📋 Pre-Production Checklist

### ✅ Security Requirements

#### Authentication & Authorization
- [ ] **SSH Authentication**: Configure proper SSH key-based authentication
- [ ] **API Keys**: Generate and secure API keys for programmatic access  
- [ ] **Environment Isolation**: Set up separate environments (dev/staging/prod)
- [ ] **User Management**: Create dedicated service accounts
- [ ] **Audit Logging**: Enable comprehensive audit trails

#### Network Security
- [ ] **Firewall Rules**: Restrict access to required ports only
- [ ] **TLS/SSL**: Enable HTTPS for all API endpoints
- [ ] **CORS Policy**: Configure restrictive CORS for production domains
- [ ] **Rate Limiting**: Implement API rate limiting
- [ ] **VPN/Private Network**: Deploy within secure network perimeter

#### Data Protection
- [ ] **Encryption at Rest**: Enable database encryption
- [ ] **Secrets Management**: Use proper secret management (not environment files)
- [ ] **Backup Strategy**: Implement automated encrypted backups
- [ ] **Data Retention**: Configure appropriate data retention policies

### 🏗️ Infrastructure Requirements

#### System Resources
- **Minimum Requirements**:
  - CPU: 2 cores
  - Memory: 4GB RAM  
  - Storage: 20GB SSD
  - Network: 100Mbps
  
- **Recommended for Production**:
  - CPU: 4+ cores
  - Memory: 8GB+ RAM
  - Storage: 100GB+ SSD with backup
  - Network: 1Gbps with redundancy

#### Database
- [ ] **PostgreSQL/MySQL**: Use production-grade database
- [ ] **Connection Pooling**: Configure proper connection limits
- [ ] **Backup & Recovery**: Automated daily backups with point-in-time recovery
- [ ] **Monitoring**: Database performance monitoring
- [ ] **High Availability**: Consider read replicas for scale

#### Observability
- [ ] **Logging**: Structured logging with log aggregation
- [ ] **Metrics**: System and application metrics collection
- [ ] **Alerting**: Critical error and performance alerting
- [ ] **Tracing**: Request tracing for debugging
- [ ] **Health Checks**: Automated health monitoring

### 🚀 Deployment Architecture

#### Single-Node Deployment
```yaml
# docker-compose.prod.yml
version: '3.8'
services:
  station:
    image: station:latest
    environment:
      - STN_DATABASE_URL=postgresql://user:pass@db:5432/station
      - STN_ENCRYPTION_KEY=${ENCRYPTION_KEY}
      - STN_API_PORT=8080
      - STN_SSH_PORT=2222
      - STN_TELEMETRY_ENABLED=true
    volumes:
      - ./config:/app/config:ro
      - station_data:/app/data
    ports:
      - "8080:8080"
      - "2222:2222"
    restart: unless-stopped
    
  db:
    image: postgres:15
    environment:
      - POSTGRES_DB=station
      - POSTGRES_USER=${DB_USER}
      - POSTGRES_PASSWORD=${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  station_data:
  postgres_data:
```

#### Kubernetes Deployment
```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: station
spec:
  replicas: 3
  selector:
    matchLabels:
      app: station
  template:
    metadata:
      labels:
        app: station
    spec:
      containers:
      - name: station
        image: station:latest
        env:
        - name: STN_DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: station-secrets
              key: database-url
        - name: STN_ENCRYPTION_KEY
          valueFrom:
            secretKeyRef:
              name: station-secrets
              key: encryption-key
        ports:
        - containerPort: 8080
        - containerPort: 2222
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"
          limits:
            memory: "4Gi" 
            cpu: "2000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

## 🔧 Configuration Management

### Environment Configuration

#### Production Environment Variables
```bash
# Core Settings
export STN_DATABASE_URL="postgresql://station:${DB_PASSWORD}@localhost:5432/station_prod"
export STN_ENCRYPTION_KEY="${ENCRYPTION_KEY}"  # 32-byte key
export STN_API_PORT="8080"
export STN_SSH_PORT="2222"
export STN_MCP_PORT="3000"

# Security Settings  
export STN_TLS_CERT_PATH="/etc/ssl/certs/station.crt"
export STN_TLS_KEY_PATH="/etc/ssl/private/station.key"
export STN_CORS_ORIGINS="https://station.company.com,https://app.company.com"

# AI Provider Settings
export STN_AI_PROVIDER="openai"  # or anthropic, gemini
export STN_AI_API_KEY="${AI_API_KEY}"
export STN_AI_MODEL="gpt-4"

# Telemetry (Optional)
export STN_TELEMETRY_ENABLED="true"
export STN_POSTHOG_API_KEY="${POSTHOG_KEY}"
export STN_POSTHOG_HOST="https://app.posthog.com"

# Logging
export STN_LOG_LEVEL="info"  # debug, info, warn, error
export STN_LOG_FORMAT="json"  # json, text
```

#### Configuration Files
```yaml
# config/production.yaml
database:
  url: ${STN_DATABASE_URL}
  max_connections: 20
  connection_timeout: 30s

api:
  port: ${STN_API_PORT:8080}
  cors_origins: ${STN_CORS_ORIGINS}
  rate_limit:
    requests_per_minute: 100
    burst: 20

ssh:
  port: ${STN_SSH_PORT:2222}
  host_key_path: /etc/station/ssh_host_key
  authorized_keys_path: /etc/station/authorized_keys

security:
  encryption_key: ${STN_ENCRYPTION_KEY}
  session_timeout: 24h
  max_concurrent_sessions: 10

telemetry:
  enabled: ${STN_TELEMETRY_ENABLED:false}
  posthog:
    api_key: ${STN_POSTHOG_API_KEY}
    host: ${STN_POSTHOG_HOST}
```

## 🔍 Monitoring & Alerting

### Health Checks
Station exposes the following health endpoints:

- **`GET /health`** - Basic health check
- **`GET /ready`** - Readiness check (database connectivity)
- **`GET /metrics`** - Prometheus metrics endpoint

### Key Metrics to Monitor

#### System Metrics
- CPU usage (< 70% sustained)
- Memory usage (< 80%)
- Disk space (< 85%)
- Network connectivity
- File descriptor usage

#### Application Metrics
- HTTP request rate and latency
- Database connection pool usage
- MCP server connection health
- Agent execution success rate
- SSH connection count
- Queue processing time

#### Business Metrics
- Active agents count
- Execution volume per day
- Error rates by component
- User session duration

### Sample Alerting Rules
```yaml
# prometheus/alerts.yml
groups:
- name: station
  rules:
  - alert: StationDown
    expr: up{job="station"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Station instance is down"
      
  - alert: HighErrorRate
    expr: rate(station_http_requests_total{code=~"5.."}[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High error rate detected"
      
  - alert: DatabaseConnectionFailure
    expr: station_database_connections_failed_total > 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Database connection failures detected"
```

## 🔐 Security Hardening

### System Hardening
```bash
# Create dedicated user
useradd -r -s /bin/false station

# Set file permissions
chown -R station:station /opt/station
chmod 750 /opt/station
chmod 600 /opt/station/config/*

# Configure systemd service
cat > /etc/systemd/system/station.service << EOF
[Unit]
Description=Station AI Agent Platform
After=network.target

[Service]
Type=simple
User=station
Group=station
WorkingDirectory=/opt/station
ExecStart=/opt/station/station server
Restart=always
RestartSec=10
KillMode=mixed
TimeoutStopSec=30

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/station/data

[Install]
WantedBy=multi-user.target
EOF
```

### Firewall Configuration
```bash
# UFW rules for Ubuntu/Debian
ufw default deny incoming
ufw default allow outgoing
ufw allow 8080/tcp  # API port
ufw allow 2222/tcp  # SSH port
ufw allow from 10.0.0.0/8 to any port 3000  # MCP port (internal)
ufw enable
```

## 📊 Performance Tuning

### Database Optimization
```sql
-- PostgreSQL performance settings
ALTER SYSTEM SET shared_buffers = '256MB';
ALTER SYSTEM SET effective_cache_size = '1GB';
ALTER SYSTEM SET maintenance_work_mem = '64MB';
ALTER SYSTEM SET checkpoint_completion_target = 0.7;
ALTER SYSTEM SET wal_buffers = '16MB';
ALTER SYSTEM SET default_statistics_target = 100;
SELECT pg_reload_conf();

-- Create performance indexes
CREATE INDEX CONCURRENTLY idx_agent_runs_status_created 
ON agent_runs(status, created_at) WHERE status IN ('running', 'queued');

CREATE INDEX CONCURRENTLY idx_agents_environment_enabled 
ON agents(environment_id, enabled) WHERE enabled = true;
```

### Application Tuning
```yaml
# High-performance configuration
execution_queue:
  workers: 10  # Adjust based on CPU cores
  buffer_size: 1000
  timeout: 300s

database:
  max_connections: 50
  max_idle_connections: 10
  connection_max_idle_time: 1h
  connection_max_lifetime: 4h

mcp:
  connection_pool_size: 20
  request_timeout: 30s
  max_retries: 3
```

## 🚨 Disaster Recovery

### Backup Strategy
```bash
#!/bin/bash
# backup-station.sh

# Database backup
pg_dump $STN_DATABASE_URL | gzip > "/backups/station-db-$(date +%Y%m%d-%H%M%S).sql.gz"

# Configuration backup
tar -czf "/backups/station-config-$(date +%Y%m%d-%H%M%S).tar.gz" /opt/station/config/

# Data directory backup
tar -czf "/backups/station-data-$(date +%Y%m%d-%H%M%S).tar.gz" /opt/station/data/

# Rotate old backups (keep 30 days)
find /backups -name "station-*" -mtime +30 -delete
```

### Recovery Procedures
```bash
# Database recovery
gunzip -c station-db-20240115-120000.sql.gz | psql $STN_DATABASE_URL

# Configuration recovery  
tar -xzf station-config-20240115-120000.tar.gz -C /

# Data recovery
tar -xzf station-data-20240115-120000.tar.gz -C /opt/station/

# Restart services
systemctl restart station
```

## 📚 Operational Procedures

### Deployment Process
1. **Pre-deployment Checks**
   - [ ] Run test suite
   - [ ] Database migration dry-run
   - [ ] Configuration validation
   - [ ] Security scan

2. **Deployment Steps**
   - [ ] Take configuration backup
   - [ ] Stop Station service
   - [ ] Deploy new version
   - [ ] Run database migrations
   - [ ] Update configuration
   - [ ] Start Station service
   - [ ] Verify health checks

3. **Post-deployment Validation**
   - [ ] Health endpoint checks
   - [ ] Agent execution tests
   - [ ] API functionality tests
   - [ ] Monitor error rates

### Maintenance Windows
- **Database Maintenance**: Monthly, 2-hour window
- **Security Updates**: Weekly, 30-minute window  
- **Application Updates**: Bi-weekly, 1-hour window
- **Infrastructure Updates**: Quarterly, 4-hour window

### Incident Response
1. **Detection**: Automated alerts or user reports
2. **Assessment**: Check logs, metrics, and health status
3. **Mitigation**: Implement immediate fixes
4. **Communication**: Update status page and notify users
5. **Resolution**: Deploy permanent fix
6. **Post-mortem**: Document lessons learned

## 🔧 Troubleshooting Guide

### Common Issues

#### Service Won't Start
```bash
# Check logs
journalctl -u station -f

# Verify configuration
station config validate

# Check database connectivity
psql $STN_DATABASE_URL -c "SELECT 1;"

# Verify file permissions
ls -la /opt/station/
```

#### High Memory Usage
```bash
# Check memory usage
ps aux | grep station
top -p $(pgrep station)

# Tune garbage collection
export GOGC=100
export GOMEMLIMIT=2GiB
```

#### Database Performance Issues
```sql
-- Check slow queries
SELECT query, mean_time, calls 
FROM pg_stat_statements 
ORDER BY mean_time DESC LIMIT 10;

-- Check connection usage
SELECT count(*) as connections, state 
FROM pg_stat_activity 
GROUP BY state;
```

#### Agent Execution Failures
```bash
# Check agent logs
station agent logs <agent-id>

# Verify MCP server connectivity
station mcp test <server-name>

# Check queue status
station queue status
```

## ✅ Production Checklist Summary

Before going live:

### Security & Compliance
- [ ] Security audit completed
- [ ] Penetration testing performed
- [ ] Compliance requirements met
- [ ] Incident response plan documented

### Performance & Reliability  
- [ ] Load testing completed
- [ ] Performance benchmarks established
- [ ] Monitoring and alerting configured
- [ ] Backup and recovery tested

### Operations & Maintenance
- [ ] Deployment procedures documented
- [ ] Operational runbooks created
- [ ] Team training completed
- [ ] Support processes established

---

**🎉 Congratulations!** Your Station deployment is ready for production. Remember to regularly review and update your security configurations, monitor performance metrics, and maintain current backups.