# VHS Demo: Dual Config Demo
# Shows loading both HTTP and template configs

# Set terminal appearance
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set TypingSpeed 0.025s  # 4x speed

# Output settings
Output assets/dual-config-demo.gif

Type "clear"
Enter

Type "# 🔄 Dual MCP Config Demo"
Enter
Sleep 400ms

Type "# 1. Show both config types"
Enter
Sleep 300ms

Type "# HTTP Config (direct, no variables):"
Enter
Type "cat examples/mcps/aws-knowledge.json"
Enter
Sleep 1.5s

Type "# Template Config (uses variables):"
Enter
Type "cat examples/mcps/filesystem.json"
Enter
Sleep 1.5s

Type "# Variables file:"
Enter
Type "cat ~/.config/station/environments/default/variables.yml"
Enter
Sleep 1s

Type "# 2. Load both configurations"
Enter
Sleep 300ms

Type "stn load examples/mcps/aws-knowledge.json"
Enter
Sleep 2s

Type "stn load examples/mcps/filesystem.json"
Enter
Sleep 2s

Type "# 3. List loaded configurations"
Enter
Type "stn mcp list"
Enter
Sleep 1.5s

Type "# 4. Discover from HTTP server"
Enter
Type "stn mcp discover 1 default"
Enter
Sleep 3s

Type "# 5. Show tools discovered so far"
Enter
Type "stn mcp tools list"
Enter
Sleep 3s

Type "# ✅ HTTP transport working! Template ready for discovery"
Enter
Sleep 1s