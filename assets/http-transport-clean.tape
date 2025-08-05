# VHS Demo: HTTP Transport Clean Demo
# Shows HTTP MCP server with clean workflow

# Set terminal appearance
Set FontSize 14
Set Width 1000
Set Height 600
Set Theme "Dracula"
Set TypingSpeed 0.025s  # 4x speed

# Output settings
Output assets/http-transport-clean.gif

Type "clear"
Enter

Type "# 🌐 HTTP Transport MCP Demo"
Enter
Sleep 400ms

Type "# Show HTTP config (no templates needed):"
Enter
Type "cat examples/mcps/aws-knowledge.json"
Enter
Sleep 1.5s

Type "# Load the configuration"
Enter
Type "stn load examples/mcps/aws-knowledge.json"
Enter
Sleep 2s

Type "# List loaded configurations"
Enter
Type "stn mcp list"
Enter
Sleep 1s

Type "# Discover tools via HTTP transport"
Enter
Type "stn mcp discover 1 default"
Enter
Sleep 3s

Type "# Show discovered tools"
Enter
Type "stn mcp tools list"
Enter
Sleep 2.5s

Type "# ✅ 3 AWS tools discovered via HTTP!"
Enter
Sleep 1s