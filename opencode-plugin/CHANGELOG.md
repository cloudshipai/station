# Changelog

All notable changes to the `@cloudshipai/opencode-plugin` package will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-01-02

### Added
- Initial release of the OpenCode Station plugin
- NATS bridge for receiving coding tasks from Station orchestrator
- Workspace management with git clone/pull support
- Session management with continuation support
- Streaming events back to Station via NATS
- Custom tools: `station_kv_get`, `station_kv_set`, `station_session_info`
- Production Dockerfile with plugin pre-installed
- Comprehensive integration test suite

### Features
- **Git Workspaces**: Clone repos, checkout branches, pull updates
- **Session Continuity**: Resume sessions across tasks
- **Event Streaming**: Real-time progress via NATS
- **Workspace Reuse**: Efficient handling of repeated tasks

[Unreleased]: https://github.com/cloudshipai/station/compare/opencode-plugin-v0.1.0...HEAD
[0.1.0]: https://github.com/cloudshipai/station/releases/tag/opencode-plugin-v0.1.0
