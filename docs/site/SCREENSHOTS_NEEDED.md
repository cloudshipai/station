# Screenshots Needed for Documentation

Based on the user's UI screenshots, we need these images added to `/docs/site/public/`:

## Required Screenshots:

1. **bundles-ui.png** - Bundles tab showing the devops-security-bundle
   - Shows the bundle card with "1.3 KB" size and modification date
   - "Install Bundle" button in top right
   - Clean dark UI theme

2. **environment-view.png** - Environment tab showing default environment 
   - Shows "default" environment with "2 agents, 2 servers"
   - Security Scanner and Terraform Auditor agent cards
   - ship-checkov and ship-tflint MCP server connections
   - Visual connection lines between components

3. **agent-interface.png** - Agent execution interface
   - Shows agent selection dropdown with "Security Scanner"
   - Agent description and capabilities
   - Connection to ship-checkov MCP server with "4 tools available"

4. **runs-view.png** - Runs tab showing completed executions
   - Shows "Terraform Auditor" and "Security Scanner" completed runs
   - Duration and completion timestamps
   - "Completed" status indicators

5. **sync-modal.png** - Sync Environment modal
   - Shows "Ready to Sync" database icon
   - "This will sync all MCP server configurations for the default environment"
   - "Start Sync" button

## Current Status:
- ✅ Copied existing assets (default-environment.png, terraform-quality-agent.png)  
- ❌ Missing actual UI screenshots from user's images
- 📝 Documentation references these images but they need to be captured from live UI

## Next Steps:
1. Capture screenshots from running Station UI at localhost:8585
2. Save as PNG files in /docs/site/public/ 
3. Update documentation references to match actual image names