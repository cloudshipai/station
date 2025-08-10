# Multi-stage Dockerfile for ntfy server with custom configuration
FROM binwiederhier/ntfy:latest

# Create necessary directories
RUN mkdir -p /var/cache/ntfy /etc/ntfy

# Copy custom configuration
COPY server.yml /etc/ntfy/server.yml

# Set proper permissions
RUN chown -R ntfy:ntfy /var/cache/ntfy /etc/ntfy

# Use non-root user
USER ntfy

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/v1/health || exit 1

# Start ntfy server with custom config
CMD ["serve", "--config", "/etc/ntfy/server.yml"]