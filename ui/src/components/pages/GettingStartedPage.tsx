import React, { useState } from 'react';
import { BookOpen, Code, Server, Package, GitBranch, Play, Settings, Database, Zap, X } from 'lucide-react';

type Section =
  | 'overview'
  | 'mcp-setup'
  | 'providers'
  | 'agents'
  | 'mcp-servers'
  | 'templates'
  | 'sync'
  | 'environments'
  | 'bundles'
  | 'runs'
  | 'cloudship';

interface TableOfContentsItem {
  id: Section;
  title: string;
  subsections?: string[];
}

const TABLE_OF_CONTENTS: TableOfContentsItem[] = [
  { id: 'overview', title: 'Overview' },
  { id: 'mcp-setup', title: 'MCP Integration', subsections: ['Config File', 'Available Tools'] },
  { id: 'providers', title: 'AI Providers & Models', subsections: ['OpenAI', 'Gemini', 'Custom Endpoints'] },
  { id: 'agents', title: 'Understanding Agents', subsections: ['What are Agents?', 'Agent Prompts', 'Creating Agents'] },
  { id: 'mcp-servers', title: 'MCP Servers & Tools', subsections: ['What are MCP Servers?', 'Adding Servers', 'Tool Discovery'] },
  { id: 'templates', title: 'Templates & Variables', subsections: ['Template System', 'Variables File', 'Go Templates'] },
  { id: 'sync', title: 'Sync Process', subsections: ['What is Sync?', 'When to Sync', 'Interactive Prompts'] },
  { id: 'environments', title: 'Environments', subsections: ['Organization', 'Multi-Environment Setup', 'Copying Environments'] },
  { id: 'bundles', title: 'Bundles & Deployment', subsections: ['What are Bundles?', 'Docker Deployment'] },
  { id: 'runs', title: 'Agent Runs', subsections: ['Executing Agents', 'Run Details', 'Structured Output'] },
  { id: 'cloudship', title: 'CloudShip Integration', subsections: ['Data Ingestion', 'Output Schemas'] },
];

export const GettingStartedPage: React.FC = () => {
  const [activeSection, setActiveSection] = useState<Section>('overview');
  const [lightboxImage, setLightboxImage] = useState<string | null>(null);

  const scrollToSection = (sectionId: Section) => {
    setActiveSection(sectionId);
    const element = document.getElementById(sectionId);
    if (element) {
      element.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  };

  const scrollToSubsection = (subsectionTitle: string) => {
    // Convert subsection title to ID format: "What are Agents?" -> "what-are-agents"
    const subsectionId = subsectionTitle.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
    const element = document.getElementById(subsectionId);
    if (element) {
      element.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  };

  return (
    <div className="flex h-screen bg-tokyo-bg overflow-hidden">
      {/* Main Content */}
      <div className="flex-1 overflow-y-auto p-8">
        <div className="max-w-4xl mx-auto">
          {/* Header */}
          <div className="mb-8">
            <div className="flex items-center gap-3 mb-4">
              <BookOpen className="h-8 w-8 text-tokyo-purple" />
              <h1 className="text-3xl font-bold text-tokyo-fg">Getting Started with Station</h1>
            </div>
            <p className="text-tokyo-comment text-lg">
              Learn how to build powerful AI agents with the most flexible MCP agent platform
            </p>
          </div>

          {/* Overview Section */}
          <section id="overview" className="mb-12 scroll-mt-8">
            <h2 className="text-2xl font-bold text-tokyo-fg mb-4 flex items-center gap-2">
              <Zap className="h-6 w-6 text-tokyo-green" />
              Welcome to Station
            </h2>
            <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 mb-4">
              <p className="text-tokyo-fg mb-4">
                Great! You have Station running, the most powerful MCP agent builder out there. Here's what you can do next:
              </p>
              <ul className="space-y-2 text-tokyo-fg">
                <li className="flex items-start gap-2">
                  <span className="text-tokyo-green mt-1">•</span>
                  <span>Build intelligent agents that interact with your LLM through MCP</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-tokyo-green mt-1">•</span>
                  <span>Organize agents and tools across multiple environments</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-tokyo-green mt-1">•</span>
                  <span>Create reusable bundles and deploy with Docker</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-tokyo-green mt-1">•</span>
                  <span>Integrate with CloudShip for structured data ingestion</span>
                </li>
              </ul>
            </div>
          </section>

          {/* MCP Setup Section */}
          <section id="mcp-setup" className="mb-12 scroll-mt-8">
            <h2 className="text-2xl font-bold text-tokyo-fg mb-4 flex items-center gap-2">
              <Code className="h-6 w-6 text-tokyo-purple" />
              MCP Integration
            </h2>

            <div className="space-y-6">
              <div>
                <h3 id="config-file" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Connecting to Station via MCP</h3>
                <p className="text-tokyo-comment mb-4">
                  Station runs as an HTTP MCP server. After running <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">stn up</code>,
                  a <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">.mcp.json</code> file is automatically created in your current directory.
                </p>

                <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-tokyo-comment text-sm font-mono">.mcp.json</span>
                    <span className="text-tokyo-comment text-xs">Auto-generated</span>
                  </div>
                  <pre className="text-tokyo-fg font-mono text-sm overflow-x-auto">
{`{
  "mcpServers": {
    "station": {
      "type": "http",
      "url": "http://localhost:8586/mcp"
    }
  }
}`}
                  </pre>
                </div>

                <div className="mt-4 bg-transparent border border-tokyo-blue rounded-lg p-4">
                  <p className="text-sm text-tokyo-fg font-mono mb-2">
                    💡 <strong>Point your MCP host to this config:</strong>
                  </p>
                  <p className="text-sm text-tokyo-fg">
                    Claude Code, Cursor, or any MCP-compatible client can use this config to interact with Station.
                  </p>
                </div>
              </div>

              <div>
                <h3 id="available-tools" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Available MCP Tools</h3>
                <p className="text-tokyo-comment mb-4">
                  Station provides 28 MCP tools for managing agents, environments, and execution:
                </p>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-purple font-semibold mb-2">Agent Management</h4>
                    <ul className="text-sm text-tokyo-comment space-y-1 font-mono">
                      <li>• create_agent</li>
                      <li>• list_agents</li>
                      <li>• get_agent_details</li>
                      <li>• update_agent</li>
                      <li>• delete_agent</li>
                    </ul>
                  </div>

                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-green font-semibold mb-2">Execution & Runs</h4>
                    <ul className="text-sm text-tokyo-comment space-y-1 font-mono">
                      <li>• call_agent</li>
                      <li>• list_runs</li>
                      <li>• inspect_run</li>
                    </ul>
                  </div>

                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-cyan font-semibold mb-2">Environment Management</h4>
                    <ul className="text-sm text-tokyo-comment space-y-1 font-mono">
                      <li>• list_environments</li>
                      <li>• create_environment</li>
                      <li>• delete_environment</li>
                    </ul>
                  </div>

                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-orange font-semibold mb-2">MCP Server Management</h4>
                    <ul className="text-sm text-tokyo-comment space-y-1 font-mono">
                      <li>• list_mcp_servers_for_environment</li>
                      <li>• add_mcp_server_to_environment</li>
                      <li>• update_mcp_server_in_environment</li>
                    </ul>
                  </div>

                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-red font-semibold mb-2">Tool Management</h4>
                    <ul className="text-sm text-tokyo-comment space-y-1 font-mono">
                      <li>• discover_tools</li>
                      <li>• list_tools</li>
                      <li>• add_tool</li>
                      <li>• remove_tool</li>
                    </ul>
                  </div>

                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-blue font-semibold mb-2">Export & Bundles</h4>
                    <ul className="text-sm text-tokyo-comment space-y-1 font-mono">
                      <li>• export_agent</li>
                      <li>• export_agents</li>
                      <li>• list_demo_bundles</li>
                      <li>• install_demo_bundle</li>
                    </ul>
                  </div>
                </div>
              </div>
            </div>
          </section>

          {/* AI Providers Section */}
          <section id="providers" className="mb-12 scroll-mt-8">
            <h2 className="text-2xl font-bold text-tokyo-fg mb-4 flex items-center gap-2">
              <Settings className="h-6 w-6 text-tokyo-cyan" />
              AI Providers & Models
            </h2>

            <p className="text-tokyo-comment mb-6">
              Station supports multiple AI providers. Configure your provider during <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">stn up --provider [openai|gemini|custom]</code> or set environment variables.
            </p>

            <div className="space-y-6">
              {/* OpenAI */}
              <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6">
                <h3 id="openai" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">OpenAI Provider</h3>
                <div className="mb-4">
                  <p className="text-sm text-tokyo-fg mb-2">Environment Variable:</p>
                  <code className="bg-tokyo-bg px-2 py-1 rounded text-tokyo-cyan text-sm">OPENAI_API_KEY=sk-...</code>
                </div>

                <div>
                  <p className="text-sm text-tokyo-fg mb-2">Supported Models (15 total):</p>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
                    <div>
                      <p className="text-xs text-tokyo-purple font-semibold mb-1">Latest Models</p>
                      <ul className="text-xs text-tokyo-fg font-mono space-y-1">
                        <li>• gpt-4.1</li>
                        <li>• gpt-4.1-mini</li>
                        <li>• gpt-4.1-nano</li>
                        <li>• gpt-4.5-preview</li>
                      </ul>
                    </div>
                    <div>
                      <p className="text-xs text-tokyo-green font-semibold mb-1">Production Models</p>
                      <ul className="text-xs text-tokyo-fg font-mono space-y-1">
                        <li>• gpt-4o</li>
                        <li>• gpt-4o-mini</li>
                        <li>• gpt-4-turbo</li>
                        <li>• gpt-4</li>
                        <li>• gpt-3.5-turbo</li>
                      </ul>
                    </div>
                    <div>
                      <p className="text-xs text-tokyo-cyan font-semibold mb-1">Reasoning Models</p>
                      <ul className="text-xs text-tokyo-fg font-mono space-y-1">
                        <li>• o3-mini</li>
                        <li>• o1</li>
                        <li>• o1-preview</li>
                        <li>• o1-mini</li>
                      </ul>
                    </div>
                  </div>
                </div>

                <div className="mt-4 p-3 bg-tokyo-blue7 bg-opacity-20 rounded">
                  <p className="text-sm text-tokyo-fg">
                    <strong>Recommended:</strong> <code className="bg-tokyo-bg px-1 rounded text-tokyo-cyan">gpt-4o-mini</code> (cost-effective)
                    or <code className="bg-tokyo-bg px-1 rounded text-tokyo-cyan">gpt-4o</code> (balanced)
                  </p>
                </div>
              </div>

              {/* Gemini */}
              <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6">
                <h3 id="gemini" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Google Gemini Provider</h3>
                <div className="mb-4">
                  <p className="text-sm text-tokyo-fg mb-2">Environment Variables:</p>
                  <div className="space-y-1">
                    <code className="block bg-tokyo-bg px-2 py-1 rounded text-tokyo-cyan text-sm">GEMINI_API_KEY=...</code>
                    <span className="text-xs text-tokyo-comment">or</span>
                    <code className="block bg-tokyo-bg px-2 py-1 rounded text-tokyo-cyan text-sm">GOOGLE_API_KEY=...</code>
                  </div>
                </div>

                <div>
                  <p className="text-sm text-tokyo-fg mb-2">Supported Models:</p>
                  <ul className="text-sm text-tokyo-fg font-mono space-y-1">
                    <li>• gemini-2.5-flash</li>
                    <li>• gemini-2.5-pro</li>
                  </ul>
                </div>
              </div>

              {/* Custom OpenAI-Compatible */}
              <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6">
                <h3 id="custom-endpoints" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Custom OpenAI-Compatible Endpoints</h3>
                <p className="text-sm text-tokyo-fg mb-3">
                  Station supports any OpenAI-compatible API endpoint (Ollama, LM Studio, vLLM, etc.)
                </p>
                <div>
                  <p className="text-sm text-tokyo-fg mb-2">Configuration via <code className="bg-tokyo-bg px-1 rounded">stn up --provider custom</code></p>
                </div>
              </div>
            </div>
          </section>

          {/* Agents Section */}
          <section id="agents" className="mb-12 scroll-mt-8">
            <h2 className="text-2xl font-bold text-tokyo-fg mb-4 flex items-center gap-2">
              <Play className="h-6 w-6 text-tokyo-green" />
              Understanding Agents
            </h2>

            <div className="space-y-6">
              <div>
                <h3 id="what-are-agents" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">What are Agents?</h3>
                <p className="text-tokyo-comment mb-4">
                  Agents are autonomous AI assistants that can use tools to accomplish tasks. Each agent has:
                </p>
                <ul className="space-y-2 text-tokyo-fg">
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-purple mt-1">•</span>
                    <span><strong>System Prompt:</strong> Instructions that define the agent's behavior and expertise</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-purple mt-1">•</span>
                    <span><strong>Tools:</strong> MCP tools the agent can call to interact with external systems</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-purple mt-1">•</span>
                    <span><strong>Model:</strong> The LLM model used for reasoning (gpt-4o, gemini-2.5-flash, etc.)</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-purple mt-1">•</span>
                    <span><strong>Max Steps:</strong> Maximum number of tool calls allowed per execution</span>
                  </li>
                </ul>
              </div>

              <div>
                <h3 id="agent-prompts" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">GenKit DotPrompt Format</h3>
                <p className="text-tokyo-comment mb-4">
                  Agents are stored as <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">.prompt</code> files
                  using GenKit's DotPrompt format with YAML frontmatter:
                </p>

                <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-tokyo-comment text-sm font-mono">~/.config/station/environments/default/agents/example-agent.prompt</span>
                  </div>
                  <pre className="text-tokyo-fg font-mono text-xs overflow-x-auto">
{`---
metadata:
  name: "AWS Cost Analyzer"
  description: "Analyzes AWS cost spikes and anomalies"
  tags: ["finops", "aws", "cost-analysis"]
model: gpt-4o-mini
max_steps: 10
tools:
  - "__get_cost_and_usage"
  - "__get_cost_forecast"
  - "__get_cost_anomalies"
---

{{role "system"}}
You are an AWS Cost Analyzer expert. You investigate cost spikes,
identify root causes, and provide actionable recommendations.

{{role "user"}}
{{userInput}}`}
                  </pre>
                </div>
              </div>

              <div>
                <h3 id="creating-agents" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Creating Agents via MCP</h3>
                <p className="text-tokyo-comment mb-4">
                  Agents are created through MCP using the <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">create_agent</code> tool.
                  There is no UI form for agent creation - you interact with Station through your LLM.
                </p>

                <div className="bg-transparent border border-tokyo-blue rounded-lg p-4">
                  <p className="text-sm text-tokyo-fg mb-2">
                    <strong>Example:</strong> Ask your LLM to create an agent
                  </p>
                  <p className="text-sm text-tokyo-fg italic">
                    "Create a new agent called 'Security Scanner' in the 'production' environment
                    that scans code for vulnerabilities using gitleaks and semgrep tools"
                  </p>
                </div>
              </div>

              <div>
                <h3 id="editing-agents-in-the-ui" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Editing Agents in the UI</h3>
                <p className="text-tokyo-comment mb-4">
                  While agents are created via MCP, you can edit them in the Station UI:
                </p>

                <div
                  className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg overflow-hidden cursor-pointer hover:border-tokyo-purple transition-colors"
                  onClick={() => setLightboxImage("/agent-edit.png")}
                >
                  <img src="/agent-edit.png" alt="Agent Edit Form" className="w-full" />
                </div>
              </div>
            </div>
          </section>

          {/* MCP Servers Section */}
          <section id="mcp-servers" className="mb-12 scroll-mt-8">
            <h2 className="text-2xl font-bold text-tokyo-fg mb-4 flex items-center gap-2">
              <Server className="h-6 w-6 text-tokyo-orange" />
              MCP Servers & Tools
            </h2>

            <div className="space-y-6">
              <div>
                <h3 id="what-are-mcp-servers" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">What are MCP Servers?</h3>
                <p className="text-tokyo-comment mb-4">
                  MCP Servers are external services that provide tools to your agents. Examples:
                </p>
                <ul className="space-y-2 text-tokyo-fg">
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-cyan mt-1">•</span>
                    <span><strong>Filesystem MCP:</strong> Read, write, and search files</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-cyan mt-1">•</span>
                    <span><strong>Ship Security MCP:</strong> 307+ security scanning tools (trivy, gitleaks, semgrep, etc.)</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-cyan mt-1">•</span>
                    <span><strong>AWS MCP:</strong> AWS cost, CloudWatch, EC2, and other AWS services</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-cyan mt-1">•</span>
                    <span><strong>Custom MCP:</strong> Build your own MCP servers for any API or service</span>
                  </li>
                </ul>
              </div>

              <div>
                <h3 id="adding-servers" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Adding MCP Servers</h3>
                <p className="text-tokyo-comment mb-4">
                  MCP servers are configured in each environment's <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">template.json</code> file:
                </p>

                <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-tokyo-comment text-sm font-mono">template.json</span>
                  </div>
                  <pre className="text-tokyo-fg font-mono text-xs overflow-x-auto">
{`{
  "name": "production",
  "description": "Production environment",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem@latest",
        "{{ .PROJECT_ROOT }}"
      ]
    },
    "ship-security": {
      "command": "ship",
      "args": ["mcp", "security", "--stdio"]
    }
  }
}`}
                  </pre>
                </div>
              </div>

              <div>
                <h3 id="tool-discovery" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Tool Discovery</h3>
                <p className="text-tokyo-comment mb-4">
                  After adding MCP servers, run <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">stn sync [environment]</code> to
                  discover all available tools. Tools are automatically namespaced with <code className="bg-tokyo-bg-dark px-1 rounded text-tokyo-cyan">__</code> prefix.
                </p>

                <div className="bg-transparent border border-tokyo-blue rounded-lg p-4">
                  <p className="text-sm text-tokyo-fg mb-2">
                    <strong>Example Tool Names:</strong>
                  </p>
                  <ul className="text-sm text-tokyo-comment font-mono space-y-1">
                    <li>• __read_text_file (from filesystem MCP)</li>
                    <li>• __list_directory (from filesystem MCP)</li>
                    <li>• __checkov_scan_directory (from ship security MCP)</li>
                    <li>• __trivy_scan_filesystem (from ship security MCP)</li>
                  </ul>
                </div>
              </div>
            </div>
          </section>

          {/* Templates & Variables Section */}
          <section id="templates" className="mb-12 scroll-mt-8">
            <h2 className="text-2xl font-bold text-tokyo-fg mb-4 flex items-center gap-2">
              <GitBranch className="h-6 w-6 text-tokyo-red" />
              Templates & Variables
            </h2>

            <div className="space-y-6">
              <div>
                <h3 id="template-system" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Template System</h3>
                <p className="text-tokyo-comment mb-4">
                  Station uses Go template syntax in <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">template.json</code> for
                  dynamic configuration. Variables are defined in <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">variables.yml</code> and
                  rendered at runtime.
                </p>

                <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-tokyo-comment text-sm font-mono">variables.yml</span>
                  </div>
                  <pre className="text-tokyo-fg font-mono text-sm overflow-x-auto">
{`PROJECT_ROOT: "/home/user/projects/my-app"
AWS_REGION: "us-east-1"
ENVIRONMENT: "production"`}
                  </pre>
                </div>

                <div className="mt-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-tokyo-comment text-sm font-mono">template.json (using variables)</span>
                  </div>
                  <pre className="text-tokyo-fg font-mono text-sm overflow-x-auto">
{`{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem@latest",
        "{{ .PROJECT_ROOT }}"
      ]
    }
  }
}`}
                  </pre>
                </div>
              </div>

              <div>
                <h3 id="variables-file" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Why Variables?</h3>
                <p className="text-tokyo-comment mb-4">
                  Variables keep secrets and environment-specific configuration separate from your MCP server definitions:
                </p>
                <ul className="space-y-2 text-tokyo-fg">
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-red mt-1">🔒</span>
                    <span><strong>Security:</strong> Secrets stay in <code className="bg-tokyo-bg-dark px-1 rounded text-tokyo-cyan">variables.yml</code> (gitignored)</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-green mt-1">♻️</span>
                    <span><strong>Reusability:</strong> Share <code className="bg-tokyo-bg-dark px-1 rounded text-tokyo-cyan">template.json</code> as bundles without exposing paths</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-purple mt-1">🌍</span>
                    <span><strong>Portability:</strong> Same template works across dev, staging, and production</span>
                  </li>
                </ul>
              </div>
            </div>
          </section>

          {/* Sync Section */}
          <section id="sync" className="mb-12 scroll-mt-8">
            <h2 className="text-2xl font-bold text-tokyo-fg mb-4 flex items-center gap-2">
              <Database className="h-6 w-6 text-tokyo-blue" />
              Sync Process
            </h2>

            <div className="space-y-6">
              <div>
                <h3 id="what-is-sync" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">What is Sync?</h3>
                <p className="text-tokyo-comment mb-4">
                  Sync connects to your MCP servers, discovers available tools, and prompts for missing template variables.
                  It's required after:
                </p>
                <ul className="space-y-2 text-tokyo-fg">
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-purple mt-1">•</span>
                    <span>Adding or updating MCP servers in <code className="bg-tokyo-bg-dark px-1 rounded text-tokyo-cyan">template.json</code></span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-purple mt-1">•</span>
                    <span>Creating a new environment</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-purple mt-1">•</span>
                    <span>Installing a bundle with new variables</span>
                  </li>
                </ul>
              </div>

              <div>
                <h3 id="when-to-sync" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Interactive Variable Prompts</h3>
                <p className="text-tokyo-comment mb-4">
                  When sync detects missing variables in <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">template.json</code>,
                  the UI automatically prompts you to fill them in:
                </p>

                <div
                  className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg overflow-hidden cursor-pointer hover:border-tokyo-purple transition-colors"
                  onClick={() => setLightboxImage("/env-variables.png")}
                >
                  <img src="/env-variables.png" alt="Sync Variable Prompts" className="w-full" />
                </div>
              </div>

              <div>
                <h3 id="interactive-prompts" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">When to Sync</h3>
                <p className="text-tokyo-comment mb-4">
                  Always sync before running agents to ensure they have access to the latest tools:
                </p>
                <div className="bg-transparent border border-tokyo-blue rounded-lg p-4">
                  <p className="text-sm text-tokyo-fg">
                    💡 <strong>Tip:</strong> You can run <code className="bg-tokyo-bg px-1 rounded text-tokyo-cyan">stn sync [environment]</code>
                    from CLI or use the Sync button in the UI's environment view
                  </p>
                </div>
              </div>
            </div>
          </section>

          {/* Environments Section */}
          <section id="environments" className="mb-12 scroll-mt-8">
            <h2 className="text-2xl font-bold text-tokyo-fg mb-4 flex items-center gap-2">
              <Package className="h-6 w-6 text-tokyo-purple" />
              Environments
            </h2>

            <div className="space-y-6">
              <div>
                <h3 id="organization" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">What are Environments?</h3>
                <p className="text-tokyo-comment mb-4">
                  Environments are isolated workspaces that organize your agents, MCP servers, and tools. Each environment has:
                </p>
                <ul className="space-y-2 text-tokyo-fg">
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-purple mt-1">•</span>
                    <span><strong>Agents:</strong> Agent .prompt files in <code className="bg-tokyo-bg-dark px-1 rounded text-tokyo-cyan">agents/</code> directory</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-purple mt-1">•</span>
                    <span><strong>MCP Servers:</strong> Server configuration in <code className="bg-tokyo-bg-dark px-1 rounded text-tokyo-cyan">template.json</code></span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-purple mt-1">•</span>
                    <span><strong>Variables:</strong> Environment-specific config in <code className="bg-tokyo-bg-dark px-1 rounded text-tokyo-cyan">variables.yml</code></span>
                  </li>
                </ul>
              </div>

              <div>
                <h3 id="multi-environment-setup" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Multi-Environment Organization</h3>
                <p className="text-tokyo-comment mb-4">
                  Common environment patterns:
                </p>

                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-green font-semibold mb-2">Development</h4>
                    <p className="text-sm text-tokyo-fg">
                      Local testing with dev API keys and filesystem access to your projects
                    </p>
                  </div>

                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-cyan font-semibold mb-2">Staging</h4>
                    <p className="text-sm text-tokyo-fg">
                      Pre-production testing with staging credentials and limited tool access
                    </p>
                  </div>

                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-red font-semibold mb-2">Production</h4>
                    <p className="text-sm text-tokyo-fg">
                      Production agents with production API keys and strict tool restrictions
                    </p>
                  </div>
                </div>
              </div>

              <div>
                <h3 id="creating-environments-via-mcp" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Creating Environments via MCP</h3>
                <p className="text-tokyo-comment mb-4">
                  Use the <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">create_environment</code> MCP tool to create new environments.
                  Environment directories are created at <code className="bg-tokyo-bg-dark px-1 rounded text-tokyo-cyan">~/.config/station/environments/[name]/</code>
                </p>
              </div>

              <div>
                <h3 id="copying-environments" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Copying Environments</h3>
                <p className="text-tokyo-comment mb-4">
                  Station provides an environment copy feature to duplicate existing environments with their agents, MCP configurations, and structure.
                  This is useful for creating staging/production clones or testing variations.
                </p>

                <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 mb-4">
                  <h4 className="text-tokyo-fg font-semibold mb-3">Copy Process Steps</h4>
                  <ol className="space-y-3 text-tokyo-fg">
                    <li className="flex items-start gap-3">
                      <span className="text-tokyo-purple font-bold mt-0.5">1.</span>
                      <div>
                        <strong>Create Target Environment:</strong>
                        <p className="text-tokyo-comment text-sm mt-1">
                          First create a new empty environment that will receive the copied configuration.
                          Use <code className="bg-tokyo-bg px-1 rounded text-tokyo-cyan">create_environment</code> with the desired name.
                        </p>
                      </div>
                    </li>
                    <li className="flex items-start gap-3">
                      <span className="text-tokyo-purple font-bold mt-0.5">2.</span>
                      <div>
                        <strong>Copy Environment Configuration:</strong>
                        <p className="text-tokyo-comment text-sm mt-1">
                          Use the <code className="bg-tokyo-bg px-1 rounded text-tokyo-cyan">copy_environment</code> MCP tool to copy agents and
                          MCP server configurations from the source to target environment.
                        </p>
                      </div>
                    </li>
                    <li className="flex items-start gap-3">
                      <span className="text-tokyo-purple font-bold mt-0.5">3.</span>
                      <div>
                        <strong>Handle Conflicts:</strong>
                        <p className="text-tokyo-comment text-sm mt-1 mb-2">
                          If the target environment has existing MCP servers or tools, you'll need to resolve conflicts:
                        </p>
                        <div className="bg-transparent border border-tokyo-orange rounded p-3 text-sm">
                          <p className="text-tokyo-orange mb-2"><strong>⚠️ Common Conflicts:</strong></p>
                          <ul className="text-tokyo-fg space-y-1 ml-4">
                            <li>• <strong>MCP Server Name Conflicts:</strong> Source and target both have a server with the same name</li>
                            <li>• <strong>Tool Assignment Conflicts:</strong> Agents reference tools that don't exist in target</li>
                          </ul>
                          <p className="text-tokyo-comment mt-2">
                            Station will notify you of conflicts during the copy process. You may need to manually merge or rename configurations.
                          </p>
                        </div>
                      </div>
                    </li>
                    <li className="flex items-start gap-3">
                      <span className="text-tokyo-purple font-bold mt-0.5">4.</span>
                      <div>
                        <strong>Sync the Target Environment:</strong>
                        <p className="text-tokyo-comment text-sm mt-1">
                          After copying, run <code className="bg-tokyo-bg px-1 rounded text-tokyo-cyan">stn sync [target-environment]</code> to:
                        </p>
                        <ul className="text-tokyo-fg text-sm space-y-1 mt-2 ml-4">
                          <li>• Connect to MCP servers and discover available tools</li>
                          <li>• Prompt for any missing template variables</li>
                          <li>• Register tools in the target environment's database</li>
                        </ul>
                      </div>
                    </li>
                    <li className="flex items-start gap-3">
                      <span className="text-tokyo-purple font-bold mt-0.5">5.</span>
                      <div>
                        <strong>Re-assign Tools to Agents:</strong>
                        <p className="text-tokyo-comment text-sm mt-1 mb-2">
                          Copied agents retain their tool references from the source environment, but these need to be re-assigned
                          in the target environment's context:
                        </p>
                        <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded p-3 text-sm">
                          <p className="text-tokyo-cyan mb-2"><strong>Why Re-assignment is Needed:</strong></p>
                          <p className="text-tokyo-comment mb-2">
                            Tools are environment-specific database records. Even though the agent's <code className="bg-tokyo-bg px-1 rounded">.prompt</code> file
                            lists tool names, the target environment needs to map those names to its own tool records.
                          </p>
                          <p className="text-tokyo-fg mb-2">
                            <strong>How to Re-assign Tools:</strong>
                          </p>
                          <ul className="text-tokyo-fg space-y-1 ml-4">
                            <li>• Use the <strong>"Assign Tool"</strong> button on the Environments page to bulk-assign tools to agents</li>
                            <li>• Use the UI's agent edit page to manually select tools</li>
                            <li>• Use <code className="bg-tokyo-bg px-1 rounded text-tokyo-cyan">update_agent</code> MCP tool to programmatically assign tools</li>
                          </ul>
                        </div>
                      </div>
                    </li>
                  </ol>
                </div>

                <div className="bg-transparent border border-tokyo-blue rounded-lg p-4">
                  <p className="text-sm text-tokyo-fg">
                    💡 <strong>Pro Tip:</strong> Variables (<code className="bg-tokyo-bg px-1 rounded text-tokyo-cyan">variables.yml</code>)
                    are intentionally NOT copied to keep secrets isolated. You'll need to set environment-specific variables
                    during the sync process or manually configure them afterward.
                  </p>
                </div>
              </div>
            </div>
          </section>

          {/* Bundles Section */}
          <section id="bundles" className="mb-12 scroll-mt-8">
            <h2 className="text-2xl font-bold text-tokyo-fg mb-4 flex items-center gap-2">
              <Package className="h-6 w-6 text-tokyo-green" />
              Bundles & Deployment
            </h2>

            <div className="space-y-6">
              <div>
                <h3 id="what-are-bundles" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">What are Bundles?</h3>
                <p className="text-tokyo-comment mb-4">
                  Bundles are shareable <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">.tar.gz</code> exports of environments
                  containing agents, MCP server configs, and templates (but NOT secrets/variables).
                </p>

                <div className="bg-tokyo-green bg-opacity-20 border border-tokyo-green rounded-lg p-4">
                  <p className="text-sm text-tokyo-fg mb-2">
                    ✅ <strong>Safe to Share:</strong>
                  </p>
                  <ul className="text-sm text-tokyo-comment space-y-1">
                    <li>• Agent .prompt files</li>
                    <li>• template.json (with variable placeholders)</li>
                    <li>• MCP server configurations</li>
                  </ul>
                </div>

                <div className="mt-4 bg-tokyo-red bg-opacity-20 border border-tokyo-red rounded-lg p-4">
                  <p className="text-sm text-tokyo-fg mb-2">
                    ⛔ <strong>NOT Included (Security):</strong>
                  </p>
                  <ul className="text-sm text-tokyo-comment space-y-1">
                    <li>• variables.yml (contains secrets and paths)</li>
                    <li>• API keys or credentials</li>
                    <li>• Environment-specific file paths</li>
                  </ul>
                </div>
              </div>

              <div>
                <h3 id="docker-deployment" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Docker Deployment</h3>
                <p className="text-tokyo-comment mb-4">
                  Station environments can be containerized and deployed with Docker for production use:
                </p>

                <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                  <pre className="text-tokyo-fg font-mono text-sm overflow-x-auto">
{`# Build Docker image with your environment
docker build -t my-station-agents .

# Run Station server with mounted config
docker run -d \\
  -p 8585:8585 \\
  -p 8586:8586 \\
  -v ~/.config/station:/root/.config/station \\
  -e OPENAI_API_KEY=\${OPENAI_API_KEY} \\
  my-station-agents`}
                  </pre>
                </div>

                <div className="mt-4 bg-transparent border border-tokyo-blue rounded-lg p-4">
                  <p className="text-sm text-tokyo-fg">
                    💡 <strong>Production Tip:</strong> Use Docker secrets or environment variables for API keys,
                    never hardcode them in your images
                  </p>
                </div>
              </div>
            </div>
          </section>

          {/* Agent Runs Section */}
          <section id="runs" className="mb-12 scroll-mt-8">
            <h2 className="text-2xl font-bold text-tokyo-fg mb-4 flex items-center gap-2">
              <Play className="h-6 w-6 text-tokyo-cyan" />
              Agent Runs
            </h2>

            <div className="space-y-6">
              <div>
                <h3 id="executing-agents" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Executing Agents via MCP</h3>
                <p className="text-tokyo-comment mb-4">
                  Use the <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">call_agent</code> MCP tool to execute agents.
                  Each execution creates a run record with complete metadata:
                </p>

                <ul className="space-y-2 text-tokyo-fg">
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-cyan mt-1">•</span>
                    <span><strong>Tool Calls:</strong> Every tool invocation with parameters and results</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-cyan mt-1">•</span>
                    <span><strong>Execution Steps:</strong> Step-by-step reasoning and actions</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-cyan mt-1">•</span>
                    <span><strong>Token Usage:</strong> Input/output/total tokens consumed</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-cyan mt-1">•</span>
                    <span><strong>Duration:</strong> Execution time in seconds</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-cyan mt-1">•</span>
                    <span><strong>Final Response:</strong> The agent's output (plain text or structured JSON)</span>
                  </li>
                </ul>
              </div>

              <div>
                <h3 id="run-details" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Viewing Run Details</h3>
                <p className="text-tokyo-comment mb-4">
                  Use <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">inspect_run</code> to get detailed execution metadata,
                  or view runs in the Station UI:
                </p>

                <div
                  className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg overflow-hidden cursor-pointer hover:border-tokyo-purple transition-colors"
                  onClick={() => setLightboxImage("/run.png")}
                >
                  <img src="/run.png" alt="Agent Run Details" className="w-full" />
                </div>
              </div>

              <div>
                <h3 id="structured-output" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Structured Output with JSON Schemas</h3>
                <p className="text-tokyo-comment mb-4">
                  Agents can return structured JSON output by defining an output schema. Example from FinOps use case:
                </p>

                <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                  <pre className="text-tokyo-fg font-mono text-xs overflow-x-auto">
{`{
  "finding": "Significant cost spike in EC2 us-east-1",
  "root_cause": "Auto-scaling triggered by load spike",
  "cost_impact": 14343.30,
  "time_range": {
    "start": "2023-10-23",
    "end": "2023-10-30"
  },
  "affected_resources": [
    {
      "service": "EC2",
      "resource_id": "i-0abc123",
      "cost_increase": 8200.50
    }
  ],
  "recommendations": [
    {
      "action": "Implement reserved instances",
      "estimated_savings": 5000.00
    }
  ]
}`}
                  </pre>
                </div>

                <div className="mt-4 bg-transparent border border-tokyo-blue rounded-lg p-4">
                  <p className="text-sm text-tokyo-fg">
                    💡 <strong>Tip:</strong> Define output schemas in agent .prompt files using JSON Schema format.
                    Station validates and enforces the schema during execution.
                  </p>
                </div>
              </div>
            </div>
          </section>

          {/* CloudShip Section */}
          <section id="cloudship" className="mb-12 scroll-mt-8">
            <h2 className="text-2xl font-bold text-tokyo-fg mb-4 flex items-center gap-2">
              <Database className="h-6 w-6 text-tokyo-purple" />
              CloudShip Integration
            </h2>

            <div className="space-y-6">
              <div>
                <h3 id="data-ingestion" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Data Ingestion Platform</h3>
                <p className="text-tokyo-comment mb-4">
                  CloudShip is a data ingestion platform that collects structured agent outputs for analytics and reporting.
                  Station integrates with CloudShip using <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">app</code> and
                  <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan ml-1">app_type</code> fields.
                </p>
              </div>

              <div>
                <h3 id="output-schemas" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Supported App Types</h3>
                <p className="text-tokyo-comment mb-4">
                  Currently, Station supports the <strong>finops</strong> app with 5 subtypes:
                </p>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-purple font-semibold mb-2">investigations</h4>
                    <p className="text-sm text-tokyo-fg">
                      Cost spike analysis, anomaly detection, and root cause investigations
                    </p>
                  </div>

                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-green font-semibold mb-2">opportunities</h4>
                    <p className="text-sm text-tokyo-fg">
                      Cost optimization recommendations and savings opportunities
                    </p>
                  </div>

                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-cyan font-semibold mb-2">projections</h4>
                    <p className="text-sm text-tokyo-fg">
                      Cost forecasting and budget projections
                    </p>
                  </div>

                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-orange font-semibold mb-2">inventory</h4>
                    <p className="text-sm text-tokyo-fg">
                      Resource inventory and tagging analysis
                    </p>
                  </div>

                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                    <h4 className="text-tokyo-red font-semibold mb-2">governance</h4>
                    <p className="text-sm text-tokyo-fg">
                      Policy compliance and governance reporting
                    </p>
                  </div>
                </div>
              </div>

              <div>
                <h3 id="configuring-cloudship-integration" className="text-xl font-semibold text-tokyo-fg mb-3 scroll-mt-8">Configuring CloudShip Integration</h3>
                <p className="text-tokyo-comment mb-4">
                  Add <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan">app</code> and
                  <code className="bg-tokyo-bg-dark px-2 py-1 rounded text-tokyo-cyan ml-1">app_type</code> to your agent's .prompt file:
                </p>

                <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-4">
                  <pre className="text-tokyo-fg font-mono text-xs overflow-x-auto">
{`---
metadata:
  name: "AWS Cost Spike Analyzer"
  description: "Investigates AWS cost anomalies"
  tags: ["finops", "aws", "investigations"]
  # CloudShip integration
  app: "finops"
  app_type: "investigations"
model: gpt-4o-mini
output:
  schema: |
    {
      "type": "object",
      "required": ["finding", "root_cause", "cost_impact"],
      "properties": {
        "finding": { "type": "string" },
        "root_cause": { "type": "string" },
        "cost_impact": { "type": "number" }
      }
    }
---`}
                  </pre>
                </div>

                <div className="mt-4 bg-transparent border border-tokyo-blue rounded-lg p-4">
                  <p className="text-sm text-tokyo-fg">
                    <strong>Important:</strong> CloudShip integration requires structured output. You MUST define
                    either <code className="bg-tokyo-bg px-1 rounded text-tokyo-cyan">output.schema</code> or use
                    an <code className="bg-tokyo-bg px-1 rounded text-tokyo-cyan">output_schema_preset</code>.
                  </p>
                </div>
              </div>
            </div>
          </section>

          {/* Footer */}
          <div className="mt-12 pt-8 border-t border-tokyo-blue7">
            <p className="text-tokyo-comment text-center">
              Need help? Check out the <a href="https://station.dev/docs" className="text-tokyo-purple hover:text-tokyo-cyan transition-colors">Station Documentation</a> or
              join our <a href="https://discord.gg/station" className="text-tokyo-purple hover:text-tokyo-cyan transition-colors ml-1">Discord Community</a>
            </p>
          </div>
        </div>
      </div>

      {/* Table of Contents Sidebar */}
      <div className="w-64 border-l border-tokyo-blue7 bg-tokyo-bg-dark p-6 overflow-y-auto">
        <h3 className="text-sm font-semibold text-tokyo-comment uppercase mb-4">On This Page</h3>
        <nav className="space-y-1">
          {TABLE_OF_CONTENTS.map((item) => (
            <div key={item.id}>
              <button
                onClick={() => scrollToSection(item.id)}
                className={`block w-full text-left px-3 py-2 rounded text-sm transition-colors ${
                  activeSection === item.id
                    ? 'border border-tokyo-purple text-tokyo-purple font-semibold'
                    : 'text-tokyo-comment hover:text-tokyo-fg hover:border hover:border-tokyo-blue7'
                }`}
              >
                {item.title}
              </button>
              {item.subsections && (
                <div className="ml-4 mt-1 space-y-1">
                  {item.subsections.map((sub) => (
                    <div
                      key={sub}
                      onClick={() => scrollToSubsection(sub)}
                      className="text-xs text-tokyo-comment hover:text-tokyo-fg cursor-pointer py-1"
                    >
                      {sub}
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))}
        </nav>
      </div>

      {/* Image Lightbox Modal */}
      {lightboxImage && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-95 p-8"
          onClick={() => setLightboxImage(null)}
        >
          <div className="relative w-full h-full flex items-center justify-center">
            <button
              onClick={() => setLightboxImage(null)}
              className="absolute top-4 right-4 text-white hover:text-tokyo-purple transition-colors z-10 bg-black bg-opacity-50 rounded-full p-2"
            >
              <X className="h-10 w-10" />
            </button>
            <img
              src={lightboxImage}
              alt="Screenshot"
              className="max-w-[95vw] max-h-[95vh] object-contain rounded-lg shadow-2xl"
              onClick={(e) => e.stopPropagation()}
            />
          </div>
        </div>
      )}
    </div>
  );
};
