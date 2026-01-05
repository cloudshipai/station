# OpenCode with Station Plugin - Production Image
# This image includes the Station NATS bridge plugin pre-installed
#
# Build: docker build -t ghcr.io/cloudshipai/opencode-station:latest .
# Run:   docker run -e NATS_URL=nats://host:4222 -e OPENAI_API_KEY=... ghcr.io/cloudshipai/opencode-station:latest

ARG OPENCODE_VERSION=latest
FROM ghcr.io/sst/opencode:${OPENCODE_VERSION}

# Plugin version (injected at build time)
ARG PLUGIN_VERSION=0.1.0
LABEL org.opencontainers.image.title="OpenCode with Station Plugin"
LABEL org.opencontainers.image.description="OpenCode AI coding agent with CloudShip Station NATS bridge plugin"
LABEL org.opencontainers.image.vendor="CloudShip AI"
LABEL org.opencontainers.image.source="https://github.com/cloudshipai/station"
LABEL org.opencontainers.image.version="${PLUGIN_VERSION}"
LABEL ai.cloudship.plugin.version="${PLUGIN_VERSION}"

# Install runtime dependencies
# - git: for workspace clone/pull operations
# - curl, unzip, bash: for Bun installation and general utilities
RUN apk add --no-cache \
    curl \
    unzip \
    bash \
    git \
    openssh-client

# Install Bun runtime (required for plugin execution)
RUN curl -fsSL https://bun.sh/install | bash \
    && ln -s /root/.bun/bin/bun /usr/local/bin/bun \
    && ln -s /root/.bun/bin/bunx /usr/local/bin/bunx

# Create directories
# - OpenCode scans ~/.config/opencode/plugin/ for local plugins at startup
# - /workspaces is the default workspace root for coding tasks
RUN mkdir -p /root/.config/opencode/plugin \
    && mkdir -p /root/.opencode \
    && mkdir -p /workspaces

# Copy the built plugin
# The plugin is built during CI and copied here
COPY dist/index.js /root/.config/opencode/plugin/station-plugin.js

# Environment variables with sensible defaults
# Override these at runtime as needed
ENV NATS_URL=""
ENV NATS_SUBJECT_PREFIX="station.coding"
ENV WORKSPACE_ROOT="/workspaces"
ENV OPENCODE_AUTO_APPROVE="true"

# Expose OpenCode API port
EXPOSE 4096

# Default working directory
WORKDIR /workspaces

# Healthcheck to verify OpenCode is running
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -sf http://localhost:4096/health || exit 1

# Run OpenCode in serve mode
# The plugin auto-loads from ~/.config/opencode/plugin/
ENTRYPOINT ["opencode", "serve", "--hostname", "0.0.0.0"]
