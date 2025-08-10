#!/bin/bash
set -e

# ntfy Server Deployment Script for Fly.io
# Usage: ./deploy.sh <app-name> [region]

if [ $# -eq 0 ]; then
    echo "❌ Error: App name is required"
    echo "Usage: ./deploy.sh <app-name> [region]"
    echo "Example: ./deploy.sh my-ntfy-server ord"
    exit 1
fi

APP_NAME=$1
REGION=${2:-"ord"}

echo "🚀 Deploying ntfy server to Fly.io"
echo "App Name: $APP_NAME"
echo "Region: $REGION"

# Check if flyctl is installed
if ! command -v flyctl &> /dev/null; then
    echo "❌ flyctl is not installed. Please install it first:"
    echo "   https://fly.io/docs/hands-on/install-flyctl/"
    exit 1
fi

# Check if user is logged in
if ! flyctl auth whoami &> /dev/null; then
    echo "❌ Not logged in to Fly.io. Please run: flyctl auth login"
    exit 1
fi

# Generate fly.toml from template
echo "📝 Generating fly.toml from template..."
if [ ! -f "fly.toml.template" ]; then
    echo "❌ Error: fly.toml.template not found"
    exit 1
fi
sed "s/{{APP_NAME}}/$APP_NAME/g" fly.toml.template > fly.toml

# Generate server.yml from template
echo "📝 Generating server.yml from template..."
if [ ! -f "server.yml.template" ]; then
    echo "❌ Error: server.yml.template not found"
    exit 1
fi
sed "s/{{APP_NAME}}/$APP_NAME/g" server.yml.template > server.yml

# Create app if it doesn't exist
echo "🏗️  Creating Fly.io app (if needed)..."
if ! flyctl apps list | grep -q "^$APP_NAME"; then
    flyctl apps create "$APP_NAME" --org personal
    echo "✅ App '$APP_NAME' created"
else
    echo "ℹ️  App '$APP_NAME' already exists"
fi

# Create volume if it doesn't exist
echo "💾 Creating persistent volume (if needed)..."
if ! flyctl volumes list -a "$APP_NAME" | grep -q "ntfy_data"; then
    flyctl volumes create ntfy_data --region "$REGION" --size 1 -a "$APP_NAME"
    echo "✅ Volume 'ntfy_data' created"
else
    echo "ℹ️  Volume 'ntfy_data' already exists"
fi

# Deploy the application
echo "🚀 Deploying application..."
flyctl deploy -a "$APP_NAME"

# Show status
echo "📊 Application status:"
flyctl status -a "$APP_NAME"

# Show URL
echo ""
echo "🎉 Deployment complete!"
echo "📱 ntfy server URL: https://$APP_NAME.fly.dev"
echo "🔍 Health check: https://$APP_NAME.fly.dev/v1/health"
echo ""
echo "📋 Next steps:"
echo "   1. Test: curl -d 'Hello World' https://$APP_NAME.fly.dev/test"
echo "   2. View logs: flyctl logs -a $APP_NAME"
echo "   3. Monitor: flyctl status -a $APP_NAME"
echo ""
echo "📚 Documentation: https://docs.ntfy.sh/"