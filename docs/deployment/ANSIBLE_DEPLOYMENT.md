# Ansible Deployment Guide

Deploy Station to remote servers via SSH using Ansible playbooks. This guide covers all deployment modes and troubleshooting.

## Overview

The Ansible deployment target supports three image strategies:

| Strategy | Image Source | Bundle Delivery | Use Case |
|----------|--------------|-----------------|----------|
| **Base Image + Bundle ID** | `ghcr.io/cloudshipai/station:latest` | CloudShip download at runtime | Simplest, recommended |
| **Base Image + Local Bundle** | `ghcr.io/cloudshipai/station:latest` | Copy `.tar.gz` to remote | No CloudShip account |
| **Custom Image** | Your registry | Baked into image | Full control, CI/CD |

## Prerequisites

1. **Ansible installed**: `pip install ansible`
2. **SSH access** to target hosts (key-based recommended)
3. **Docker** on target hosts (playbook can install it)
4. **For custom images**: Container registry access

## Quick Start

### Option 1: Base Image + Bundle ID (Recommended)

Upload your bundle to CloudShip first, then deploy:

```bash
# 1. Create and upload bundle
stn bundle create my-env --output my-bundle.tar.gz
stn bundle share my-bundle.tar.gz
# Returns: Bundle ID e26b414a-f076-4135-927f-810bc1dc892a

# 2. Deploy with bundle ID
stn deploy --bundle-id e26b414a-f076-4135-927f-810bc1dc892a \
  --target ansible \
  --hosts "ubuntu@192.168.1.100" \
  --ssh-key ~/.ssh/id_rsa
```

### Option 2: Base Image + Local Bundle File

Deploy from a local environment, Station uses the base image and copies configuration:

```bash
# Deploy local environment (uses base image automatically)
stn deploy my-env \
  --target ansible \
  --hosts "ubuntu@192.168.1.100" \
  --ssh-key ~/.ssh/id_rsa
```

### Option 3: Custom Image (Registry Required)

Build and push your image first, then reference it:

```bash
# 1. Build custom image with bundle baked in
stn build env my-env --output my-station:v1.0

# 2. Push to registry
docker tag my-station:v1.0 ghcr.io/myorg/station:v1.0
docker push ghcr.io/myorg/station:v1.0

# 3. Generate Ansible configs (dry-run)
stn deploy my-env --target ansible --dry-run --output-dir ./ansible-deploy

# 4. Edit vars/main.yml to use your image
#    docker_image: ghcr.io/myorg/station:v1.0

# 5. Run playbook manually
ansible-playbook -i ./ansible-deploy/inventory.ini ./ansible-deploy/playbook.yml
```

## Command Reference

```bash
stn deploy <environment> --target ansible [flags]

# Or with bundle ID
stn deploy --bundle-id <id> --target ansible [flags]

Flags:
  --hosts strings      Target hosts (user@host format, comma-separated)
  --ssh-key string     Path to SSH private key
  --ssh-user string    SSH username (overrides user in --hosts)
  --output-dir string  Output directory for generated configs
  --dry-run            Generate configs only, don't deploy
  --secrets-from       External secrets provider URI
```

### Examples

```bash
# Single host
stn deploy prod --target ansible --hosts "ubuntu@10.0.0.5" --ssh-key ~/.ssh/prod_key

# Multiple hosts
stn deploy prod --target ansible \
  --hosts "ubuntu@10.0.0.5,ubuntu@10.0.0.6,ubuntu@10.0.0.7" \
  --ssh-key ~/.ssh/prod_key

# Dry run (generate configs only)
stn deploy prod --target ansible --dry-run --output-dir ./ansible-configs

# With external secrets
stn deploy prod --target ansible \
  --hosts "ubuntu@10.0.0.5" \
  --ssh-key ~/.ssh/prod_key \
  --secrets-from vault://secret/data/station/prod
```

## Generated Files

When you run the deploy command, it generates:

```
ansible-deploy/
├── inventory.ini              # Target hosts
├── playbook.yml               # Main playbook
├── vars/
│   └── main.yml               # Configuration variables
└── templates/
    ├── docker-compose.yml.j2  # Docker Compose template
    └── station.service.j2     # Systemd service template
```

### inventory.ini

```ini
[station_servers]
192.168.1.100 ansible_user=ubuntu ansible_ssh_private_key_file=/path/to/key

[station_servers:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_ssh_common_args='-o StrictHostKeyChecking=no'
```

### vars/main.yml

```yaml
station_name: "my-env"
station_install_dir: "/opt/station/my-env"
station_data_dir: "/opt/station/my-env/data"
docker_image: "ghcr.io/cloudshipai/station:latest"
station_api_port: "8586"
station_mcp_port: "8587"

# Environment variables for container
station_env_vars:
  STN_AI_PROVIDER: anthropic
  STN_AI_MODEL: claude-sonnet-4-20250514
  # ... more vars
```

## Understanding Image Strategies

### Why Custom Images Don't Work Without a Registry

When you build a Docker image locally:

```bash
stn build env my-env --output my-station:v1.0
# Creates: my-station:v1.0 (LOCAL ONLY)
```

This image exists **only on your machine**. When Ansible runs on the remote host:

```yaml
- name: Pull Station image
  docker_image:
    name: "{{ docker_image }}"
    source: pull
```

The remote Docker daemon tries to pull `my-station:v1.0` but can't find it anywhere!

### Solution: Push to a Registry

```bash
# Tag for your registry
docker tag my-station:v1.0 ghcr.io/myorg/station:v1.0

# Authenticate (GitHub Container Registry example)
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Push
docker push ghcr.io/myorg/station:v1.0
```

Now the remote host can pull from `ghcr.io/myorg/station:v1.0`.

### Base Image Strategy (Default)

The base image `ghcr.io/cloudshipai/station:latest` is publicly available. Configuration is passed via:

1. **Environment variables** - AI config, CloudShip config
2. **Bundle ID** - Downloaded at runtime from CloudShip
3. **Volume mount** - For persistent data

This is why `stn deploy` now defaults to the base image for Ansible.

## Private Registry Configuration

If using a private registry, configure Docker login on target hosts:

### Option 1: Pre-configure hosts

SSH to each host and login:

```bash
docker login your-registry.com -u username -p password
```

### Option 2: Add to playbook

Edit `playbook.yml` to include registry auth:

```yaml
- name: Login to private registry
  docker_login:
    registry_url: "your-registry.com"
    username: "{{ registry_username }}"
    password: "{{ registry_password }}"
  no_log: true

- name: Pull Station image
  docker_image:
    name: "{{ docker_image }}"
    source: pull
```

Add credentials to `vars/main.yml`:

```yaml
registry_username: "myuser"
registry_password: "{{ lookup('env', 'REGISTRY_PASSWORD') }}"
```

## Customizing the Playbook

### Add Custom Tasks

Edit `playbook.yml` to add pre/post deployment tasks:

```yaml
- name: Pre-deployment backup
  shell: |
    if [ -d "{{ station_install_dir }}" ]; then
      tar -czf /tmp/station-backup-$(date +%Y%m%d).tar.gz {{ station_install_dir }}
    fi

# ... existing tasks ...

- name: Post-deployment health check
  uri:
    url: "http://localhost:{{ station_mcp_port }}/health"
    method: GET
    return_content: yes
  register: health_check
  retries: 10
  delay: 5
  until: health_check.status == 200
```

### Add Firewall Rules

```yaml
- name: Configure UFW firewall
  ufw:
    rule: allow
    port: "{{ item }}"
    proto: tcp
  loop:
    - "{{ station_api_port }}"
    - "{{ station_mcp_port }}"
  when: ansible_os_family == "Debian"
```

### Add Monitoring

```yaml
- name: Configure health check cron
  cron:
    name: "Station health check"
    minute: "*/5"
    job: "curl -sf http://localhost:{{ station_mcp_port }}/health || systemctl restart station-{{ station_name }}"
```

## Troubleshooting

### SSH Connection Failures

```bash
# Test connectivity
ansible -i inventory.ini all -m ping

# Debug SSH
ssh -vvv -i /path/to/key user@host
```

Common issues:
- Wrong SSH key permissions (`chmod 600 ~/.ssh/key`)
- Host key verification (use `StrictHostKeyChecking=no` for automation)
- Firewall blocking port 22

### Docker Pull Failures

```
Error: manifest for my-station:v1.0 not found
```

**Cause**: Image doesn't exist in any registry the remote host can access.

**Solutions**:
1. Use base image: `ghcr.io/cloudshipai/station:latest`
2. Push your image to a registry
3. Build image on remote host (not recommended)

### Container Won't Start

```bash
# Check on remote host
ssh user@host
docker ps -a
docker logs station-my-env
```

Common issues:
- Missing environment variables (AI API key)
- Port already in use
- Permission denied on data directory

### Health Check Fails

```bash
# Check container health
docker inspect station-my-env | jq '.[0].State.Health'

# Check logs
docker logs station-my-env --tail 50

# Test health endpoint manually
curl http://localhost:8587/health
```

## CI/CD Integration

### GitHub Actions

```yaml
name: Deploy to Production

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Install Station CLI
        run: curl -fsSL https://get.station.dev | bash
      
      - name: Setup SSH
        run: |
          mkdir -p ~/.ssh
          echo "${{ secrets.SSH_PRIVATE_KEY }}" > ~/.ssh/deploy_key
          chmod 600 ~/.ssh/deploy_key
          ssh-keyscan -H ${{ secrets.PROD_HOST }} >> ~/.ssh/known_hosts
      
      - name: Deploy
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
        run: |
          stn deploy production --target ansible \
            --hosts "${{ secrets.PROD_USER }}@${{ secrets.PROD_HOST }}" \
            --ssh-key ~/.ssh/deploy_key
```

### GitLab CI

```yaml
deploy-production:
  stage: deploy
  image: ghcr.io/cloudshipai/station:latest
  before_script:
    - pip install ansible
    - mkdir -p ~/.ssh
    - echo "$SSH_PRIVATE_KEY" > ~/.ssh/deploy_key
    - chmod 600 ~/.ssh/deploy_key
  script:
    - stn deploy production --target ansible \
        --hosts "$PROD_USER@$PROD_HOST" \
        --ssh-key ~/.ssh/deploy_key
  only:
    - main
```

## Security Best Practices

1. **Use SSH keys, not passwords**
2. **Store secrets in vault** (AWS Secrets Manager, HashiCorp Vault)
3. **Restrict SSH access** (firewall, fail2ban)
4. **Run Station as non-root user**
5. **Use private registries** for custom images
6. **Rotate credentials** regularly
7. **Enable TLS** for external access (reverse proxy)

## Reference: Full Command Flow

```bash
# What happens when you run:
stn deploy my-env --target ansible --hosts "ubuntu@10.0.0.5" --ssh-key ~/.ssh/key

# 1. Validates Ansible is installed
# 2. Loads environment config from ~/.config/station/environments/my-env/
# 3. Detects AI config (provider, model, API key/OAuth)
# 4. Detects CloudShip config (if enabled)
# 5. Generates Ansible files to temp directory (or --output-dir)
# 6. Runs ansible-playbook in check mode first
# 7. Prompts for confirmation
# 8. Runs ansible-playbook to deploy
# 9. Station container starts on remote host
# 10. Health check confirms deployment success
```
