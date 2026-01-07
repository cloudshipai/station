# Lattice Lab Example

Example files for deploying Station Lattice across VMs using Vagrant and Ansible.

## Deployment Modes

### Mode 1: Embedded NATS (Default)
Orchestrator runs embedded NATS server. Simplest setup.

### Mode 2: Centralized NATS
Separate NATS server, all stations connect as clients. Better for production.

## Prerequisites

- Vagrant with libvirt or VirtualBox provider
- Ansible
- Station CLI (`stn`)

## Quick Start (Embedded NATS)

```bash
cd vagrant
vagrant up orchestrator member1

cd ../ansible-orchestrator
ansible-playbook -i inventory.ini playbook.yml

cd ../ansible-member
ansible-playbook -i inventory.ini playbook.yml

stn lattice --nats nats://192.168.56.10:4222 agents
```

## Quick Start (Centralized NATS)

```bash
cd vagrant
vagrant up nats orchestrator member1

cd ../ansible-nats
ansible-playbook -i inventory.ini playbook.yml

cd ../ansible-orchestrator-centralized
ansible-playbook -i inventory.ini playbook.yml

cd ../ansible-member
ansible-playbook -i inventory.ini playbook.yml

stn lattice --nats nats://192.168.56.9:4222 agents
```

## Files

```
lattice-lab/
├── vagrant/
│   └── Vagrantfile                    # VM definitions (nats, orchestrator, member1, member2)
├── ansible-nats/                      # Standalone NATS server
│   ├── inventory.ini
│   ├── playbook.yml
│   ├── vars/main.yml
│   ├── vars/main-with-auth.yml        # Auth enabled config
│   └── templates/
│       ├── nats.conf.j2
│       └── docker-compose.yml.j2
├── ansible-orchestrator/              # Embedded NATS mode
│   ├── inventory.ini
│   ├── playbook.yml
│   ├── vars/main.yml
│   └── templates/docker-compose.yml.j2
├── ansible-orchestrator-centralized/  # External NATS mode
│   ├── inventory.ini
│   ├── playbook.yml
│   ├── vars/main.yml
│   ├── files/config.yaml
│   └── templates/docker-compose.yml.j2
└── ansible-member/
    ├── inventory.ini
    ├── playbook.yml
    ├── vars/main.yml
    └── templates/docker-compose.yml.j2
```

## IP Addresses

| VM | IP | Purpose |
|----|-------|---------|
| nats | 192.168.56.9 | Standalone NATS server |
| orchestrator | 192.168.56.10 | Station orchestrator |
| member1 | 192.168.56.11 | Station with agents |
| member2 | 192.168.56.12 | Additional station (optional) |

## Configuration

Update `vars/main.yml` in each ansible directory with:

- `STN_AI_PROVIDER` and `STN_AI_API_KEY` or OAuth tokens
- `STN_CLOUDSHIP_*` if using CloudShip
- Any MCP server API keys (e.g., `POSTHOG_API_KEY`)

For NATS auth, edit `ansible-nats/vars/main.yml`:
```yaml
nats_auth_enabled: true
nats_auth_token: "your-secret-token"
```

## Documentation

See [docs/LATTICE_LAB.md](../../docs/LATTICE_LAB.md) for the full tutorial.
