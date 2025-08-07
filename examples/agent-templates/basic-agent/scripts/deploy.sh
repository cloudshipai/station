#!/bin/bash
# Basic Agent Deployment Script
# Usage: ./deploy.sh [environment] [client_name]

set -e

ENVIRONMENT=${1:-"development"}
CLIENT_NAME=${2:-"Default Client"}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUNDLE_DIR="${SCRIPT_DIR}/../bundle"
VARIABLES_DIR="${SCRIPT_DIR}/../variables"

echo "🚀 Deploying Basic Agent Template"
echo "📦 Bundle: ${BUNDLE_DIR}"
echo "🌍 Environment: ${ENVIRONMENT}" 
echo "👤 Client: ${CLIENT_NAME}"
echo

# Check if bundle exists
if [ ! -d "$BUNDLE_DIR" ]; then
  echo "❌ Bundle directory not found: $BUNDLE_DIR"
  exit 1
fi

# Validate bundle
echo "✅ Validating bundle..."
stn agent bundle validate "$BUNDLE_DIR"

# Choose deployment method based on environment
case $ENVIRONMENT in
  "development")
    echo "🔧 Installing in interactive mode for development..."
    stn agent bundle install "$BUNDLE_DIR" --interactive --env "$ENVIRONMENT"
    ;;
  "staging")
    echo "📝 Installing with YAML variables for staging..."
    if [ -f "$VARIABLES_DIR/staging.yml" ]; then
      stn agent bundle install "$BUNDLE_DIR" \
        --vars-file "$VARIABLES_DIR/staging.yml" \
        --env "$ENVIRONMENT"
    else
      echo "⚠️  No staging.yml found, using interactive mode..."
      stn agent bundle install "$BUNDLE_DIR" --interactive --env "$ENVIRONMENT"
    fi
    ;;
  "production")
    echo "🏭 Installing with production variables..."
    if [ -f "$VARIABLES_DIR/production.json" ]; then
      # Override CLIENT_NAME if provided
      if [ "$CLIENT_NAME" != "Default Client" ]; then
        echo "📝 Overriding client name: $CLIENT_NAME"
        stn agent bundle install "$BUNDLE_DIR" \
          --vars-file "$VARIABLES_DIR/production.json" \
          --vars "CLIENT_NAME=$CLIENT_NAME" \
          --env "$ENVIRONMENT"
      else
        stn agent bundle install "$BUNDLE_DIR" \
          --vars-file "$VARIABLES_DIR/production.json" \
          --env "$ENVIRONMENT"
      fi
    else
      echo "❌ No production.json found in $VARIABLES_DIR"
      exit 1
    fi
    ;;
  *)
    echo "⚠️  Unknown environment: $ENVIRONMENT"
    echo "Using development variables with custom environment..."
    stn agent bundle install "$BUNDLE_DIR" \
      --vars-file "$VARIABLES_DIR/development.json" \
      --vars "CLIENT_NAME=$CLIENT_NAME" \
      --env "$ENVIRONMENT"
    ;;
esac

echo "✅ Deployment completed successfully!"
echo "🎯 Next steps:"
echo "   • Check agent status: stn agent list --env $ENVIRONMENT"  
echo "   • Test agent: stn agent run <agent_id> 'list files in current directory'"
echo "   • Monitor logs: stn logs --env $ENVIRONMENT"