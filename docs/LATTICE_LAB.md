# Lattice Lab Tutorial

A hands-on guide to deploying Station Lattice across multiple VMs using Vagrant and Ansible.

## Overview

This tutorial walks through setting up a real distributed agent mesh with:
- **Orchestrator VM**: Runs embedded NATS server (the hub)
- **Member VM**: Runs Station with agents that register to the lattice

By the end, you'll have agents discoverable and executable across the network.

## Prerequisites

- Vagrant with libvirt provider (or VirtualBox)
- Ansible
- Station CLI (`stn`) built locally
- An existing Station environment with agents (we'll bundle these)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Host Machine (CLI Client)                                   │
│  stn lattice --nats nats://192.168.56.10:4222 ...           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Orchestrator VM (192.168.56.10)                            │
│  Station + Embedded NATS Server (port 4222)                 │
│  - Acts as NATS hub for lattice                             │
│  - Monitoring on port 8222                                  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Member VM (192.168.56.11)                                  │
│  Station (lattice member mode)                              │
│  - Connected to nats://192.168.56.10:4222                   │
│  - Hosts agents from your bundle                            │
│  - Registered in lattice registry                           │
└─────────────────────────────────────────────────────────────┘
```

## Step 1: Create Working Directory

```bash
mkdir -p /tmp/lattice-lab
cd /tmp/lattice-lab
```

## Step 2: Create Vagrantfile

Create `vagrant/Vagrantfile`:

```ruby
Vagrant.configure("2") do |config|
  config.vm.box = "generic/ubuntu2204"
  
  # Orchestrator - runs embedded NATS
  config.vm.define "orchestrator" do |node|
    node.vm.hostname = "station-orchestrator"
    node.vm.network "private_network", ip: "192.168.56.10"
    node.vm.provider "libvirt" do |v|
      v.memory = 2048
      v.cpus = 2
    end
    node.vm.provision "docker"
  end

  # Member 1 - runs agents
  config.vm.define "member1" do |node|
    node.vm.hostname = "station-member1"
    node.vm.network "private_network", ip: "192.168.56.11"
    node.vm.provider "libvirt" do |v|
      v.memory = 2048
      v.cpus = 2
    end
    node.vm.provision "docker"
  end
end
```

Start the VMs:

```bash
cd vagrant
vagrant up orchestrator member1
```

## Step 3: Create Agent Bundle

Bundle your existing environment (without secrets):

```bash
# From your station environment
stn bundle create default --output /tmp/lattice-lab/agent-bundle.tar.gz
```

> **Important**: Bundles use template variables like `{{.POSTHOG_API_KEY}}` instead of actual secrets.
> Verify no secrets are included:
> ```bash
> tar -tzf /tmp/lattice-lab/agent-bundle.tar.gz
> tar -xzf /tmp/lattice-lab/agent-bundle.tar.gz -O | grep -i "password\|secret\|api_key=" || echo "Clean!"
> ```

## Step 4: Deploy Orchestrator

### Generate Ansible Configs

```bash
stn deploy default --target ansible \
  --hosts vagrant@192.168.56.10 \
  --ssh-key /tmp/lattice-lab/vagrant/.vagrant/machines/orchestrator/libvirt/private_key \
  --output-dir /tmp/lattice-lab/ansible-orchestrator \
  --dry-run
```

### Configure Orchestrator Mode

Edit `ansible-orchestrator/vars/main.yml` to add lattice orchestration:

```yaml
station_env:
  STN_LATTICE_NATS_PORT: "4222"
  STN_LATTICE_NATS_HTTP_PORT: "8222"
  STN_LATTICE_STATION_NAME: "orchestrator"
  # ... other env vars from deploy command
```

Edit `ansible-orchestrator/templates/docker-compose.yml.j2` to expose NATS ports:

```yaml
services:
  station:
    # ... existing config
    ports:
      - "8586:8586"
      - "8587:8587"
      - "4222:4222"   # NATS client port
      - "8222:8222"   # NATS monitoring
```

### Create config.yaml for Orchestrator

The key insight: **`lattice_orchestration` must be set in config.yaml** because it's not bound to an env var.

Create `ansible-orchestrator/files/config.yaml`:

```yaml
lattice_orchestration: true
lattice_url: ""
```

Add a task to copy this file in your playbook before starting the container.

### Fix Playbook for Docker Compose v2

Modern systems use `docker compose` (plugin) not `docker-compose` (standalone). Update the playbook:

```yaml
- name: Check if Docker is installed
  command: docker --version
  register: docker_check
  ignore_errors: yes
  changed_when: false

- name: Ensure Docker is installed
  apt:
    name: [docker.io, docker-compose]
    state: present
  when: docker_check.rc != 0

- name: Pull Station image
  shell: docker pull {{ station_image }}
  register: pull_result
  changed_when: "'Downloaded' in pull_result.stdout"
```

### Run Ansible

```bash
cd /tmp/lattice-lab/ansible-orchestrator
ansible-playbook -i inventory.ini playbook.yml
```

### Verify Orchestrator

```bash
# Check container
vagrant ssh orchestrator -c "docker ps"

# Check NATS is running
curl http://192.168.56.10:8222/varz

# Check logs for lattice
vagrant ssh orchestrator -c "docker logs station-default 2>&1 | grep -i nats"
# Should see: "Embedded NATS started on port 4222"
```

## Step 5: Deploy Member Station

### Generate Ansible Configs

```bash
stn deploy default --target ansible \
  --hosts vagrant@192.168.56.11 \
  --ssh-key /tmp/lattice-lab/vagrant/.vagrant/machines/member1/libvirt/private_key \
  --output-dir /tmp/lattice-lab/ansible-member1 \
  --dry-run
```

### Configure Member Mode

Edit `ansible-member1/vars/main.yml`:

```yaml
station_env:
  STN_LATTICE_NATS_URL: "nats://192.168.56.10:4222"
  STN_LATTICE_STATION_NAME: "agent-member"
  # ... other env vars
```

### Create config.yaml for Member

**Critical**: The `lattice_url` config key activates member mode. It's read from config.yaml, not env vars.

Create `ansible-member1/files/config.yaml`:

```yaml
lattice_url: "nats://192.168.56.10:4222"
lattice_orchestration: false
```

### Add Bundle Installation

Add tasks to your playbook to install the agent bundle:

```yaml
- name: Copy bundle to station
  copy:
    src: /tmp/lattice-lab/agent-bundle.tar.gz
    dest: "{{ station_install_dir }}/bundle.tar.gz"

- name: Create environments directory
  file:
    path: "{{ station_data_dir }}/environments/default"
    state: directory
    owner: '1000'
    group: '1000'

- name: Extract bundle
  unarchive:
    src: "{{ station_install_dir }}/bundle.tar.gz"
    dest: "{{ station_data_dir }}/environments/default"
    remote_src: yes
    owner: '1000'
    group: '1000'
```

### Run Ansible

```bash
cd /tmp/lattice-lab/ansible-member1
ansible-playbook -i inventory.ini playbook.yml
```

### Verify Member Connected

```bash
# Check logs for lattice connection
vagrant ssh member1 -c "docker logs station-default 2>&1 | grep -i lattice"

# Should see:
# 🔗 Initializing Station Lattice mesh network...
# ✅ Lattice client mode: connecting to nats://192.168.56.10:4222
# ✅ Connected to lattice NATS
# ✅ Lattice presence heartbeat started
```

## Step 6: Test the Lattice

### Check Lattice Status

```bash
stn lattice --nats nats://192.168.56.10:4222 status
```

Output:
```
Lattice Status: MEMBER
Orchestrator: nats://192.168.56.10:4222
Connection:   Connected
```

### Discover Agents

```bash
stn lattice --nats nats://192.168.56.10:4222 agents
```

Output:
```
Agents in Lattice (1 total)
============================================
AGENT                    STATION              
--------------------------------------------
posthog-dashboard-reporter  agent-member
```

### Execute Remote Agent

```bash
stn lattice --nats nats://192.168.56.10:4222 agent exec posthog-dashboard-reporter "What can you do?"
```

The agent executes on the member VM and returns results through the lattice!

## Configuration Reference

### Config File Keys

| Key | Type | Description |
|-----|------|-------------|
| `lattice_orchestration` | bool | Enable orchestrator mode (embedded NATS) |
| `lattice_url` | string | NATS URL for member mode |
| `lattice.station_name` | string | Station name in lattice |
| `lattice.station_id` | string | Custom station ID |
| `lattice.nats.url` | string | Alternative NATS URL location |

### Environment Variables

| Variable | Maps To | Description |
|----------|---------|-------------|
| `STN_LATTICE_NATS_URL` | `lattice.nats.url` | NATS connection URL |
| `STN_LATTICE_STATION_NAME` | `lattice.station_name` | Station name |
| `STN_LATTICE_NATS_PORT` | `lattice.orchestrator.embedded_nats.port` | Embedded NATS port |
| `STN_LATTICE_NATS_HTTP_PORT` | `lattice.orchestrator.embedded_nats.http_port` | NATS monitoring port |

> **Gotcha**: `lattice_url` and `lattice_orchestration` are only bound to CLI flags (`--lattice` and `--orchestration`),
> not environment variables. For container deployments, you must set these in config.yaml.

## Troubleshooting

### "Not connected to lattice"

The station didn't enter lattice mode. Check:
1. `config.yaml` has `lattice_url` or `lattice_orchestration` set
2. The config file is mounted into the container at `/home/station/.config/station/config.yaml`

### Agents not appearing

1. Check member logs for "Loading N agents from environment"
2. Verify bundle was extracted to `environments/default/`
3. Check presence heartbeat: `grep presence` in logs

### NATS connection refused

1. Verify orchestrator is running: `curl http://192.168.56.10:8222/varz`
2. Check firewall allows port 4222
3. Verify NATS ports are exposed in docker-compose

### Docker Compose v1 vs v2

If you see `docker-compose: command not found`:
- Modern Docker uses `docker compose` (space, not hyphen)
- Update systemd service to use `docker compose up/down`
- Or install docker-compose standalone: `apt install docker-compose`

## Example Files

Complete example files are available in `examples/lattice-lab/`:

```
examples/lattice-lab/
├── README.md
├── vagrant/
│   └── Vagrantfile
├── ansible-orchestrator/
│   ├── inventory.ini
│   ├── playbook.yml
│   ├── vars/main.yml
│   ├── files/config.yaml
│   └── templates/docker-compose.yml.j2
└── ansible-member/
    ├── inventory.ini
    ├── playbook.yml
    ├── vars/main.yml
    ├── files/config.yaml
    └── templates/docker-compose.yml.j2
```

## Lab 8: Centralized NATS Deployment

For production deployments, you may want to run NATS separately from Station instances. This provides:
- Independent scaling of NATS infrastructure
- Dedicated NATS cluster management
- Easier NATS upgrades without touching Station
- Support for existing NATS infrastructure

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Host Machine (CLI Client)                                   │
│  stn lattice --nats nats://192.168.56.9:4222 ...            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  NATS Server VM (192.168.56.9)                              │
│  Standalone NATS with JetStream                             │
│  - Client port: 4222                                        │
│  - Monitoring: 8222                                         │
│  - Optional: Auth enabled                                   │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
┌─────────────────────────┐     ┌─────────────────────────┐
│  Orchestrator VM        │     │  Member VM              │
│  (192.168.56.10)        │     │  (192.168.56.11)        │
│  Station (lattice mode) │     │  Station (lattice mode) │
│  - No embedded NATS     │     │  - Hosts agents         │
│  - Connects as client   │     │  - Connects as client   │
└─────────────────────────┘     └─────────────────────────┘
```

### Step 1: Start NATS Server VM

```bash
cd /tmp/lattice-lab/vagrant
vagrant up nats
```

### Step 2: Deploy Standalone NATS

```bash
cd /tmp/lattice-lab/ansible-nats
ansible-playbook -i inventory.ini playbook.yml
```

Verify NATS is running:
```bash
curl http://192.168.56.9:8222/varz
```

### Step 3: Deploy Stations (Centralized Mode)

For the orchestrator (no embedded NATS):
```bash
cd /tmp/lattice-lab/ansible-orchestrator-centralized
ansible-playbook -i inventory.ini playbook.yml
```

The key config difference (`files/config.yaml`):
```yaml
lattice_orchestration: false
lattice_url: "nats://192.168.56.9:4222"
```

For members, use the same `lattice_url` pointing to the NATS server.

### Step 4: Verify Connectivity

```bash
# Check lattice status (all stations connected to central NATS)
stn lattice --nats nats://192.168.56.9:4222 status

# List agents from all stations
stn lattice --nats nats://192.168.56.9:4222 agents
```

### Enabling Authentication

For production, enable NATS authentication:

1. Edit `ansible-nats/vars/main.yml`:
```yaml
nats_auth_enabled: true
nats_auth_token: "my-secret-lattice-token"
```

2. Redeploy NATS:
```bash
ansible-playbook -i inventory.ini playbook.yml
```

3. Update Station configs to include auth:
```yaml
lattice_url: "nats://my-secret-lattice-token@192.168.56.9:4222"
```

Or with environment variables:
```bash
STN_LATTICE_NATS_URL="nats://my-secret-lattice-token@192.168.56.9:4222"
```

### Example Files

Complete centralized NATS example files:
```
examples/lattice-lab/
├── ansible-nats/
│   ├── inventory.ini
│   ├── playbook.yml
│   ├── vars/main.yml
│   ├── vars/main-with-auth.yml
│   └── templates/
│       ├── nats.conf.j2
│       └── docker-compose.yml.j2
└── ansible-orchestrator-centralized/
    ├── inventory.ini
    ├── playbook.yml
    ├── vars/main.yml
    ├── files/config.yaml
    └── templates/docker-compose.yml.j2
```

---

## Next Steps

- Add more member stations with different agent bundles
- Configure NATS authentication for production
- Enable TLS for secure communication
- Set up monitoring with the TUI dashboard: `stn lattice dashboard`
- Scale NATS with clustering for high availability

---

*See also: [LATTICE.md](./LATTICE.md) for full architecture documentation*
