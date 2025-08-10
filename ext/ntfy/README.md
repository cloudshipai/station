# ntfy Server Deployment on Fly.io

This directory contains the configuration for deploying a [ntfy](https://ntfy.sh) notification server on Fly.io with Docker.

## 🚀 Quick Deployment

### Prerequisites

- [Fly.io CLI](https://fly.io/docs/hands-on/install-flyctl/) installed
- Fly.io account and authentication

### Deploy Steps

1. **Navigate to the ntfy directory**:
   ```bash
   cd ext/ntfy
   ```

2. **Deploy with your app name**:
   ```bash
   ./deploy.sh your-app-name
   ```
   
   Or with a specific region:
   ```bash
   ./deploy.sh your-app-name ord
   ```

The deploy script will:
- Generate `fly.toml` and `server.yml` from templates with your app name
- Create the Fly.io app if it doesn't exist
- Deploy the ntfy server

3. **Alternative: Manual deployment**:
   ```bash
   flyctl apps create your-ntfy-server
   ```

3. **Create persistent volume for data**:
   ```bash
   flyctl volumes create ntfy_data --region ord --size 1
   ```

4. **Update configuration**:
   - Edit `fly.toml` and replace `your-ntfy-server` with your actual app name
   - Update `server.yml` and replace `your-ntfy-server.fly.dev` with your domain

5. **Deploy**:
   ```bash
   flyctl deploy
   ```

6. **Check status**:
   ```bash
   flyctl status
   flyctl logs
   ```

## 📋 Configuration

### Fly.io Configuration (`fly.toml`)

- **App Name**: Configure your unique app name
- **Region**: Set to your preferred region (default: `ord`)
- **Resources**: 1 CPU, 256MB RAM (adjustable)
- **Auto-scaling**: Enabled with auto-stop/start
- **Health Checks**: HTTP health check on `/v1/health`
- **Persistent Storage**: Volume mounted at `/var/cache/ntfy`

### ntfy Configuration (`server.yml`)

Key settings configured for production:

- **Base URL**: Must match your Fly.io app URL
- **Authentication**: SQLite-based user management
- **Caching**: 12-hour message retention
- **Attachments**: 5GB total, 15MB per file
- **Rate Limiting**: 60 requests/burst, 10K messages/day
- **CORS**: Enabled for web access
- **Web UI**: Disabled by default (set `web-root: "app"` to enable)

## 🔧 Customization

### Enable Web UI

Edit `server.yml`:
```yaml
web-root: "app"
```

### Add Authentication

Edit `server.yml`:
```yaml
auth-default-access: "deny-all"
```

Then create users via CLI:
```bash
flyctl ssh console
ntfy user add --role=admin adminuser
```

### Configure Email Notifications

Edit `server.yml` with your SMTP settings:
```yaml
smtp-sender-addr: "smtp.gmail.com:587"
smtp-sender-from: "notifications@yourdomain.com"
smtp-sender-user: "your-email@gmail.com"
smtp-sender-pass: "your-app-password"
```

## 📊 Monitoring

### Health Check
```bash
curl https://your-ntfy-server.fly.dev/v1/health
```

### View Logs
```bash
flyctl logs
```

### Metrics (if enabled)
```bash
curl https://your-ntfy-server.fly.dev/metrics
```

## 🔐 Security

### Production Recommendations

1. **Enable Authentication**:
   ```yaml
   auth-default-access: "deny-all"
   ```

2. **Use HTTPS Only**:
   - Fly.io provides automatic TLS termination
   - Configure `force_https = true` in `fly.toml`

3. **Rate Limiting**:
   - Adjust `visitor-request-limit-burst` and `visitor-message-daily-limit`
   - Monitor usage patterns

4. **Access Control**:
   - Configure CORS origins appropriately
   - Use topic-based access control if needed

## 🛠️ Maintenance

### Update ntfy Version

1. Update the image tag in `fly.toml`:
   ```toml
   image = "binwiederhier/ntfy:v2.x.x"
   ```

2. Deploy:
   ```bash
   flyctl deploy
   ```

### Backup Data

```bash
flyctl ssh console
tar -czf /tmp/ntfy-backup.tar.gz /var/cache/ntfy
flyctl sftp get /tmp/ntfy-backup.tar.gz ./ntfy-backup.tar.gz
```

### Scale Resources

```bash
flyctl scale vm shared-cpu-1x --memory 512
```

## 📚 Usage

### Send Notifications

```bash
# Simple message
curl -d "Hello World" https://your-ntfy-server.fly.dev/mytopic

# With title and priority
curl -H "Title: Alert" -H "Priority: high" -d "Server is down!" https://your-ntfy-server.fly.dev/alerts

# JSON format
curl -H "Content-Type: application/json" -d '{"topic":"mytopic","message":"Hello","title":"Notification"}' https://your-ntfy-server.fly.dev/
```

### Subscribe to Topics

```bash
# Command line
curl -s https://your-ntfy-server.fly.dev/mytopic/json

# JavaScript
fetch('https://your-ntfy-server.fly.dev/mytopic/json')
  .then(response => response.json())
  .then(data => console.log(data));
```

## 🆘 Troubleshooting

### Common Issues

1. **App won't start**: Check logs with `flyctl logs`
2. **Volume mount issues**: Ensure volume exists and is in the same region
3. **Health check failures**: Verify port 8080 is accessible
4. **Configuration errors**: Validate `server.yml` syntax

### Debug Commands

```bash
# SSH into container
flyctl ssh console

# Check disk usage
df -h

# Test ntfy config
ntfy serve --config /etc/ntfy/server.yml --dry-run

# View container logs
flyctl logs --app your-ntfy-server
```

## 🔗 Resources

- [ntfy Documentation](https://docs.ntfy.sh/)
- [Fly.io Documentation](https://fly.io/docs/)
- [ntfy GitHub Repository](https://github.com/binwiederhier/ntfy)
- [Fly.io Pricing](https://fly.io/docs/about/pricing/)