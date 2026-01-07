# Lattice Lab Example

Example files for deploying Station Lattice across VMs using Vagrant and Ansible.

## Prerequisites

- Vagrant with libvirt or VirtualBox provider
- Ansible
- Station CLI (`stn`)

## Quick Start

```bash
# 1. Start VMs
cd vagrant
vagrant up

# 2. Deploy orchestrator
cd ../ansible-orchestrator
# Edit vars/main.yml with your credentials
ansible-playbook -i inventory.ini playbook.yml

# 3. Deploy member
cd ../ansible-member
# Edit vars/main.yml with your credentials  
ansible-playbook -i inventory.ini playbook.yml

# 4. Test lattice
stn lattice --nats nats://192.168.56.10:4222 agents
```

## Files

```
lattice-lab/
├── vagrant/
│   └── Vagrantfile           # VM definitions
├── ansible-orchestrator/
│   ├── inventory.ini         # Host inventory
│   ├── playbook.yml          # Deployment playbook
│   ├── vars/main.yml         # Configuration (add your secrets here)
│   └── templates/
│       └── docker-compose.yml.j2
└── ansible-member/
    ├── inventory.ini
    ├── playbook.yml
    ├── vars/main.yml
    └── templates/
        └── docker-compose.yml.j2
```

## Configuration

Before running, update `vars/main.yml` in each ansible directory with:

- `STN_AI_PROVIDER` and `STN_AI_API_KEY` or OAuth tokens
- `STN_CLOUDSHIP_*` if using CloudShip
- Any MCP server API keys (e.g., `POSTHOG_API_KEY`)

## Documentation

See [docs/LATTICE_LAB.md](../../docs/LATTICE_LAB.md) for the full tutorial.
