import React, { useState, useEffect } from 'react';
import { Plus, Server, Globe, Database, Terminal, Code, Search, Cloud, Layers, FileText, Settings, AlertCircle, MessageSquare, Package, Shield, ExternalLink, Wand2, X } from 'lucide-react';
import { SyncModal } from '../sync/SyncModal';
import { FakerBuilderModal } from '../modals/FakerBuilderModal';

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
  openapiSpec?: string;           // OpenAPI spec filename
  requiresOpenAPISpec?: boolean;  // Whether this template requires OpenAPI spec
}

interface Environment {
  id: number;
  name: string;
}

const mcpServers: MCPServer[] = [
  // Ship Security Tools
  {
    id: 'ship-gitleaks',
    name: 'GitLeaks',
    description: 'Fast secret scanning for git repositories (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'gitleaks'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-trufflehog',
    name: 'TruffleHog',
    description: 'Advanced secret scanning with verification (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'trufflehog'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-trivy',
    name: 'Trivy',
    description: 'Comprehensive vulnerability scanner for containers and filesystems (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'trivy'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-syft',
    name: 'Syft',
    description: 'SBOM (Software Bill of Materials) generation tool (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'syft'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-grype',
    name: 'Grype',
    description: 'Vulnerability scanner for container images and filesystems (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'grype'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-semgrep',
    name: 'Semgrep',
    description: 'Static analysis security scanning for code (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'semgrep'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-checkov',
    name: 'Checkov',
    description: 'Infrastructure as Code security scanning (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'checkov'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-terrascan',
    name: 'Terrascan',
    description: 'Infrastructure as Code security scanner (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'terrascan'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-tfsec',
    name: 'TFSec',
    description: 'Terraform-specific security scanner (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'tfsec'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-tflint',
    name: 'TFLint',
    description: 'Terraform linter for syntax and best practices (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'tflint'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-terraform-docs',
    name: 'Terraform Docs',
    description: 'Terraform documentation generator (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'terraform-docs'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-nuclei',
    name: 'Nuclei',
    description: 'Fast vulnerability scanner with community templates (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'nuclei'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-dockle',
    name: 'Dockle',
    description: 'Container image linter for security best practices (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'dockle'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-kubescape',
    name: 'Kubescape',
    description: 'Kubernetes security scanner and compliance checker (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'kubescape'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-kube-bench',
    name: 'Kube-bench',
    description: 'Kubernetes CIS benchmark security scanner (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'kube-bench'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-kube-hunter',
    name: 'Kube-hunter',
    description: 'Kubernetes penetration testing tool (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'kube-hunter'],
    icon: Shield,
    requiresShip: true
  },
  {
    id: 'ship-cosign',
    name: 'Cosign',
    description: 'Container signing and verification for supply chain security (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Ship Security Tools',
    command: 'ship',
    args: ['mcp', 'cosign'],
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
    <div className={`bg-white rounded-xl border border-gray-200/60 shadow-sm hover:shadow-md hover:-translate-y-1 transition-all duration-300 p-5 ${disabled ? 'opacity-50' : ''}`}>
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center space-x-3">
          <div className="p-2 bg-purple-50 rounded-lg">
            <Icon size={20} className="text-purple-600" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <h3 className="text-base font-semibold text-gray-900">{server.name}</h3>
              {server.requiresOpenAPISpec && (
                <span className="px-2 py-0.5 text-[10px] bg-blue-50 text-blue-700 rounded-md border border-blue-200/60 font-medium" title={`OpenAPI Spec: ${server.openapiSpec}`}>
                  OpenAPI
                </span>
              )}
            </div>
            <span className="text-xs text-gray-600">{server.category}</span>
          </div>
        </div>
        {server.isInstalled && (
          <span className="px-2.5 py-1 text-xs bg-emerald-50 text-emerald-700 rounded-full font-medium border border-emerald-200/60">
            Installed
          </span>
        )}
      </div>

      <p className="text-sm text-gray-600 mb-4 leading-relaxed line-clamp-2">{server.description}</p>

      <div className="space-y-2.5 mb-4">
        <div className="text-sm">
          <span className="font-medium text-gray-700 text-xs">Command:</span>
          <code className="mt-1 block px-3 py-2 bg-gray-50/50 text-gray-800 rounded-lg text-xs font-mono border border-gray-200/60">
            {server.command} {server.args?.join(' ') || ''}
          </code>
        </div>

        {server.env && Object.keys(server.env).length > 0 && (
          <div className="text-sm">
            <span className="font-medium text-gray-700 text-xs">Environment:</span>
            <div className="mt-1.5 space-y-1">
              {Object.entries(server.env).map(([key, value]) => (
                <div key={key} className="flex items-center space-x-2">
                  <code className="px-2 py-1 bg-gray-50 rounded text-xs font-mono border border-gray-200/60">
                    {key}={value}
                  </code>
                  {value.includes('{{ .') && (
                    <AlertCircle size={14} className="text-amber-600" title="Requires configuration" />
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
        className={`w-full flex items-center justify-center space-x-2 px-4 py-2.5 rounded-lg font-medium text-sm transition-all duration-200 ${
          disabled || server.isInstalled
            ? 'bg-gray-100 text-gray-400 cursor-not-allowed border border-gray-200/60'
            : 'bg-gray-900 text-white hover:bg-gray-800 hover:shadow-md hover:-translate-y-0.5 active:scale-95 shadow-sm'
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
    <div className="fixed inset-0 bg-black/30 flex items-center justify-center z-50 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-white rounded-xl p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto border border-gray-200/60 shadow-lg animate-in zoom-in-95 fade-in slide-in-from-bottom-4 duration-300">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-semibold text-gray-900">Add {server.name} Server</h2>
          <button onClick={onClose} className="p-2 text-gray-400 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-all">
            <X size={20} />
          </button>
        </div>

        <div className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Select Environment
            </label>
            <select
              value={selectedEnvironment}
              onChange={(e) => setSelectedEnvironment(e.target.value)}
              className="w-full border border-gray-200 rounded-lg px-3 py-2.5 bg-white text-gray-900 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900/10 focus:border-gray-300 transition-all"
            >
              <option value="">Choose environment...</option>
              {Array.isArray(environments) && environments.map(env => (
                <option key={env.id} value={env.id}>{env.name}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Server Configuration
            </label>
            <div className="bg-gray-50/50 p-4 rounded-lg border border-gray-200/60">
              <pre className="text-xs text-gray-800 font-mono overflow-x-auto">
                {JSON.stringify(config, null, 2)}
              </pre>
            </div>
          </div>

          {server.env && Object.keys(server.env).length > 0 && (
            <div className="bg-amber-50 border border-amber-200/60 rounded-lg p-4">
              <div className="flex items-center space-x-2 mb-2">
                <AlertCircle size={16} className="text-amber-600" />
                <span className="text-sm font-medium text-amber-900">Configuration Required</span>
              </div>
              <p className="text-sm text-amber-700 leading-relaxed">
                This server requires environment variables to be configured. Please update the values after installation.
              </p>
            </div>
          )}
        </div>

        <div className="flex justify-end space-x-3 mt-6">
          <button
            onClick={onClose}
            className="px-4 py-2.5 text-gray-700 bg-white border border-gray-200 rounded-lg hover:bg-gray-50 transition-all text-sm font-medium"
          >
            Cancel
          </button>
          <button
            onClick={handleConfirm}
            disabled={!selectedEnvironment}
            className="px-4 py-2.5 bg-gray-900 text-white rounded-lg hover:bg-gray-800 hover:shadow-md hover:-translate-y-0.5 disabled:bg-gray-200 disabled:text-gray-400 disabled:hover:shadow-none disabled:hover:translate-y-0 transition-all duration-200 active:scale-95 text-sm font-medium shadow-sm"
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
  const [fakerModalOpen, setFakerModalOpen] = useState(false);
  const [selectedEnvironmentForFaker, setSelectedEnvironmentForFaker] = useState<number>(0);

  // Build categories from servers, excluding Ship Security Tools
  const serverCategories = Array.from(new Set(mcpServers.map(s => s.category)))
    .filter(cat => cat !== 'Ship Security Tools');
  const categories = ['All', ...serverCategories];

  useEffect(() => {
    fetchEnvironments();
    fetchInstalledServers();
    checkShipInstallation();
    fetchDirectoryTemplates();
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

  const fetchDirectoryTemplates = async () => {
    try {
      const response = await fetch('/api/v1/directory/templates');
      const data = await response.json();

      // Map API templates to MCPServer format
      const mappedTemplates: MCPServer[] = (data.templates || []).map((template: any) => ({
        id: template.id,
        name: template.name,
        description: template.description,
        category: template.category,
        command: template.command,
        args: template.args || [], // Ensure args is always an array
        env: template.env,
        icon: Package, // Use Package icon for directory templates
        isInstalled: false,
        requiresShip: false,
        openapiSpec: template.openapiSpec,
        requiresOpenAPISpec: template.requiresOpenAPISpec || false,
      }));

      // Merge with existing hardcoded servers
      setServers(prev => [...prev, ...mappedTemplates]);
    } catch (error) {
      console.error('Failed to fetch directory templates:', error);
    }
  };

  const handleAddServer = (server: MCPServer) => {
    setSelectedServer(server);
  };

  const handleConfirmAddServer = async (serverId: string, environmentId: string, config: any) => {
    try {
      // Check if this is an OpenAPI template
      const server = servers.find(s => s.id === serverId);
      const isOpenAPITemplate = server?.requiresOpenAPISpec;

      let response;
      if (isOpenAPITemplate) {
        // Use the new OpenAPI template installation endpoint
        response = await fetch(`/api/v1/directory/templates/${serverId}/install`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            environment_id: parseInt(environmentId),
            config: config,
            openapi_spec_file: server?.openapiSpec || ''
          }),
        });
      } else {
        // Use the regular MCP server installation endpoint
        response = await fetch(`/api/v1/environments/${environmentId}/mcp-servers`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            server_name: serverId,
            config: config
          }),
        });
      }

      if (response.ok) {
        // Mark server as installed and refresh
        setServers(prev => prev.map(s =>
          s.id === serverId ? { ...s, isInstalled: true } : s
        ));

        // Find environment name and trigger sync
        // Convert environmentId to number for comparison since API returns numeric IDs
        const environment = environments.find(env => env.id === parseInt(environmentId));
        if (environment) {
          setSyncEnvironment(environment.name);
          setSyncModalOpen(true);
        } else {
          console.error('Environment not found for ID:', environmentId);
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

  const handleFakerCreated = (fakerName: string, envName: string) => {
    // Trigger sync automatically
    setSyncEnvironment(envName);
    setSyncModalOpen(true);
  };

  const handleOpenFakerBuilder = () => {
    if (environments.length === 0) {
      alert('No environments available. Please create an environment first.');
      return;
    }
    // Default to first environment
    setSelectedEnvironmentForFaker(environments[0].id);
    setFakerModalOpen(true);
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
      <div className="flex items-center justify-center h-64 bg-[#fafaf8]">
        <div className="text-gray-600 text-sm font-medium animate-pulse">Loading MCP Directory...</div>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col bg-[#fafaf8]">
      {/* Header - Paper matte style */}
      <div className="flex-shrink-0 px-6 py-5 border-b border-gray-200/60 bg-white/60 backdrop-blur-sm">
        <div className="mb-6 animate-in fade-in slide-in-from-top-2 duration-300">
          <h1 className="text-2xl font-medium text-gray-900 mb-2">MCP Directory</h1>
          <p className="text-sm text-gray-600">Discover and install MCP servers to extend your Station capabilities</p>
        </div>

        <div className="mb-6 flex items-center gap-3 animate-in fade-in slide-in-from-top-2 duration-300 delay-100">
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input
              type="text"
              placeholder="Search servers..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full pl-9 pr-3 py-2.5 bg-gray-50 border border-gray-200 text-gray-900 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-gray-900/10 focus:border-gray-300 focus:bg-white transition-all duration-200 placeholder:text-gray-400"
            />
          </div>
          <button
            onClick={handleOpenFakerBuilder}
            className="flex items-center gap-2 px-4 py-2.5 bg-gray-900 hover:bg-gray-800 hover:shadow-lg hover:-translate-y-0.5 text-white rounded-lg text-sm font-medium transition-all duration-200 active:scale-95 shadow-sm whitespace-nowrap"
          >
            <Wand2 className="h-4 w-4" />
            Create Faker
          </button>
        </div>

        {/* Category Tabs - Clean pill style */}
        <div className="flex flex-wrap gap-2 animate-in fade-in slide-in-from-top-2 duration-300 delay-200">
          {categories.map(category => (
            <button
              key={category}
              onClick={() => setSelectedCategory(category)}
              className={`px-3.5 py-2 rounded-lg text-sm font-medium transition-all duration-200 ${
                selectedCategory === category
                  ? 'bg-gray-900 text-white shadow-sm scale-105'
                  : 'bg-white text-gray-700 hover:bg-gray-100 hover:scale-105 border border-gray-200/60 shadow-sm'
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
          <div className="text-center py-16 animate-in fade-in zoom-in duration-500">
            <Server size={48} className="mx-auto text-gray-300 mb-4 animate-in zoom-in duration-500 delay-100" />
            <h3 className="text-lg font-medium text-gray-900 mb-2 animate-in fade-in slide-in-from-bottom-2 duration-500 delay-200">No servers found</h3>
            <p className="text-sm text-gray-600 animate-in fade-in duration-500 delay-300">Try adjusting your search or category filter</p>
          </div>
        ) : (
          <>
            {/* Show Ship installation prompt if Ship category selected and Ship not installed */}
            {selectedCategory === 'Ship Security Tools' && !shipInstalled && !checkingShip ? (
              <div className="bg-white border border-gray-200/60 rounded-xl p-8 text-center max-w-2xl mx-auto shadow-sm animate-in fade-in zoom-in-95 duration-500">
                <Shield size={64} className="mx-auto text-purple-600 mb-4" />
                <h3 className="text-2xl font-semibold text-gray-900 mb-3">Ship CLI Required</h3>
                <p className="text-gray-600 mb-6 text-base leading-relaxed">
                  Install Ship to access 300+ security tools including GitLeaks, Semgrep, Checkov, TFLint, Trivy, and more.
                </p>
                <div className="bg-gray-50 border border-gray-200/60 rounded-lg p-4 mb-6 font-mono text-left overflow-x-auto">
                  <code className="text-sm text-emerald-600">curl -fsSL https://ship.cloudship.ai/install.sh | sh</code>
                </div>
                <a
                  href="https://github.com/cloudshipai/ship"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 px-6 py-3 bg-gray-900 hover:bg-gray-800 hover:shadow-lg hover:-translate-y-0.5 text-white rounded-lg text-sm font-medium transition-all duration-200 active:scale-95 shadow-sm"
                >
                  <span>Learn More About Ship</span>
                  <ExternalLink size={18} />
                </a>
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
                {filteredServers.map((server, index) => (
                  <div
                    key={server.id}
                    className="animate-in fade-in slide-in-from-bottom-4 duration-500"
                    style={{ animationDelay: `${index * 50}ms` }}
                  >
                    <MCPServerCard
                      server={server}
                      onAddServer={handleAddServer}
                    />
                  </div>
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

      <FakerBuilderModal
        isOpen={fakerModalOpen}
        onClose={() => setFakerModalOpen(false)}
        environmentName={environments.find(e => e.id === selectedEnvironmentForFaker)?.name || 'default'}
        environmentId={selectedEnvironmentForFaker}
        onSuccess={handleFakerCreated}
      />
    </div>
  );
};