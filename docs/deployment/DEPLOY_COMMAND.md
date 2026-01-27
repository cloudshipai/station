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

Deploy Station to remote servers via SSH + Docker using Ansible playbooks.

> **Full Documentation**: See [ANSIBLE_DEPLOYMENT.md](./ANSIBLE_DEPLOYMENT.md) for complete guide including image strategies, private registries, and troubleshooting.

### Prerequisites

1. Ansible installed: `pip install ansible`
2. SSH access to target hosts (key-based recommended)
3. Docker on target hosts (playbook can install it)

### Quick Deploy (Recommended)

The simplest approach uses the base Station image with your local config:

```bash
# Deploy to a single host
stn deploy my-env --target ansible \
  --hosts "ubuntu@192.168.1.100" \
  --ssh-key ~/.ssh/id_rsa

# Deploy to multiple hosts
stn deploy my-env --target ansible \
  --hosts "ubuntu@10.0.0.5,ubuntu@10.0.0.6" \
  --ssh-key ~/.ssh/prod_key
```

### Deploy with Bundle ID

If you've uploaded a bundle to CloudShip:

```bash
stn deploy --bundle-id e26b414a-f076-4135-927f-810bc1dc892a \
  --target ansible \
  --hosts "ubuntu@192.168.1.100" \
  --ssh-key ~/.ssh/id_rsa
```

### Ansible-Specific Flags

| Flag | Description | Example |
|------|-------------|---------|
| `--hosts` | Target hosts (user@host format) | `ubuntu@10.0.0.5,root@10.0.0.6` |
| `--ssh-key` | Path to SSH private key | `~/.ssh/id_rsa` |
| `--ssh-user` | Override SSH user for all hosts | `ubuntu` |
| `--output-dir` | Output directory for configs | `./ansible-deploy` |
| `--dry-run` | Generate configs only | (flag) |

### Generate Configs Only (Dry Run)

```bash
# Generate Ansible playbook without deploying
stn deploy my-env --target ansible --dry-run --output-dir ./ansible-deploy

# Review and customize generated files
ls ./ansible-deploy/
# inventory.ini  playbook.yml  templates/  vars/

# Then run manually
ansible-playbook -i ./ansible-deploy/inventory.ini ./ansible-deploy/playbook.yml
```

### Using Custom Images

If you need a custom image (with specific tools or configurations baked in):

```bash
# 1. Build and push to a registry
stn build env my-env --output my-station:v1.0
docker tag my-station:v1.0 ghcr.io/myorg/station:v1.0
docker push ghcr.io/myorg/station:v1.0

# 2. Generate configs (dry-run)
stn deploy my-env --target ansible --dry-run --output-dir ./ansible-deploy

# 3. Edit vars/main.yml to use your image
#    docker_image: ghcr.io/myorg/station:v1.0

# 4. Run playbook
ansible-playbook -i ./ansible-deploy/inventory.ini ./ansible-deploy/playbook.yml
```

> **Why can't I use a locally-built image directly?** The remote host needs to `docker pull` the image, but locally-built images only exist on your machine. You must push to a registry (Docker Hub, GHCR, private registry) first. See [ANSIBLE_DEPLOYMENT.md](./ANSIBLE_DEPLOYMENT.md#understanding-image-strategies) for details.

### Post-Deployment

```bash
# Check status on remote host
ssh ubuntu@192.168.1.100 "docker ps && curl http://localhost:8587/health"

# View systemd service status
ssh ubuntu@192.168.1.100 "sudo systemctl status station-my-env"

# View logs
ssh ubuntu@192.168.1.100 "docker logs station-my-env --tail 50"
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

# General Flags
--target string      Deployment target: fly, kubernetes/k8s, ansible (default: fly)
--bundle-id string   CloudShip bundle ID (no local environment needed)
--bundle string      Local bundle file (.tar.gz)
--name string        Custom app name
--output-dir string  Output directory for generated configs
--dry-run            Generate configs only, don't deploy
--destroy            Tear down deployment
--secrets-from       Secret provider URI

# Fly.io Flags
--region string      Deployment region (default: ord)
--auto-stop          Enable auto-stop when idle

# Kubernetes Flags
--namespace string   Kubernetes namespace
--context string     Kubernetes context

# Ansible Flags
--hosts strings      Target hosts (user@host format, comma-separated)
--ssh-key string     Path to SSH private key
--ssh-user string    SSH username (overrides user in --hosts)

# Examples
stn deploy default --target fly
stn deploy --bundle-id abc123 --target k8s --namespace prod
stn deploy default --target ansible --hosts "ubuntu@10.0.0.5" --ssh-key ~/.ssh/key
stn deploy --bundle ./bundle.tar.gz --target ansible --dry-run --output-dir ./configs
```
