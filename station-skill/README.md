# Station Skill for OpenCode

OpenCode plugin that provides skills for [Station](https://github.com/cloudshipai/station) - self-hosted AI agent orchestration platform.

## Installation

```bash
bunx @cloudshipai/station-skill install
```

This adds the plugin to your `~/.config/opencode/opencode.json`. Restart OpenCode to load the skills.

## Uninstall

```bash
bunx @cloudshipai/station-skill uninstall
```

## Skills Included

| Skill | Description |
|-------|-------------|
| `station` | Core CLI commands - agents, workflows, MCP tools, deployment, benchmarking |
| `station-config` | Configuration management via CLI and browser UI |

## Usage

After installation, ask OpenCode to use the skills:

```
Load the station skill and create an agent
```

```
/skill station
How do I deploy my agents to Fly.io?
```

## Prerequisites

- [Station CLI](https://github.com/cloudshipai/station) installed (`stn --version`)
- Station initialized (`stn init`)

## Related

- [Station CLI Documentation](https://docs.cloudshipai.com/station/overview)
- [Claude Code Plugin](https://github.com/cloudshipai/station/tree/main/claude-code-plugin) - For Claude Code users

## License

MIT
