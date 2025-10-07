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

# Install Node.js 20
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install uv (Python package manager)
RUN curl -LsSf https://astral.sh/uv/install.sh | sh \
    && ln -sf /root/.cargo/bin/uv /usr/local/bin/uv \
    && ln -sf /root/.cargo/bin/uvx /usr/local/bin/uvx

# Install Docker CLI (for Docker-in-Docker via socket mount)
ARG TARGETARCH
RUN ARCH=$([ "$TARGETARCH" = "arm64" ] && echo "aarch64" || echo "x86_64") && \
    curl -fsSL https://download.docker.com/linux/static/stable/${ARCH}/docker-27.1.1.tgz | tar -xz \
    && mv docker/docker /usr/local/bin/docker \
    && rm -rf docker \
    && chmod +x /usr/local/bin/docker

# Install Ship CLI for security tools
ARG INSTALL_SHIP=true
RUN if [ "$INSTALL_SHIP" = "true" ]; then \
        timeout 300 bash -c 'curl -fsSL --max-time 60 https://raw.githubusercontent.com/cloudshipai/ship/main/install.sh | bash' || \
        echo 'Ship CLI installation failed or timed out'; \
        if [ -f /root/.local/bin/ship ]; then \
            cp /root/.local/bin/ship /usr/local/bin/ship && \
            chmod +x /usr/local/bin/ship; \
        fi \
    fi

# Copy Station binary for the target architecture
# GoReleaser builds binaries in dist/station_${GOOS}_${GOARCH}/stn
COPY dist/station_${TARGETOS}_${TARGETARCH}*/stn /usr/local/bin/stn
RUN chmod +x /usr/local/bin/stn

# Create necessary directories
RUN mkdir -p /workspace /root/.config/station /root/.local/bin /root/.cache /var/log/station

# Make /root writable for Dagger and other tools when running as mapped user
RUN chmod 777 /root /root/.cache

# Health check script
RUN echo '#!/bin/bash\ncurl -f http://localhost:3000/health || exit 1' > /usr/local/bin/health-check && \
    chmod +x /usr/local/bin/health-check

# Entrypoint script to fix permissions on mounted volumes
RUN echo '#!/bin/bash\n\
# Fix permissions for Dagger cache if it exists and is not writable\n\
if [ -d /root/.cache ] && [ ! -w /root/.cache ]; then\n\
    chmod 777 /root/.cache 2>/dev/null || true\n\
fi\n\
# Execute the actual command\n\
exec "$@"' > /usr/local/bin/docker-entrypoint.sh && \
    chmod +x /usr/local/bin/docker-entrypoint.sh

# Environment variables
ENV PATH="/root/.local/bin:/root/.cargo/bin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
ENV HOME="/root"
ENV STATION_RUNTIME="docker"
# Disable MCP connection pooling in container to prevent hanging
ENV STATION_MCP_POOLING="false"

# Set working directory
WORKDIR /workspace

# Expose ports for MCP, Dynamic Agent MCP, and UI/API
EXPOSE 3000 3001 3002 8585

# Use entrypoint to handle permission fixes
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

# Default command runs Station server
CMD ["stn", "serve"]

# OCI labels
LABEL org.opencontainers.image.source="https://github.com/cloudshipai/station"
LABEL org.opencontainers.image.description="Station - AI Infrastructure Platform for MCP agents"
LABEL org.opencontainers.image.licenses="AGPL-3.0"