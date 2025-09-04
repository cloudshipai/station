# Station Agent Creation Guide

## Overview

Station automatically creates agents with a proper dotprompt format that includes:
- YAML metadata with tools and configuration
- Multi-role structure with system and user roles
- Handlebars template variables for dynamic input
- Support for custom input schemas

## Agent Creation Format

When you create an agent in Station, it automatically generates a `.prompt` file with this structure:

### Basic Agent (no custom inputs)

```yaml
---
metadata:
  name: "Agent Name"
  description: "Agent description"
  tags: ["station", "agent"]
model: gpt-4o-mini
max_steps: 8
tools:
  - "__tool_name_1"
  - "__tool_name_2"
input:
  schema:
    userInput: string
---

{{role "system"}}
Your agent's system prompt goes here...

{{role "user"}}
{{userInput}}
```

### Agent with Custom Input Schema

```yaml
---
metadata:
  name: "Web Automation Agent"
  description: "Playwright automation agent"
  tags: ["web", "automation", "testing"]
model: gpt-4o-mini
max_steps: 8
tools:
  - "__browser_navigate"
  - "__browser_click"
  - "__browser_screenshot"
input:
  schema:
    userInput: string
    website_url: string, URL of the website to interact with
    action_type: ["navigate", "screenshot", "extract", "test"], Type of action to perform
---

{{role "system"}}
You are an expert web automation agent...

{{role "user"}}
{{userInput}}

**website_url:** {{website_url}}

**action_type:** {{action_type}}
```

## Creating Agents via MCP

### Basic Agent Creation

```javascript
// Via MCP create_agent tool
{
  "name": "My Agent",
  "description": "Agent description",
  "prompt": "System prompt for the agent",
  "environment_id": "1",
  "max_steps": 8,
  "tool_names": ["__tool1", "__tool2"]
}
```

### Agent with Custom Input Schema

```javascript
// Via MCP create_agent tool with input_schema
{
  "name": "Web Automation Agent", 
  "description": "Playwright automation agent",
  "prompt": "You are an expert web automation agent...",
  "environment_id": "1",
  "max_steps": 8,
  "tool_names": ["__browser_navigate", "__browser_click"],
  "input_schema": {
    "website_url": {
      "type": "string",
      "description": "URL of the website to interact with",
      "required": true
    },
    "action_type": {
      "type": "string", 
      "description": "Type of action to perform",
      "enum": ["navigate", "screenshot", "extract", "test"],
      "default": "navigate"
    }
  }
}
```

## Input Schema Format

Station supports JSON Schema format for custom input variables:

```json
{
  "variable_name": {
    "type": "string|number|boolean|array|object",
    "description": "Description of the variable",
    "required": true|false,
    "default": "default_value",
    "enum": ["option1", "option2"]
  }
}
```

### Supported Field Types

- **string**: Text input
- **number**: Numeric input
- **boolean**: True/false input
- **array**: List of values
- **object**: Complex object structure

### Field Properties

- **type**: Required. Data type of the field
- **description**: Optional. Human-readable description
- **required**: Optional. Whether the field is mandatory (default: false)
- **default**: Optional. Default value if not provided
- **enum**: Optional. List of allowed values

## Auto-Generated Files

When you create an agent, Station automatically:

1. **Creates Database Entry**: Stores agent metadata in SQLite
2. **Assigns Tools**: Links MCP tools to the agent
3. **Exports Dotprompt File**: Creates `.prompt` file in:
   ```
   ~/.config/station/environments/{environment}/agents/{agent_name}.prompt
   ```

## Best Practices

### System Prompts
- Be specific about the agent's capabilities
- Include examples of expected behavior
- Explain how to use available tools
- Set clear boundaries and limitations

### Input Schemas
- Use descriptive variable names
- Provide clear descriptions for each field
- Set reasonable defaults where appropriate
- Use enums for constrained options

### Tool Assignment
- Only assign tools the agent actually needs
- Group related tools together
- Ensure tool names match exactly with MCP server exports

## Example: Complete Web Automation Agent

```javascript
{
  "name": "Playwright Agent",
  "description": "Web automation agent using Playwright for browser interactions, testing, and scraping",
  "prompt": "You are an expert web automation agent that uses Playwright to interact with web pages. You excel at:\n\n• **Web Navigation**: Opening pages, following links, handling navigation\n• **Element Interaction**: Clicking buttons, filling forms, typing text\n• **Data Extraction**: Scraping content, taking screenshots, getting page text\n• **Web Testing**: Verifying page elements, testing user workflows\n• **Browser Automation**: Handling dynamic content, waiting for elements\n\n**Key Capabilities:**\n- Take screenshots of web pages for visual verification\n- Navigate to any URL and interact with page elements\n- Fill out forms and submit data\n- Extract text content and data from pages\n- Wait for elements to load before interacting\n- Execute JavaScript in the browser context\n\nAlways explain what you're doing with the web page and provide clear feedback about the results of your actions.",
  "environment_id": "1",
  "max_steps": 8,
  "tool_names": [
    "__browser_take_screenshot",
    "__browser_navigate", 
    "__browser_click",
    "__browser_fill_form",
    "__browser_type",
    "__browser_wait_for",
    "__browser_evaluate",
    "__browser_snapshot",
    "__browser_hover",
    "__browser_drag",
    "__browser_tabs",
    "__browser_console_messages"
  ],
  "input_schema": {
    "website_url": {
      "type": "string",
      "description": "URL of the website to interact with",
      "required": true
    },
    "task_type": {
      "type": "string",
      "description": "Type of web automation task",
      "enum": ["navigate", "screenshot", "form_fill", "data_extract", "test"],
      "default": "navigate"
    },
    "wait_timeout": {
      "type": "number",
      "description": "Timeout in seconds for waiting operations",
      "default": 30
    },
    "headless": {
      "type": "boolean", 
      "description": "Run browser in headless mode",
      "default": true
    }
  }
}
```

This will generate a complete dotprompt file with proper role structure, handlebars templates, and input schema support.