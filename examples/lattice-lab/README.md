# Lattice Lab Example

Example files for deploying Station Lattice across multiple environments.

## Quick Start

```bash
# Fastest way - local Docker E2E
cd e2e-local
cp .env.example .env  # Edit with your credentials
docker compose up -d
stn lattice --nats "nats://${NATS_AUTH_TOKEN}@localhost:14222" agents
```

## Full Tutorial

See **[docs/LATTICE_LAB.md](../../docs/LATTICE_LAB.md)** for the complete 9-lab tutorial.

## Directory Structure

```
lattice-lab/
├── README.md                           # This file
├── agents/                             # Lab agent prompts
│   ├── echo-agent.prompt               # Simple echo test
│   ├── math-agent.prompt               # Calculations
│   ├── time-agent.prompt               # Time queries  
│   ├── joke-agent.prompt               # Programming humor
│   └── sysinfo-agent.prompt            # Station info
│
├── e2e-local/                          # Lab 9: Docker E2E (easiest)
│   ├── .env.example                    # Required variables template
│   ├── .gitignore                      # Excludes .env (secrets)
│   ├── docker-compose.yml              # NATS + Station containers
│   └── README.md                       # Detailed instructions
│
├── vagrant/                            # VM definitions
│   └── Vagrantfile                     # nats, orchestrator, member1, member2
│
├── ansible-nats/                       # Lab 8: Standalone NATS server
│   ├── inventory.ini
│   ├── playbook.yml
│   ├── vars/main.yml                   # NATS config
│   ├── vars/main-with-auth.yml         # Auth enabled config
│   └── templates/
│       ├── nats.conf.j2
│       └── docker-compose.yml.j2
│
├── ansible-orchestrator/               # Embedded NATS mode
│   ├── inventory.ini
│   ├── playbook.yml
│   ├── vars/main.yml
│   └── templates/docker-compose.yml.j2
│
├── ansible-orchestrator-centralized/   # External NATS mode
│   ├── inventory.ini
│   ├── playbook.yml
│   ├── vars/main.yml
│   ├── files/config.yaml
│   └── templates/docker-compose.yml.j2
│
└── ansible-member/                     # Member station
    ├── inventory.ini
    ├── playbook.yml
    ├── vars/main.yml
    └── templates/docker-compose.yml.j2
```

## Deployment Modes

### Mode 1: Embedded NATS (Simple)

Orchestrator runs embedded NATS server. Best for small deployments.

```
┌─────────────────────────────────────────────────────────────┐
│                                                              │
│  ┌───────────────────────────────┐     ┌─────────────────┐  │
│  │ Orchestrator                  │     │ Member 1        │  │
│  │ • Embedded NATS :4222         │◄────│ • Your agents   │  │
│  │ • Station agents              │     └─────────────────┘  │
│  └───────────────────────────────┘                          │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Mode 2: Centralized NATS (Production)

Separate NATS server, all stations connect as clients.

```
┌─────────────────────────────────────────────────────────────┐
│                                                              │
│               ┌─────────────────────┐                       │
│               │ NATS Server         │                       │
│               │ :4222 (JetStream)   │                       │
│               └──────────┬──────────┘                       │
│                          │                                   │
│          ┌───────────────┼───────────────┐                  │
│          │               │               │                  │
│  ┌───────▼───────┐ ┌─────▼─────┐ ┌──────▼──────┐           │
│  │ Orchestrator  │ │ Member 1  │ │ Member 2    │           │
│  │ sysinfo, echo │ │ math      │ │ time, joke  │           │
│  └───────────────┘ └───────────┘ └─────────────┘           │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Lab Agents

| Agent | Purpose | Station Assignment |
|-------|---------|-------------------|
| `echo-agent` | Echo test for connectivity | Orchestrator |
| `sysinfo-agent` | Station/lattice status | Orchestrator |
| `math-agent` | Mathematical calculations | Compute |
| `time-agent` | Time queries | Utility |
| `joke-agent` | Programming humor | Utility |

## Configuration

### Required Variables

Run `stn deploy export-vars default --target ansible` to see all needed variables.

Key variables:
- `STN_AI_PROVIDER` / `STN_AI_API_KEY` - AI provider credentials
- `STN_LATTICE_NATS_URL` - NATS connection URL (for members)
- `STN_LATTICE_STATION_NAME` - Unique station name
- `POSTHOG_API_KEY`, etc. - MCP server credentials (if using those agents)

### NATS Authentication

Edit `ansible-nats/vars/main.yml`:

```yaml
nats_auth_token: "your-secret-token"
```

Or for user/password:
```yaml
nats_users:
  - user: admin
    password: admin-secret
  - user: member
    password: member-secret
```

## VM IP Addresses

| VM | IP | Purpose |
|----|-------|---------|
| nats | 192.168.56.9 | Standalone NATS server |
| orchestrator | 192.168.56.10 | Station orchestrator |
| member1 | 192.168.56.11 | Station with agents |
| member2 | 192.168.56.12 | Additional station (optional) |

## Commands

```bash
# Start VMs
cd vagrant && vagrant up orchestrator member1

# Deploy orchestrator
cd ansible-orchestrator && ansible-playbook -i inventory.ini playbook.yml

# Deploy member
cd ansible-member && ansible-playbook -i inventory.ini playbook.yml

# Test lattice
stn lattice --nats nats://192.168.56.10:4222 agents
stn lattice --nats nats://192.168.56.10:4222 agent exec echo-agent "Hello!"
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| NATS connection refused | Check port 4222 is open, NATS is running |
| No agents appearing | Wait 30s for presence heartbeat, check station logs |
| Auth failed | Verify token/credentials match in NATS and station config |
| JetStream error | Ensure NATS started with `-js` flag |
