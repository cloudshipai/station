# Station Deploy Command

Deploy Station environments to cloud platforms with a single command. Supports Fly.io, Kubernetes, and Ansible (SSH + Docker) targets.

## Overview

The `stn deploy` command provides three deployment methods:

| Method | Use Case | Container Registry Required |
|--------|----------|---------------------------|
| Local Environment | Deploy from local config | Yes (for K8s/Ansible) |
| Bundle ID | Deploy CloudShip bundle | No (uses base image) |
| Local Bundle | Deploy local .tar.gz | Yes (for K8s/Ansible) |

## Deployment Targets

| Target | Description | Prerequisites |
|--------|-------------|---------------|
| `fly` | Fly.io (default) | `flyctl` CLI, Fly.io account |
| `kubernetes` / `k8s` | Kubernetes cluster | `kubectl`, cluster access |
| `ansible` | SSH + Docker deployment | `ansible`, SSH access to hosts |

## Quick Start

```bash
# Deploy to Fly.io (simplest)
stn deploy default --target fly

# Deploy CloudShip bundle (no local environment needed)
stn deploy --bundle-id e26b414a-f076-4135-927f-810bc1dc892a --target fly

# Deploy local bundle file
stn deploy --bundle ./my-bundle.tar.gz --target fly
```

## Container Registry Requirement

**Important**: For Kubernetes and Ansible deployments, you need a container registry to push the built image.

| Target | Registry Required | Why |
|--------|-------------------|-----|
| Fly.io | No | Fly.io has built-in registry |
| Kubernetes | Yes | K8s pulls from external registry |
| Ansible | Yes | Docker pull on remote host |

For K8s/Ansible, push your image to a registry:
```bash
# Build and tag
docker build -t your-registry.com/station:v1.0 .

# Push
docker push your-registry.com/station:v1.0

# Then update generated configs with your image
```

---

## Fly.io Deployment

Fly.io is the recommended target for quick production deployments. It handles container registry, SSL, and persistent storage automatically.

### Prerequisites

1. Install Fly CLI: `curl -L https://fly.io/install.sh | sh`
2. Login: `fly auth login`
3. Configure AI provider in Station config

### Deploy Local Environment

```bash
# Deploy default environment
stn deploy default --target fly

# Deploy with auto-stop (saves cost when idle)
stn deploy default --target fly --auto-stop

# Deploy to specific region
stn deploy default --target fly --region syd
```

### Deploy CloudShip Bundle

No local environment needed - uses the base Station image:

```bash
# Deploy bundle by ID
stn deploy --bundle-id e26b414a-f076-4135-927f-810bc1dc892a --target fly

# With custom app name
stn deploy --bundle-id e26b414a-f076-4135-927f-810bc1dc892a --target fly --name my-station
```

### Deploy Local Bundle File

```bash
# Create bundle from environment
stn bundle create default --output ./my-bundle.tar.gz

# Deploy the bundle
stn deploy --bundle ./my-bundle.tar.gz --target fly
```

### What Gets Created

- Fly.io app with persistent volume
- Secrets from your Station config (AI keys, CloudShip config)
- Public IPv4 address
- SSL certificate (automatic via Fly.io)

### Post-Deployment

```bash
# Check status
fly status --app your-app-name

# View logs
fly logs --app your-app-name

# SSH into container
fly ssh console --app your-app-name

# Destroy deployment
stn deploy default --target fly --destroy
```

### Access Points

After deployment:
- MCP Endpoint: `https://your-app.fly.dev/mcp`
- Health Check: `https://your-app.fly.dev/health`

---

## Kubernetes Deployment

Generate Kustomize manifests for Kubernetes deployment.

### Prerequisites

1. `kubectl` configured with cluster access
2. Container registry for your image (see below)
3. AI provider credentials

### Container Registry Setup

Kubernetes requires pulling images from a registry. Options:

**Docker Hub:**
```bash
# Build and push
docker build -t your-username/station:v1 .
docker push your-username/station:v1

# Update generated deployment.yaml with your image
```

**GitHub Container Registry:**
```bash
docker build -t ghcr.io/your-org/station:v1 .
docker push ghcr.io/your-org/station:v1
```

**Private Registry:**
```bash
# Create image pull secret
kubectl create secret docker-registry regcred \
  --docker-server=your-registry.com \
  --docker-username=user \
  --docker-password=pass

# Add to deployment spec:
# imagePullSecrets:
#   - name: regcred
```

### Generate Manifests (Dry Run)

```bash
# Generate manifests without deploying
stn deploy default --target k8s --dry-run --output-dir ./k8s-manifests

# Preview what will be created
ls ./k8s-manifests/
# deployment.yaml  ingress.yaml  kustomization.yaml  namespace.yaml  pvc.yaml  secret.yaml  service.yaml
```

### Deploy to Cluster

```bash
# Generate and apply directly
stn deploy default --target k8s

# Or apply generated manifests
kubectl apply -k ./k8s-manifests/
```

### Namespace and Context

```bash
# Deploy to specific namespace
stn deploy default --target k8s --namespace production

# Use specific kubectl context
stn deploy default --target k8s --context my-cluster-context
```

### Generated Resources

| File | Description |
|------|-------------|
| `namespace.yaml` | Namespace (if specified) |
| `secret.yaml` | AI keys, CloudShip config |
| `deployment.yaml` | Station container spec |
| `service.yaml` | ClusterIP service |
| `ingress.yaml` | Ingress with TLS |
| `pvc.yaml` | Persistent volume claim |
| `kustomization.yaml` | Kustomize manifest |

### Update Image Reference

After pushing to your registry, update `deployment.yaml`:

```yaml
spec:
  template:
    spec:
      containers:
      - name: station
        image: your-registry.com/station:v1  # Update this
```

### Post-Deployment

```bash
# Check pods
kubectl get pods -n your-namespace

# View logs
kubectl logs -l app=station -n your-namespace

# Port forward for testing
kubectl port-forward svc/station 8586:8586 -n your-namespace

# Access MCP endpoint
curl http://localhost:8586/health
```

---

## Ansible Deployment

Generate Ansible playbooks for deploying Station to remote servers via SSH + Docker.

### Prerequisites

1. Ansible installed: `pip install ansible`
2. SSH access to target hosts
3. Docker on target hosts (or playbook will install it)
4. Container registry for your image

### Container Registry Setup

Same as Kubernetes - push your image first:

```bash
docker build -t your-registry.com/station:v1 .
docker push your-registry.com/station:v1
```

### Generate Playbook (Dry Run)

```bash
# Generate Ansible playbook
stn deploy default --target ansible --dry-run --output-dir ./ansible-deploy

# Review generated files
ls ./ansible-deploy/
# inventory.ini  playbook.yml  templates/  vars/
```

### Configure Inventory

Edit `inventory.ini` with your target hosts:

```ini
[station_servers]
server1.example.com ansible_user=ubuntu
server2.example.com ansible_user=ubuntu ansible_ssh_private_key_file=~/.ssh/mykey

[station_servers:vars]
ansible_python_interpreter=/usr/bin/python3
```

### Configure Variables

Edit `vars/main.yml`:

```yaml
station_install_dir: /opt/station
station_data_dir: /opt/station/data
docker_image: your-registry.com/station:v1  # Update this
station_name: my-production-station
```

### Run Playbook

```bash
# Deploy to all hosts
ansible-playbook -i ./ansible-deploy/inventory.ini ./ansible-deploy/playbook.yml

# Deploy to specific host
ansible-playbook -i ./ansible-deploy/inventory.ini ./ansible-deploy/playbook.yml --limit server1.example.com

# Dry run (check mode)
ansible-playbook -i ./ansible-deploy/inventory.ini ./ansible-deploy/playbook.yml --check
```

### Or Deploy Directly

```bash
# Generate and run playbook immediately
stn deploy default --target ansible
```

### Generated Files

| File | Description |
|------|-------------|
| `inventory.ini` | Target hosts (edit this!) |
| `playbook.yml` | Main deployment playbook |
| `vars/main.yml` | Configuration variables |
| `templates/docker-compose.yml.j2` | Docker Compose template |
| `templates/station.service.j2` | Systemd service file |

### What the Playbook Does

1. Installs Docker (if not present)
2. Creates Station directories
3. Deploys docker-compose configuration
4. Sets up systemd service
5. Starts Station container
6. Configures auto-restart

### Post-Deployment

```bash
# SSH to server and check status
ssh user@server1.example.com
sudo systemctl status station-your-env

# View logs
sudo journalctl -u station-your-env -f

# Docker logs
sudo docker logs station-your-env
```

---

## Bundle Management

### Export Required Variables

Before deploying, see what secrets you'll need:

```bash
# From local bundle
stn bundle export-vars ./my-bundle.tar.gz --format yaml

# From CloudShip bundle
stn bundle export-vars e26b414a-f076-4135-927f-810bc1dc892a --format yaml

# Output as .env file
stn bundle export-vars ./my-bundle.tar.gz --format env > .env.deploy
```

Example output:
```yaml
_comment: Replace ***MASKED*** and ***REQUIRED*** values with actual secrets.
bundle_variables:
  GITHUB_TOKEN: '***REQUIRED***'
  AWS_ACCESS_KEY_ID: '***REQUIRED***'
deployment_variables:
  STN_AI_PROVIDER: anthropic
  STN_AI_API_KEY: '***MASKED***'
  STN_CLOUDSHIP_KEY: '***MASKED***'
```

### Export Environment Variables

Export variables from an environment for CI/CD:

```bash
# YAML format
stn deploy export-vars default --format yaml > deploy-vars.yml

# .env format
stn deploy export-vars default --format env > .env.deploy
```

---

## Secret Providers

Pull secrets from external stores during deployment:

```bash
# AWS Secrets Manager
stn deploy default --target k8s --secrets-from aws-secretsmanager://station-prod

# AWS SSM Parameter Store
stn deploy default --target k8s --secrets-from aws-ssm:///station/prod/

# HashiCorp Vault
stn deploy default --target fly --secrets-from vault://secret/data/station/prod

# Google Secret Manager
stn deploy default --target k8s --secrets-from gcp-secretmanager://projects/myproj/secrets/station

# SOPS encrypted file
stn deploy default --target ansible --secrets-from sops://./secrets.enc.yaml
```

---

## CI/CD Integration

### GitHub Actions - Fly.io

```yaml
name: Deploy to Fly.io
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

      - name: Install Fly CLI
        run: curl -L https://fly.io/install.sh | sh

      - name: Deploy
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}
          STN_AI_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
        run: stn deploy default --target fly
```

### GitHub Actions - Kubernetes

```yaml
name: Deploy to Kubernetes
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

      - name: Configure kubectl
        uses: azure/k8s-set-context@v3
        with:
          kubeconfig: ${{ secrets.KUBECONFIG }}

      - name: Build and Push Image
        run: |
          docker build -t ghcr.io/${{ github.repository }}/station:${{ github.sha }} .
          docker push ghcr.io/${{ github.repository }}/station:${{ github.sha }}

      - name: Deploy
        run: |
          stn deploy default --target k8s --dry-run --output-dir ./k8s
          # Update image in deployment
          sed -i "s|station:latest|ghcr.io/${{ github.repository }}/station:${{ github.sha }}|g" ./k8s/deployment.yaml
          kubectl apply -k ./k8s/
```

---

## Troubleshooting

### Fly.io Issues

```bash
# Check app status
fly status --app your-app

# View recent logs
fly logs --app your-app

# SSH into container
fly ssh console --app your-app

# Restart app
fly apps restart your-app
```

### Kubernetes Issues

```bash
# Check pod status
kubectl get pods -l app=station

# Describe pod for events
kubectl describe pod station-xxx

# View logs
kubectl logs -l app=station --tail=100

# Check secrets
kubectl get secret station-secrets -o yaml
```

### Ansible Issues

```bash
# Test connectivity
ansible -i inventory.ini all -m ping

# Verbose playbook run
ansible-playbook playbook.yml -vvv

# Check Docker on target
ssh user@host "docker ps"
```

---

## Command Reference

```bash
# Full command syntax
stn deploy [environment] [flags]

# Flags
--target string      Deployment target: fly, kubernetes/k8s, ansible (default: fly)
--bundle-id string   CloudShip bundle ID (no local environment needed)
--bundle string      Local bundle file (.tar.gz)
--name string        Custom app name
--region string      Deployment region (default: ord)
--namespace string   Kubernetes namespace
--context string     Kubernetes context
--output-dir string  Output directory for generated configs
--dry-run           Generate configs only, don't deploy
--auto-stop         Enable auto-stop when idle (Fly.io)
--destroy           Tear down deployment
--secrets-from      Secret provider URI

# Examples
stn deploy default --target fly
stn deploy --bundle-id abc123 --target k8s --namespace prod
stn deploy --bundle ./bundle.tar.gz --target ansible --dry-run
```
