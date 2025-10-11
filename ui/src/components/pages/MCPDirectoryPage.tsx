import React, { useState, useEffect } from 'react';
import { Plus, Server, Globe, Database, Terminal, Code, Search, Cloud, Layers, FileText, Settings, AlertCircle, MessageSquare, Package, Shield, ExternalLink } from 'lucide-react';
import { SyncModal } from '../sync/SyncModal';

interface MCPServer {
  id: string;
  name: string;
  description: string;
  category: string;
  command: string;
  args: string[];
  env?: Record<string, string>;
  transport?: string;
  icon: React.ComponentType<any>;
  isInstalled?: boolean;
  requiresShip?: boolean;
}

interface Environment {
  id: string;
  name: string;
}

const mcpServers: MCPServer[] = [
  {
    id: 'tavily',
    name: 'Tavily',
    description: 'Search the web and get search results for a given query',
    category: 'Search & Research',
    command: 'npx',
    args: ['-y', '@mcptools/mcp-tavily'],
    env: { TAVILY_API_KEY: '{{ .TAVILY_API_KEY }}' },
    icon: Search
  },
  {
    id: 'powertools',
    name: 'PowerTools for AWS',
    description: 'AWS utilities and tools for development',
    category: 'Cloud Platform',
    command: 'npx',
    args: ['-y', 'powertools-for-aws-mcp'],
    icon: Cloud
  },
  {
    id: 'e2b-mcp-server',
    name: 'E2B Code Interpreter',
    description: 'Run code in secure, sandboxed cloud environments',
    category: 'Code Execution',
    command: 'uvx',
    args: ['e2b-mcp-server'],
    env: { E2B_API_KEY: '{{ .E2B_API_KEY }}' },
    icon: Code
  },
  {
    id: 'heroku',
    name: 'Heroku',
    description: 'Use Heroku CLI to manage apps on the Heroku platform',
    category: 'Cloud Platform',
    command: 'heroku',
    args: ['mcp:start'],
    icon: Cloud
  },
  {
    id: 'kubernetes',
    name: 'Kubernetes',
    description: 'Manage Kubernetes cluster (pods, deployments, services)',
    category: 'Infrastructure',
    command: 'npx',
    args: ['mcp-server-kubernetes'],
    icon: Layers
  },
  {
    id: 'couchbase',
    name: 'Couchbase',
    description: 'Query and manage Couchbase databases',
    category: 'Database',
    command: 'docker',
    args: ['run', '-i', '--rm', '-e', 'CB_CONNECTION_STRING', '-e', 'CB_USERNAME', '-e', 'CB_BUCKET_NAME', '-e', 'CB_MCP_READ_ONLY_QUERY_MODE', '-e', 'CB_PASSWORD', 'mcp/couchbase'],
    env: {
      CB_CONNECTION_STRING: '{{ .CB_CONNECTION_STRING }}',
      CB_USERNAME: '{{ .CB_USERNAME }}',
      CB_BUCKET_NAME: '{{ .CB_BUCKET_NAME }}',
      CB_MCP_READ_ONLY_QUERY_MODE: '{{ .CB_MCP_READ_ONLY_QUERY_MODE }}',
      CB_PASSWORD: '{{ .CB_PASSWORD }}'
    },
    icon: Database
  },
  {
    id: 'terraform',
    name: 'Terraform',
    description: 'Manage infrastructure with Terraform',
    category: 'Infrastructure',
    command: 'docker',
    args: ['run', '-i', '--rm', 'hashicorp/terraform-mcp-server'],
    icon: Layers
  },
  {
    id: 'sqlite-mcp-server',
    name: 'SQLite MCP Server',
    description: 'Read and write data in SQLite databases',
    category: 'Database',
    command: 'uvx',
    args: ['mcp-server-sqlite@git+https://github.com/neverinfamous/mcp_server_sqlite.git'],
    icon: Database
  },
  {
    id: 'argocd-mcp',
    name: 'ArgoCD',
    description: 'Manage ArgoCD applications and repositories',
    category: 'Infrastructure',
    command: 'npx',
    args: ['argocd-mcp@latest', 'stdio'],
    env: {
      ARGOCD_BASE_URL: '{{ .ARGOCD_BASE_URL }}',
      ARGOCD_API_TOKEN: '{{ .ARGOCD_API_TOKEN }}'
    },
    icon: Layers
  },
  {
    id: 'supabase',
    name: 'Supabase',
    description: 'Interact with Supabase projects and databases',
    category: 'Database',
    command: 'npx',
    args: ['-y', '@supabase/mcp-server-supabase@latest', '--access-token', '{{ .SUPABASE_ACCESS_TOKEN }}'],
    icon: Database
  },
  {
    id: 'github',
    name: 'GitHub',
    description: 'Create issues, search repositories, manage files and more',
    category: 'Development Tools',
    command: 'npx',
    args: ['-y', '@modelcontextprotocol/server-github'],
    env: { GITHUB_PERSONAL_ACCESS_TOKEN: '{{ .GITHUB_PERSONAL_ACCESS_TOKEN }}' },
    transport: 'http',
    icon: Code
  },
  {
    id: 'dbhub-postgres-npx',
    name: 'DBHub Postgres',
    description: 'Connect to Postgres databases via DBHub',
    category: 'Database',
    command: 'npx',
    args: ['-y', '@bytebase/dbhub', '--transport', 'stdio', '--dsn', '{{ .POSTGRES_DSN }}'],
    icon: Database
  },
  {
    id: 'python-repl',
    name: 'Python REPL',
    description: 'Execute Python code in a REPL environment',
    category: 'Code Execution',
    command: 'uv',
    args: ['--directory', '{{ .PYTHON_REPL_PATH }}', 'run', 'mcp_python'],
    icon: Code
  },
  {
    id: 'docling',
    name: 'Docling',
    description: 'Convert various document formats to text and structured formats',
    category: 'Document Processing',
    command: 'uvx',
    args: ['--from=docling-mcp', 'docling-mcp-server'],
    icon: FileText
  },
  {
    id: 'sentry',
    name: 'Sentry',
    description: 'Query Sentry for error tracking and performance monitoring',
    category: 'Monitoring',
    command: 'npx',
    args: ['-y', '@sentry/mcp-server@latest', '--access-token={{ .SENTRY_AUTH_TOKEN }}'],
    icon: AlertCircle
  },
  {
    id: 'perplexity-ask',
    name: 'Perplexity',
    description: 'Search and ask questions using Perplexity AI',
    category: 'Search & Research',
    command: 'npx',
    args: ['-y', 'server-perplexity-ask'],
    env: { PERPLEXITY_API_KEY: '{{ .PERPLEXITY_API_KEY }}' },
    icon: Search
  },
  {
    id: 'posthog',
    name: 'PostHog',
    description: 'Query PostHog for analytics insights and user behavior data',
    category: 'Analytics',
    command: 'npx',
    args: ['-y', 'mcp-remote@latest', 'https://mcp.posthog.com/mcp', '--header', 'Authorization:{{ .POSTHOG_AUTH_HEADER }}'],
    env: { POSTHOG_AUTH_HEADER: '{{ .POSTHOG_AUTH_HEADER }}' },
    icon: Settings
  },
  {
    id: 'fetch',
    name: 'Fetch',
    description: 'Fetch and extract content from web pages',
    category: 'Web Scraping',
    command: 'uvx',
    args: ['mcp-server-fetch'],
    icon: Globe
  },
  {
    id: 'stripe',
    name: 'Stripe',
    description: 'Access Stripe API for payment processing and financial data',
    category: 'Financial',
    command: 'npx',
    args: ['-y', '@stripe/mcp', '--tools=all', '--api-key={{ .STRIPE_SECRET_KEY }}'],
    icon: Settings
  },
  {
    id: 'notionMCP',
    name: 'Notion',
    description: 'Read and search through Notion pages and databases',
    category: 'Productivity',
    command: 'npx',
    args: ['-y', 'mcp-remote', 'https://mcp.notion.com/sse'],
    icon: FileText
  },
  {
    id: 'aws-knowledge-mcp',
    name: 'AWS Knowledge',
    description: 'Search AWS documentation and get answers about AWS services',
    category: 'Cloud Platform',
    command: 'npx',
    args: ['-y', 'mcp-remote', 'https://knowledge-mcp.global.api.aws'],
    transport: 'http',
    icon: Cloud
  },
  {
    id: 'desktop-commander',
    name: 'Desktop Commander',
    description: 'Control your desktop environment and applications',
    category: 'System Control',
    command: 'npx',
    args: ['-y', '@wonderwhy-er/desktop-commander@latest'],
    icon: Terminal
  },
  {
    id: 'resend',
    name: 'Resend',
    description: 'Send emails using the Resend API',
    category: 'Communication',
    command: 'node',
    args: ['{{ .RESEND_MCP_PATH }}/build/index.js', '--key={{ .RESEND_API_KEY }}'],
    icon: MessageSquare
  },
  {
    id: 'upstash',
    name: 'Upstash',
    description: 'Interact with Upstash Redis and Kafka services',
    category: 'Database',
    command: 'npx',
    args: ['-y', '@upstash/mcp-server', 'run', '{{ .UPSTASH_EMAIL }}', '{{ .UPSTASH_API_KEY }}'],
    icon: Database
  },
  {
    id: 'playwright',
    name: 'Playwright',
    description: 'Browser automation and web testing framework',
    category: 'Development Tools',
    command: 'npx',
    args: ['@playwright/mcp@latest'],
    icon: Code
  },
  {
    id: 'awslabs.cost-explorer-mcp-server',
    name: 'AWS Cost Explorer',
    description: 'Analyze AWS costs and usage with Cost Explorer API',
    category: 'Cloud Platform',
    command: 'uvx',
    args: ['awslabs.cost-explorer-mcp-server@latest'],
    env: {
      FASTMCP_LOG_LEVEL: '{{ .FASTMCP_LOG_LEVEL }}',
      AWS_PROFILE: '{{ .AWS_PROFILE }}'
    },
    icon: Cloud
  },
  // Ship Security Tools
  {
    id: 'ship-security',
    name: 'Ship Security (All)',
    description: 'All 31 security tools: Trivy, GitLeaks, Semgrep, Grype, TruffleHog, and more',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'security'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-terraform',
    name: 'Ship Terraform',
    description: 'All 11 Terraform tools: TFLint, Checkov, terraform-docs, TFSec, and more',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'terraform'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-kubernetes',
    name: 'Ship Kubernetes',
    description: 'All Kubernetes tools: Kubescape, Kube-bench, Velero, Falco, Kyverno',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'kubernetes'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-cloud',
    name: 'Ship Cloud',
    description: 'Cloud security tools: Prowler, Scout-suite, CloudQuery, Custodian',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'cloud'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-aws-iam',
    name: 'Ship AWS IAM',
    description: 'AWS IAM security: Cloudsplaining, Parliament, PMMapper, Policy Sentry',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'aws-iam'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-gitleaks',
    name: 'GitLeaks',
    description: 'Detect secrets and sensitive information in code repositories',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'gitleaks'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-trivy',
    name: 'Trivy',
    description: 'Comprehensive security scanner for containers and filesystems',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'trivy'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-semgrep',
    name: 'Semgrep',
    description: 'Static analysis security scanning for code',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'semgrep'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-checkov',
    name: 'Checkov',
    description: 'Infrastructure as Code security scanning',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'checkov'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-tflint',
    name: 'TFLint',
    description: 'Terraform linting for syntax and best practices',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'tflint'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-grype',
    name: 'Grype',
    description: 'Vulnerability scanning with Anchore Grype',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'grype'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-kubescape',
    name: 'Kubescape',
    description: 'Kubernetes security scanning and compliance',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'kubescape'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-all',
    name: 'Ship All Tools',
    description: 'All 86 Ship tools across security, infrastructure, cloud, and development',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'all'],
    icon: Shield,
    requiresShip: true
  }
];

interface MCPServerCardProps {
  server: MCPServer;
  onAddServer: (server: MCPServer) => void;
  disabled?: boolean;
}

const MCPServerCard: React.FC<MCPServerCardProps> = ({ server, onAddServer, disabled }) => {
  const Icon = server.icon;

  return (
    <div className={`bg-gray-800 rounded-lg border border-gray-700 shadow-sm p-6 ${disabled ? 'opacity-50' : ''}`}>
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center space-x-3">
          <div className="p-2 bg-purple-900 rounded-lg">
            <Icon size={24} className="text-purple-400" />
          </div>
          <div>
            <h3 className="text-lg font-semibold text-gray-100">{server.name}</h3>
            <span className="text-sm text-gray-400">{server.category}</span>
          </div>
        </div>
        {server.isInstalled && (
          <span className="px-2 py-1 text-xs bg-green-100 text-green-800 rounded-full">
            Installed
          </span>
        )}
      </div>

      <p className="text-gray-300 mb-4">{server.description}</p>

      <div className="space-y-2 mb-4">
        <div className="text-sm">
          <span className="font-medium text-gray-300">Command:</span>
          <code className="ml-2 px-2 py-1 bg-gray-700 text-gray-300 rounded text-xs">
            {server.command} {server.args.join(' ')}
          </code>
        </div>

        {server.env && Object.keys(server.env).length > 0 && (
          <div className="text-sm">
            <span className="font-medium text-gray-300">Environment:</span>
            <div className="mt-1 space-y-1">
              {Object.entries(server.env).map(([key, value]) => (
                <div key={key} className="flex items-center space-x-2">
                  <code className="px-2 py-1 bg-gray-100 rounded text-xs">
                    {key}={value}
                  </code>
                  {value.includes('{{ .') && (
                    <AlertCircle size={14} className="text-amber-500" title="Requires configuration" />
                  )}
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      <button
        onClick={() => onAddServer(server)}
        disabled={disabled || server.isInstalled}
        className={`w-full flex items-center justify-center space-x-2 px-4 py-2 rounded-lg font-medium ${
          disabled || server.isInstalled
            ? 'bg-gray-700 text-gray-500 cursor-not-allowed'
            : 'bg-purple-600 text-white hover:bg-purple-700'
        }`}
      >
        <Plus size={16} />
        <span>{server.isInstalled ? 'Already Installed' : 'Add Server'}</span>
      </button>
    </div>
  );
};

interface AddServerModalProps {
  server: MCPServer | null;
  environments: Environment[];
  onClose: () => void;
  onConfirm: (serverId: string, environmentId: string, config: any) => void;
}

const AddServerModal: React.FC<AddServerModalProps> = ({ server, environments, onClose, onConfirm }) => {
  const [selectedEnvironment, setSelectedEnvironment] = useState<string>('');
  const [config, setConfig] = useState<any>({});

  useEffect(() => {
    if (server) {
      setConfig({
        command: server.command,
        args: server.args,
        env: server.env || {},
        transport: server.transport || 'stdio'
      });
    }
  }, [server]);

  if (!server) return null;

  const handleConfirm = () => {
    if (selectedEnvironment) {
      onConfirm(server.id, selectedEnvironment, config);
      onClose();
    }
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-gray-800 rounded-lg p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto border border-gray-700">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-semibold text-white">Add {server.name} Server</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-200">
            ×
          </button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">
              Select Environment
            </label>
            <select
              value={selectedEnvironment}
              onChange={(e) => setSelectedEnvironment(e.target.value)}
              className="w-full border border-gray-600 rounded-lg px-3 py-2 bg-gray-700 text-white"
            >
              <option value="">Choose environment...</option>
              {Array.isArray(environments) && environments.map(env => (
                <option key={env.id} value={env.id}>{env.name}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">
              Server Configuration
            </label>
            <div className="bg-gray-700 p-4 rounded-lg border border-gray-600">
              <pre className="text-sm text-gray-300">
                {JSON.stringify(config, null, 2)}
              </pre>
            </div>
          </div>

          {server.env && Object.keys(server.env).length > 0 && (
            <div className="bg-yellow-900 border border-yellow-700 rounded-lg p-4">
              <div className="flex items-center space-x-2 mb-2">
                <AlertCircle size={16} className="text-yellow-400" />
                <span className="text-sm font-medium text-yellow-300">Configuration Required</span>
              </div>
              <p className="text-sm text-yellow-200">
                This server requires environment variables to be configured. Please update the values after installation.
              </p>
            </div>
          )}
        </div>

        <div className="flex justify-end space-x-3 mt-6">
          <button
            onClick={onClose}
            className="px-4 py-2 text-gray-300 border border-gray-600 rounded-lg hover:bg-gray-700"
          >
            Cancel
          </button>
          <button
            onClick={handleConfirm}
            disabled={!selectedEnvironment}
            className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:bg-gray-600 disabled:text-gray-400"
          >
            Add Server
          </button>
        </div>
      </div>
    </div>
  );
};

export const MCPDirectoryPage: React.FC = () => {
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [servers, setServers] = useState<MCPServer[]>(mcpServers);
  const [selectedServer, setSelectedServer] = useState<MCPServer | null>(null);
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedCategory, setSelectedCategory] = useState('All');
  const [loading, setLoading] = useState(true);
  const [shipInstalled, setShipInstalled] = useState(false);
  const [checkingShip, setCheckingShip] = useState(true);
  const [syncModalOpen, setSyncModalOpen] = useState(false);
  const [syncEnvironment, setSyncEnvironment] = useState<string>('');

  // Build categories from servers, ensuring Ship Security Tools is always included
  const serverCategories = Array.from(new Set(mcpServers.map(s => s.category)));
  if (!serverCategories.includes('Ship Security Tools')) {
    serverCategories.push('Ship Security Tools');
  }
  const categories = ['All', ...serverCategories];

  useEffect(() => {
    fetchEnvironments();
    fetchInstalledServers();
    checkShipInstallation();
  }, []);

  const fetchEnvironments = async () => {
    try {
      const response = await fetch('/api/v1/environments');
      const data = await response.json();
      // Handle both data.environments and direct array response
      const environmentsData = data.environments || data || [];
      setEnvironments(Array.isArray(environmentsData) ? environmentsData : []);
    } catch (error) {
      console.error('Failed to fetch environments:', error);
      setEnvironments([]);
    }
  };

  const fetchInstalledServers = async () => {
    try {
      // This would fetch currently installed servers to mark them as installed
      // For now, we'll mark them as not installed
      setLoading(false);
    } catch (error) {
      console.error('Failed to fetch installed servers:', error);
      setLoading(false);
    }
  };

  const checkShipInstallation = async () => {
    try {
      const response = await fetch('/api/v1/ship/installed');
      const data = await response.json();
      setShipInstalled(data.installed || false);
    } catch (error) {
      console.error('Failed to check Ship installation:', error);
      setShipInstalled(false);
    } finally {
      setCheckingShip(false);
    }
  };

  const handleAddServer = (server: MCPServer) => {
    setSelectedServer(server);
  };

  const handleConfirmAddServer = async (serverId: string, environmentId: string, config: any) => {
    try {
      const response = await fetch(`/api/v1/environments/${environmentId}/mcp-servers`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          server_name: serverId,
          config: config
        }),
      });

      if (response.ok) {
        // Mark server as installed and refresh
        setServers(prev => prev.map(s =>
          s.id === serverId ? { ...s, isInstalled: true } : s
        ));

        // Find environment name and trigger sync
        const environment = environments.find(env => env.id === environmentId);
        if (environment) {
          setSyncEnvironment(environment.name);
          setSyncModalOpen(true);
        } else {
          alert('Server added successfully!');
        }
      } else {
        const errorData = await response.text();
        alert(`Failed to add server: ${errorData}`);
      }
    } catch (error) {
      console.error('Failed to add server:', error);
      alert('Failed to add server');
    }
  };

  const handleSyncComplete = () => {
    // Refresh environments or any other data if needed
    fetchEnvironments();
  };

  const filteredServers = servers.filter(server => {
    const matchesSearch = server.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
                         server.description.toLowerCase().includes(searchTerm.toLowerCase());
    const matchesCategory = selectedCategory === 'All' || server.category === selectedCategory;
    // Filter out Ship servers if Ship is not installed
    const isShipAvailable = server.requiresShip ? shipInstalled : true;
    return matchesSearch && matchesCategory && isShipAvailable;
  });

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading MCP Directory...</div>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col bg-gray-900">
      {/* Header */}
      <div className="flex-shrink-0 p-6 bg-gray-800 border-b border-gray-700">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-gray-100 mb-2">MCP Directory</h1>
          <p className="text-gray-400">Discover and install MCP servers to extend your Station capabilities</p>
        </div>

        <div className="mb-6">
          <input
            type="text"
            placeholder="Search servers..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full px-4 py-2 bg-gray-800 border border-gray-700 text-gray-100 rounded-lg focus:ring-2 focus:ring-purple-500 focus:border-transparent"
          />
        </div>

        {/* Category Tabs */}
        <div className="flex flex-wrap gap-2 mb-6 border-b border-gray-700 pb-4">
          {categories.map(category => (
            <button
              key={category}
              onClick={() => setSelectedCategory(category)}
              className={`px-4 py-2 rounded-lg font-medium transition-colors ${
                selectedCategory === category
                  ? 'bg-purple-600 text-white'
                  : 'bg-gray-800 text-gray-400 hover:bg-gray-700 hover:text-gray-200'
              }`}
            >
              {category}
            </button>
          ))}
        </div>
      </div>

      {/* Scrollable Content */}
      <div className="flex-1 overflow-y-auto p-6 space-y-8">
        {filteredServers.length === 0 ? (
          <div className="text-center py-12">
            <Server size={48} className="mx-auto text-gray-400 mb-4" />
            <h3 className="text-lg font-medium text-gray-100 mb-2">No servers found</h3>
            <p className="text-gray-400">Try adjusting your search or category filter</p>
          </div>
        ) : (
          <>
            {/* Show Ship installation prompt if Ship category selected and Ship not installed */}
            {selectedCategory === 'Ship Security Tools' && !shipInstalled && !checkingShip ? (
              <div className="bg-gray-800 border border-gray-700 rounded-lg p-8 text-center max-w-2xl mx-auto">
                <Shield size={64} className="mx-auto text-purple-500 mb-4" />
                <h3 className="text-2xl font-bold text-gray-100 mb-3">Ship CLI Required</h3>
                <p className="text-gray-300 mb-6 text-lg">
                  Install Ship to access 300+ security tools including GitLeaks, Semgrep, Checkov, TFLint, Trivy, and more.
                </p>
                <div className="bg-gray-900 border border-gray-700 rounded-lg p-4 mb-6 font-mono text-left overflow-x-auto">
                  <code className="text-sm text-green-400">curl -fsSL https://ship.cloudship.ai/install.sh | sh</code>
                </div>
                <a
                  href="https://github.com/cloudshipai/ship"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 px-6 py-3 bg-purple-600 hover:bg-purple-700 text-white rounded-lg font-medium transition-colors"
                >
                  <span>Learn More About Ship</span>
                  <ExternalLink size={18} />
                </a>
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                {filteredServers.map(server => (
                  <MCPServerCard
                    key={server.id}
                    server={server}
                    onAddServer={handleAddServer}
                  />
                ))}
              </div>
            )}
          </>
        )}
      </div>

      <AddServerModal
        server={selectedServer}
        environments={environments}
        onClose={() => setSelectedServer(null)}
        onConfirm={handleConfirmAddServer}
      />

      <SyncModal
        isOpen={syncModalOpen}
        onClose={() => setSyncModalOpen(false)}
        environment={syncEnvironment}
        onSyncComplete={handleSyncComplete}
      />
    </div>
  );
};