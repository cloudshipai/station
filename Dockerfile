# Station Runtime Container
# Published to ghcr.io/cloudshipai/station and used by `stn up`
# Multi-platform support: linux/amd64 and linux/arm64

FROM ubuntu:22.04

ARG TARGETARCH
ARG TARGETOS

# Install essential packages
RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    git \
    sqlite3 \
    python3 \
    python3-pip \
    python3-venv \
    build-essential \
    openssh-client \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js 20 and genkit CLI for development mode
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/* \
    && npm install -g genkit-cli

# Install Docker CLI (for Docker-in-Docker via socket mount)
ARG TARGETARCH
RUN ARCH=$([ "$TARGETARCH" = "arm64" ] && echo "aarch64" || echo "x86_64") && \
    curl -fsSL https://download.docker.com/linux/static/stable/${ARCH}/docker-27.1.1.tgz | tar -xz \
    && mv docker/docker /usr/local/bin/docker \
    && rm -rf docker \
    && chmod +x /usr/local/bin/docker

# Create a non-root user 'station' with UID 1000 (common Linux desktop UID)
RUN groupadd -g 1000 station && \
    useradd -m -u 1000 -g station -s /bin/bash station && \
    # Give station user sudo access for flexibility
    apt-get update && apt-get install -y sudo gosu && rm -rf /var/lib/apt/lists/* && \
    usermod -aG sudo station && \
    echo "station ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Set up directories for station user
RUN mkdir -p /home/station/.local/bin /home/station/.cargo/bin \
    /home/station/.config/station /home/station/.cache \
    /home/station/.npm /home/station/.local/share && \
    chown -R station:station /home/station

# Install uv as station user
USER station
WORKDIR /home/station
ENV HOME=/home/station
ENV PATH="/home/station/.local/bin:/home/station/.cargo/bin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

RUN curl -LsSf https://astral.sh/uv/install.sh | sh

# Install Ship CLI as station user
ARG INSTALL_SHIP=true
RUN if [ "$INSTALL_SHIP" = "true" ]; then \
        timeout 300 bash -c 'curl -fsSL --max-time 60 https://raw.githubusercontent.com/cloudshipai/ship/main/install.sh | bash' || \
        echo 'Ship CLI installation failed or timed out'; \
    fi

# Switch back to root for system-level setup
USER root

# Copy Station binary for the target architecture
# GoReleaser builds binaries in dist/station_${GOOS}_${GOARCH}/stn
COPY dist/station_${TARGETOS}_${TARGETARCH}*/stn /usr/local/bin/stn
RUN chmod +x /usr/local/bin/stn

# Copy MCP server templates directory
COPY mcp-servers/ /usr/share/station/mcp-servers/
RUN chown -R station:station /usr/share/station

# Create necessary directories with proper ownership
# Also create symlink from /root/.config to /home/station/.config for compatibility
RUN mkdir -p /workspace /var/log/station /root && \
    ln -s /home/station/.config /root/.config && \
    chown -R station:station /workspace /var/log/station /home/station && \
    chmod 755 /root

# Health check script
RUN echo '#!/bin/bash\ncurl -f http://localhost:3000/health || exit 1' > /usr/local/bin/health-check && \
    chmod +x /usr/local/bin/health-check

# Flexible entrypoint that handles different UIDs
RUN echo '#!/bin/bash\n\
set -e\n\
\n\
# If running with --user flag (Linux host), just exec\n\
if [ "$(id -u)" != "0" ] && [ "$(id -u)" != "1000" ]; then\n\
    exec "$@"\n\
fi\n\
\n\
# If running as root, drop to station user using gosu\n\
if [ "$(id -u)" = "0" ]; then\n\
    # Fix workspace ownership if needed\n\
    if [ -d /workspace ]; then\n\
        chown station:station /workspace 2>/dev/null || true\n\
    fi\n\
    # Fix data directory ownership for SQLite (prevents "out of memory (14)" errors)\n\
    if [ -d /data ]; then\n\
        chown -R station:station /data 2>/dev/null || true\n\
    fi\n\
    # Set proper environment for station user\n\
    export HOME=/home/station\n\
    export USER=station\n\
    # Drop privileges to station user\n\
    exec gosu station "$@"\n\
fi\n\
\n\
# Running as station user (UID 1000)\n\
exec "$@"' > /usr/local/bin/docker-entrypoint.sh && \
    chmod +x /usr/local/bin/docker-entrypoint.sh

# Switch to station user by default
USER station

# Environment variables for station user
ENV PATH="/home/station/.local/bin:/home/station/.cargo/bin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
ENV HOME="/home/station"
ENV STATION_RUNTIME="docker"
ENV STATION_CONFIG_DIR="/home/station/.config/station"
# Disable MCP connection pooling in container to prevent hanging
ENV STATION_MCP_POOLING="false"

# Set working directory
WORKDIR /workspace

# Expose ports for MCP, Dynamic Agent MCP, UI/API, and Genkit Developer UI
EXPOSE 3000 3030 3002 4000 8585

# Use entrypoint to handle permission fixes
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

# Default command runs Station server
CMD ["stn", "serve"]

# OCI labels
LABEL org.opencontainers.image.source="https://github.com/cloudshipai/station"
LABEL org.opencontainers.image.description="Station - AI Infrastructure Platform for MCP agents"
LABEL org.opencontainers.image.licenses="Apache-2.0"