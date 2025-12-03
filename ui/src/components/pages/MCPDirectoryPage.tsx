import React, { useState, useEffect, useCallback } from 'react';
import { Plus, Server, Database, Terminal, Code, Search, Cloud, Layers, FileText, AlertCircle, Package, Shield, Wand2, X, Key, Container, FileCode, Network, Anchor, Github, HelpCircle } from 'lucide-react';
import { SyncModal } from '../sync/SyncModal';
import { FakerBuilderModal } from '../modals/FakerBuilderModal';
import { HelpModal } from '../ui/HelpModal';

interface TocItem {
  id: string;
  label: string;
}

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
  logoUrl?: string;                // Logo image URL for the service
  isInstalled?: boolean;
  requiresShip?: boolean;
  openapiSpec?: string;           // OpenAPI spec filename
  requiresOpenAPISpec?: boolean;  // Whether this template requires OpenAPI spec
  githubUrl?: string;             // MCP implementation code URL
  toolSourceUrl?: string;          // Original tool source code URL
  docsUrl?: string;                // Documentation URL for API keys/setup
  requiresApiKey?: boolean;        // Whether this server requires API keys
  version?: string;                // MCP server version (e.g., "1.0.0", "latest")
  availableVersions?: string[];    // Available versions to install
}

interface Environment {
  id: number;
  name: string;
}

const mcpServers: MCPServer[] = [
  // Secret Scanning
  {
    id: 'ship-gitleaks',
    name: 'GitLeaks',
    description: 'Fast secret scanning for git repositories (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Secret Scanning',
    command: 'ship',
    args: ['mcp', 'gitleaks'],
    icon: Key,
    logoUrl: '/logos/gitleaks.svg',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/gitleaks.go',
    toolSourceUrl: 'https://github.com/gitleaks/gitleaks',
    docsUrl: 'https://github.com/gitleaks/gitleaks#readme',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0', 'v0.7.3']
  },
  {
    id: 'ship-trufflehog',
    name: 'TruffleHog',
    description: 'Advanced secret scanning with verification (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Secret Scanning',
    command: 'ship',
    args: ['mcp', 'trufflehog'],
    icon: Key,
    logoUrl: '/logos/trufflehog.svg',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/trufflehog.go',
    toolSourceUrl: 'https://github.com/trufflesecurity/trufflehog',
    docsUrl: 'https://github.com/trufflesecurity/trufflehog#readme',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0', 'v0.7.3']
  },
  // Container Security
  {
    id: 'ship-trivy',
    name: 'Trivy',
    description: 'Comprehensive vulnerability scanner for containers and filesystems (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Container Security',
    command: 'ship',
    args: ['mcp', 'trivy'],
    icon: Container,
    logoUrl: '/logos/trivy.png',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/trivy.go',
    toolSourceUrl: 'https://github.com/aquasecurity/trivy',
    docsUrl: 'https://aquasecurity.github.io/trivy',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0', 'v0.7.3']
  },
  {
    id: 'ship-syft',
    name: 'Syft',
    description: 'SBOM (Software Bill of Materials) generation tool (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Supply Chain',
    command: 'ship',
    args: ['mcp', 'syft'],
    icon: Package,
    logoUrl: '/logos/syft.png',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/syft.go',
    toolSourceUrl: 'https://github.com/anchore/syft',
    docsUrl: 'https://github.com/anchore/syft#readme',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0', 'v0.7.3']
  },
  {
    id: 'ship-grype',
    name: 'Grype',
    description: 'Vulnerability scanner for container images and filesystems (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Container Security',
    command: 'ship',
    args: ['mcp', 'grype'],
    icon: Container,
    logoUrl: '/logos/grype.png',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship', // MCP implementation not found, linking to main repo
    toolSourceUrl: 'https://github.com/anchore/grype',
    docsUrl: 'https://github.com/anchore/grype#readme',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0']
  },
  {
    id: 'ship-dockle',
    name: 'Dockle',
    description: 'Container image linter for security best practices (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Container Security',
    command: 'ship',
    args: ['mcp', 'dockle'],
    icon: Container,
    logoUrl: '/logos/dockle.png',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/dockle.go',
    toolSourceUrl: 'https://github.com/goodwithtech/dockle',
    docsUrl: 'https://github.com/goodwithtech/dockle#readme',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0']
  },
  // Code Analysis
  {
    id: 'ship-semgrep',
    name: 'Semgrep',
    description: 'Static analysis security scanning for code (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Code Analysis',
    command: 'ship',
    args: ['mcp', 'semgrep'],
    icon: Code,
    logoUrl: '/logos/semgrep.svg',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/semgrep.go',
    toolSourceUrl: 'https://github.com/semgrep/semgrep',
    docsUrl: 'https://semgrep.dev/docs',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0']
  },
  // Infrastructure as Code
  {
    id: 'ship-checkov',
    name: 'Checkov',
    description: 'Infrastructure as Code security scanning (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Infrastructure as Code',
    command: 'ship',
    args: ['mcp', 'checkov'],
    icon: FileCode,
    logoUrl: '/logos/checkov.png',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/checkov.go',
    toolSourceUrl: 'https://github.com/bridgecrewio/checkov',
    docsUrl: 'https://www.checkov.io/1.Welcome/What%20is%20Checkov.html',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0']
  },
  {
    id: 'ship-terrascan',
    name: 'Terrascan',
    description: 'Infrastructure as Code security scanner (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Infrastructure as Code',
    command: 'ship',
    args: ['mcp', 'terrascan'],
    icon: FileCode,
    logoUrl: '/logos/terrascan.png',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/terrascan.go',
    toolSourceUrl: 'https://github.com/tenable/terrascan',
    docsUrl: 'https://runterrascan.io/docs/',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0']
  },
  {
    id: 'ship-tfsec',
    name: 'TFSec',
    description: 'Terraform-specific security scanner (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Infrastructure as Code',
    command: 'ship',
    args: ['mcp', 'tfsec'],
    icon: FileCode,
    logoUrl: '/logos/tfsec.png',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/tfsec.go',
    toolSourceUrl: 'https://github.com/aquasecurity/tfsec',
    docsUrl: 'https://aquasecurity.github.io/tfsec',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0']
  },
  {
    id: 'ship-tflint',
    name: 'TFLint',
    description: 'Terraform linter for syntax and best practices (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Infrastructure as Code',
    command: 'ship',
    args: ['mcp', 'tflint'],
    icon: FileCode,
    logoUrl: '/logos/tflint.png',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/tflint.go',
    toolSourceUrl: 'https://github.com/terraform-linters/tflint',
    docsUrl: 'https://github.com/terraform-linters/tflint#readme',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0']
  },
  {
    id: 'ship-terraform-docs',
    name: 'Terraform Docs',
    description: 'Terraform documentation generator (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Infrastructure as Code',
    command: 'ship',
    args: ['mcp', 'terraform-docs'],
    icon: FileText,
    logoUrl: '/logos/terraform-docs.png',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/terraform_docs.go',
    toolSourceUrl: 'https://github.com/terraform-docs/terraform-docs',
    docsUrl: 'https://terraform-docs.io/',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0']
  },
  // Network Security
  {
    id: 'ship-nuclei',
    name: 'Nuclei',
    description: 'Fast vulnerability scanner with community templates (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Network Security',
    command: 'ship',
    args: ['mcp', 'nuclei'],
    icon: Network,
    logoUrl: '/logos/nuclei.png',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/nuclei.go',
    toolSourceUrl: 'https://github.com/projectdiscovery/nuclei',
    docsUrl: 'https://docs.projectdiscovery.io/nuclei',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0']
  },
  // Kubernetes Security
  {
    id: 'ship-kubescape',
    name: 'Kubescape',
    description: 'Kubernetes security scanner and compliance checker (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Kubernetes Security',
    command: 'ship',
    args: ['mcp', 'kubescape'],
    icon: Anchor,
    logoUrl: '/logos/kubescape.svg',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/kubescape.go',
    toolSourceUrl: 'https://github.com/kubescape/kubescape',
    docsUrl: 'https://hub.armosec.io/docs/',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0']
  },
  {
    id: 'ship-kube-bench',
    name: 'Kube-bench',
    description: 'Kubernetes CIS benchmark security scanner (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Kubernetes Security',
    command: 'ship',
    args: ['mcp', 'kube-bench'],
    icon: Anchor,
    logoUrl: '/logos/kube-bench.png',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/kube_bench.go',
    toolSourceUrl: 'https://github.com/aquasecurity/kube-bench',
    docsUrl: 'https://github.com/aquasecurity/kube-bench#readme',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0']
  },
  {
    id: 'ship-kube-hunter',
    name: 'Kube-hunter',
    description: 'Kubernetes penetration testing tool (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Kubernetes Security',
    command: 'ship',
    args: ['mcp', 'kube-hunter'],
    icon: Anchor,
    logoUrl: '/logos/kube-hunter.png',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/kube_hunter.go',
    toolSourceUrl: 'https://github.com/aquasecurity/kube-hunter',
    docsUrl: 'https://github.com/aquasecurity/kube-hunter#readme',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0']
  },
  // Supply Chain Security
  {
    id: 'ship-cosign',
    name: 'Cosign',
    description: 'Container signing and verification for supply chain security (Note: Requires Docker-in-Docker for ECS deployments)',
    category: 'Supply Chain',
    command: 'ship',
    args: ['mcp', 'cosign'],
    icon: Package,
    logoUrl: '/logos/cosign.png',
    requiresShip: true,
    githubUrl: 'https://github.com/cloudshipai/ship/blob/main/internal/cli/mcp/cosign.go',
    toolSourceUrl: 'https://github.com/sigstore/cosign',
    docsUrl: 'https://docs.sigstore.dev/cosign/overview/',
    version: 'latest',
    availableVersions: ['latest', 'v0.9.0', 'v0.8.2', 'v0.8.1', 'v0.8.0']
  }
];

interface MCPServerCardProps {
  server: MCPServer;
  onAddServer: (server: MCPServer) => void;
  disabled?: boolean;
  shipVersion?: string;
}

const MCPServerCard: React.FC<MCPServerCardProps> = ({ server, onAddServer, disabled, shipVersion }) => {
  const Icon = server.icon;
  const [logoError, setLogoError] = useState(false);

  // Reset logo error when logoUrl changes
  useEffect(() => {
    setLogoError(false);
  }, [server.logoUrl]);

  return (
    <div className={`bg-white rounded-xl border border-gray-200/60 shadow-sm hover:shadow-md hover:-translate-y-1 transition-all duration-300 p-5 overflow-hidden ${disabled ? 'opacity-50' : ''}`}>
      {/* Header with title and action buttons */}
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center space-x-3 min-w-0 flex-1">
          <div className="p-2 bg-purple-50 rounded-lg flex-shrink-0">
            {server.logoUrl && !logoError ? (
              <img 
                src={server.logoUrl}
                alt={`${server.name} logo`} 
                className="w-6 h-6 object-contain"
                onError={(e) => {
                  console.error('Logo failed to load:', server.logoUrl, e);
                  setLogoError(true);
                }}
                onLoad={() => {
                  console.log('Logo loaded successfully:', server.logoUrl);
                }}
              />
            ) : (
              <Icon size={20} className="text-purple-600" />
            )}
          </div>
          <div className="min-w-0">
            <div className="flex items-center space-x-2">
              <h3 className="text-base font-semibold text-gray-900 truncate">{server.name}</h3>
              {server.requiresOpenAPISpec && (
                <span className="px-2 py-0.5 text-[10px] bg-blue-50 text-blue-700 rounded-md border border-blue-200/60 font-medium flex-shrink-0" title={`OpenAPI Spec: ${server.openapiSpec}`}>
                  OpenAPI
                </span>
              )}
            </div>
            <span className="text-xs text-gray-600">{server.category}</span>
          </div>
        </div>

        {/* Action buttons in top right */}
        <div className="flex items-start gap-2 flex-shrink-0 ml-2">
          {server.isInstalled && (
            <span className="px-2.5 py-1 text-xs bg-emerald-50 text-emerald-700 rounded-full font-medium border border-emerald-200/60">
              Installed
            </span>
          )}
          {server.githubUrl && (
            <a
              href={server.githubUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="p-2.5 text-gray-900 bg-gray-100 hover:bg-gray-200 border-2 border-gray-400 rounded-lg transition-all hover:shadow-md"
              title="View MCP Implementation"
            >
              <Github size={20} strokeWidth={2} />
            </a>
          )}
          {server.toolSourceUrl && (
            <a
              href={server.toolSourceUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="p-2.5 text-gray-900 bg-gray-100 hover:bg-gray-200 border-2 border-gray-400 rounded-lg transition-all hover:shadow-md"
              title={`View ${server.name} Source Code`}
            >
              <Code size={20} strokeWidth={2} />
            </a>
          )}
          {server.docsUrl && (
            <a
              href={server.docsUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="p-2.5 text-blue-700 bg-blue-100 hover:bg-blue-200 border-2 border-blue-400 rounded-lg transition-all hover:shadow-md"
              title="View Documentation"
            >
              <HelpCircle size={20} strokeWidth={2} />
            </a>
          )}
          {server.requiresApiKey && (
            <a
              href={server.docsUrl || '#'}
              target="_blank"
              rel="noopener noreferrer"
              className="p-2.5 text-amber-800 bg-amber-100 hover:bg-amber-200 border-2 border-amber-500 rounded-lg transition-all hover:shadow-md"
              title="API Key Required"
            >
              <Key size={20} strokeWidth={2} />
            </a>
          )}
        </div>
      </div>

      <p className="text-sm text-gray-600 mb-4 leading-relaxed line-clamp-2">{server.description}</p>

      <div className="space-y-2.5 mb-4">
        {/* Ship MCP Version Info */}
        {server.requiresShip && (
          <div className="text-sm">
            <div className="px-3 py-2 bg-purple-50/50 rounded-lg border border-purple-200/60">
              <div className="flex items-center justify-between mb-1">
                <span className="font-medium text-purple-900 text-xs">Ship CLI MCP</span>
                {shipVersion ? (
                  <span className="text-xs font-mono text-purple-700">
                    Your version: {shipVersion.match(/Version:\s*([\d.]+)/)?.[1] || shipVersion.trim()}
                  </span>
                ) : (
                  <span className="text-xs text-purple-600">
                    Not installed
                  </span>
                )}
              </div>
              <p className="text-xs text-purple-700 leading-relaxed">
                {shipVersion ? (
                  'Uses your installed Ship CLI version.'
                ) : (
                  <button
                    onClick={() => window.open('https://github.com/cloudshipai/ship#installation', '_blank')}
                    className="text-purple-900 hover:text-purple-700 underline font-medium"
                  >
                    Install Ship CLI to access this MCP →
                  </button>
                )}
              </p>
            </div>
          </div>
        )}

        <div className="text-sm">
          <span className="font-medium text-gray-700 text-xs">Command:</span>
          <code className="mt-1 block px-3 py-2 bg-gray-50/50 text-gray-800 rounded-lg text-xs font-mono border border-gray-200/60 overflow-x-auto">
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
                    <AlertCircle size={14} className="text-amber-600" />
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
  shipVersion?: string;
}

const AddServerModal: React.FC<AddServerModalProps> = ({ server, environments, onClose, onConfirm, shipVersion }) => {
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
    <div className="fixed inset-0 bg-black/30 flex items-center justify-center z-50 backdrop-blur-sm">
      <div className="bg-white rounded-xl p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto border border-gray-200/60 shadow-lg">
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

          {server.requiresShip && (
            <div className="bg-purple-50 border border-purple-200/60 rounded-lg p-4">
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-medium text-purple-900">Ship CLI Required</span>
                {shipVersion ? (
                  <span className="text-xs font-mono text-purple-700">
                    Using: v{shipVersion.match(/Version:\s*([\d.]+)/)?.[1] || shipVersion.trim()}
                  </span>
                ) : (
                  <span className="text-xs text-red-600 font-medium">Not installed</span>
                )}
              </div>
              <p className="text-sm text-purple-700 leading-relaxed">
                {shipVersion ? (
                  'This MCP will use your installed Ship CLI version.'
                ) : (
                  <span className="text-red-700">
                    Ship CLI must be installed to use this MCP. <a href="https://github.com/cloudshipai/ship#installation" target="_blank" rel="noopener noreferrer" className="underline font-medium">Install Ship →</a>
                  </span>
                )}
              </p>
            </div>
          )}

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
  const [loading, setLoading] = useState(false);
  const [shipVersion, setShipVersion] = useState<string>('');
  const [syncModalOpen, setSyncModalOpen] = useState(false);
  const [syncEnvironment, setSyncEnvironment] = useState<string>('');
  const [fakerModalOpen, setFakerModalOpen] = useState(false);
  const [selectedEnvironmentForFaker, setSelectedEnvironmentForFaker] = useState<number>(0);
  const [isHelpModalOpen, setIsHelpModalOpen] = useState(false);

  // Define TOC items for help modal
  const helpTocItems: TocItem[] = [
    { id: 'what-is-mcp', label: 'What is MCP' },
    { id: 'how-mcp-works', label: 'How It Works' },
    { id: 'server-categories', label: 'Categories' },
    { id: 'installation', label: 'Installation' },
    { id: 'official-vs-community', label: 'Official vs Community' },
    { id: 'security', label: 'Security' },
    { id: 'use-cases', label: 'Use Cases' },
    { id: 'faker', label: 'AI Faker' }
  ];

  // Build categories from servers
  const serverCategories = Array.from(new Set(servers.map(s => s.category)))
    .sort();
  const categories = ['All', ...serverCategories];

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

  const fetchInstalledServers = useCallback(async () => {
    try {
      // Fetch installed servers for all environments
      const installedServerNames = new Set<string>();

      for (const env of environments) {
        try {
          const response = await fetch(`/api/v1/environments/${env.id}/mcp-servers`);
          if (response.ok) {
            const data = await response.json();
            // data.servers is a map of server names to configs
            const serverNames = Object.keys(data.servers || {});
            serverNames.forEach(name => installedServerNames.add(name));
          }
        } catch (envError) {
          console.error(`Failed to fetch servers for environment ${env.name}:`, envError);
        }
      }

      // Update servers list with installation status
      setServers(prev => prev.map(server => ({
        ...server,
        isInstalled: installedServerNames.has(server.id)
      })));

      setLoading(false);
    } catch (error) {
      console.error('Failed to fetch installed servers:', error);
      setLoading(false);
    }
  }, [environments]);

  const checkShipInstallation = async () => {
    try {
      const response = await fetch('/api/v1/ship/installed');
      const data = await response.json();
      setShipVersion(data.version || '');
    } catch (error) {
      console.error('Failed to check Ship installation:', error);
      setShipVersion('');
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
        logoUrl: template.logoUrl,
        githubUrl: template.githubUrl,
        toolSourceUrl: template.toolSourceUrl,
        docsUrl: template.docsUrl,
        requiresApiKey: template.requiresApiKey,
      }));

      // Merge with existing hardcoded servers, avoiding duplicates
      setServers(prev => {
        const existingIds = new Set(prev.map(s => s.id));
        const newTemplates = mappedTemplates.filter(t => !existingIds.has(t.id));
        return [...prev, ...newTemplates];
      });
    } catch (error) {
      console.error('Failed to fetch directory templates:', error);
    }
  };

  useEffect(() => {
    fetchEnvironments();
    checkShipInstallation();
    fetchDirectoryTemplates();
  }, []);

  // Fetch installed servers whenever environments change
  useEffect(() => {
    if (environments.length > 0) {
      fetchInstalledServers();
    }
  }, [environments, fetchInstalledServers]);

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
    // Refresh environments and installed servers status
    fetchEnvironments();
    fetchInstalledServers();
  };

  const handleFakerCreated = (_fakerName: string, envName: string) => {
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
    // Show all servers - Ship MCPs will display "Not installed" badge if Ship isn't available
    return matchesSearch && matchesCategory;
  });

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64 bg-[#fafaf8]">
        <div className="text-gray-600 text-sm font-medium animate-pulse">Loading MCP Directory...</div>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col bg-[#fafaf8] overflow-hidden">
      {/* Header - Paper matte style */}
      <div className="flex-shrink-0 px-6 py-5 border-b border-gray-200/60 bg-white/60 backdrop-blur-sm overflow-hidden">
        <div className="mb-6">
          <h1 className="text-2xl font-medium text-gray-900 mb-2">MCP Directory</h1>
          <p className="text-sm text-gray-600">Discover and install MCP servers to extend your Station capabilities</p>
        </div>

        <div className="mb-6 flex items-center gap-3">
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
          <button
            onClick={() => setIsHelpModalOpen(true)}
            className="flex items-center gap-2 px-4 py-2.5 text-gray-600 bg-white hover:bg-gray-50 border border-gray-300 rounded-lg text-sm font-medium transition-all duration-200 shadow-sm whitespace-nowrap"
          >
            <HelpCircle className="h-4 w-4" />
            Help
          </button>
        </div>

        {/* Category Tabs - Wrapping pill style */}
        <div className="flex flex-wrap gap-2 pb-1">
          {categories.map(category => (
            <button
              key={category}
              onClick={() => setSelectedCategory(category)}
              className={`px-3.5 py-2 rounded-lg text-sm font-medium transition-all duration-200 whitespace-nowrap ${
                selectedCategory === category
                  ? 'bg-gray-900 text-white shadow-sm'
                  : 'bg-white text-gray-700 hover:bg-gray-100 border border-gray-200/60 shadow-sm'
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
          <div className="text-center py-16">
            <Server size={48} className="mx-auto text-gray-300 mb-4" />
            <h3 className="text-lg font-medium text-gray-900 mb-2">No servers found</h3>
            <p className="text-sm text-gray-600">Try adjusting your search or category filter</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
            {filteredServers.map((server) => (
              <div key={server.id}>
                <MCPServerCard
                  server={server}
                  onAddServer={handleAddServer}
                  shipVersion={shipVersion}
                />
              </div>
            ))}
          </div>
        )}
      </div>

      <AddServerModal
        server={selectedServer}
        environments={environments}
        onClose={() => setSelectedServer(null)}
        onConfirm={handleConfirmAddServer}
        shipVersion={shipVersion}
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

      {/* Help Modal */}
      <HelpModal
        isOpen={isHelpModalOpen}
        onClose={() => setIsHelpModalOpen(false)}
        title="MCP Directory"
        pageDescription="Browse and install Model Context Protocol (MCP) servers that extend your agents with new capabilities. Each MCP server provides a collection of tools - from filesystem operations to cloud APIs, databases, and security scanners."
        tocItems={helpTocItems}
      >
        <div className="space-y-6">
          {/* What is MCP */}
          <div id="what-is-mcp">
            <h3 className="text-base font-semibold text-gray-900 mb-3">What is MCP?</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-6">
              <div className="text-sm text-gray-900 mb-4">
                <strong>Model Context Protocol (MCP)</strong> is an open standard created by Anthropic for connecting AI agents to external systems.
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-3">
                  <div className="bg-white border border-gray-200 rounded-lg p-3">
                    <div className="font-mono text-sm text-gray-900 mb-1">Security First</div>
                    <div className="text-xs text-gray-600">Fine-grained permissions - grant only specific tools to each agent, not full API access</div>
                  </div>
                  <div className="bg-white border border-gray-200 rounded-lg p-3">
                    <div className="font-mono text-sm text-gray-900 mb-1">Ecosystem</div>
                    <div className="text-xs text-gray-600">100+ community MCP servers available for AWS, databases, security tools, and more</div>
                  </div>
                </div>
                <div className="space-y-3">
                  <div className="bg-white border border-gray-200 rounded-lg p-3">
                    <div className="font-mono text-sm text-gray-900 mb-1">Standardized</div>
                    <div className="text-xs text-gray-600">Same MCP server works across Claude Desktop, Cursor, and Station without changes</div>
                  </div>
                  <div className="bg-white border border-gray-200 rounded-lg p-3">
                    <div className="font-mono text-sm text-gray-900 mb-1">Credential Safety</div>
                    <div className="text-xs text-gray-600">MCP servers handle authentication - agents never see your API keys or credentials</div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* MCP Architecture */}
          <div id="how-mcp-works">
            <h3 className="text-base font-semibold text-gray-900 mb-3">How MCP Works</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-6">
              <div className="flex items-center justify-between gap-8 mb-4">
                <div className="flex-1 bg-white border border-gray-200 rounded-lg p-4 text-center">
                  <div className="text-lg font-bold text-[#0084FF] mb-1">Agent</div>
                  <div className="text-xs text-gray-600">Your AI agent with instructions</div>
                </div>
                <div className="text-gray-400 font-bold">→</div>
                <div className="flex-1 bg-white border border-gray-200 rounded-lg p-4 text-center">
                  <div className="text-lg font-bold text-[#0084FF] mb-1">MCP Server</div>
                  <div className="text-xs text-gray-600">Filesystem, AWS, Database</div>
                </div>
                <div className="text-gray-400 font-bold">→</div>
                <div className="flex-1 bg-white border border-gray-200 rounded-lg p-4 text-center">
                  <div className="text-lg font-bold text-[#0084FF] mb-1">Tools</div>
                  <div className="text-xs text-gray-600">read_file, get_cost, sql_query</div>
                </div>
              </div>
              <div className="text-xs text-gray-600 bg-white border border-gray-300 px-3 py-2 rounded font-mono">
                Example: Agent calls read_file → MCP Server executes → Returns file contents
              </div>
            </div>
          </div>

          {/* Server Categories */}
          <div id="server-categories">
            <h3 className="text-base font-semibold text-gray-900 mb-3">MCP Server Categories</h3>
            <div className="grid grid-cols-2 gap-3">
              <div className="bg-white rounded border border-gray-200 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <Terminal className="h-4 w-4 text-gray-600" />
                  <div className="font-mono text-sm text-gray-900">Filesystem</div>
                </div>
                <div className="text-xs text-gray-600">Read/write files, list directories, search for patterns. Essential for code analysis and log parsing.</div>
              </div>

              <div className="bg-white rounded border border-gray-200 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <Cloud className="h-4 w-4 text-gray-600" />
                  <div className="font-mono text-sm text-gray-900">Cloud Providers</div>
                </div>
                <div className="text-xs text-gray-600">AWS, GCP, Azure APIs. Query costs, list resources, check configurations across cloud infrastructure.</div>
              </div>

              <div className="bg-white rounded border border-gray-200 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <Database className="h-4 w-4 text-gray-600" />
                  <div className="font-mono text-sm text-gray-900">Databases</div>
                </div>
                <div className="text-xs text-gray-600">PostgreSQL, MySQL queries. Read-only access for analysis agents, no INSERT/UPDATE/DELETE.</div>
              </div>

              <div className="bg-white rounded border border-gray-200 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <Shield className="h-4 w-4 text-gray-600" />
                  <div className="font-mono text-sm text-gray-900">Security Tools</div>
                </div>
                <div className="text-xs text-gray-600">Trivy, Semgrep, GitLeaks, Checkov. Vulnerability scanning for containers, code, and infrastructure.</div>
              </div>

              <div className="bg-white rounded border border-gray-200 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <Code className="h-4 w-4 text-gray-600" />
                  <div className="font-mono text-sm text-gray-900">Development Tools</div>
                </div>
                <div className="text-xs text-gray-600">GitHub, GitLab APIs. Create issues, list PRs, search code across repositories.</div>
              </div>

              <div className="bg-white rounded border border-gray-200 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <Layers className="h-4 w-4 text-gray-600" />
                  <div className="font-mono text-sm text-gray-900">Monitoring</div>
                </div>
                <div className="text-xs text-gray-600">Grafana, Datadog, CloudWatch. Query metrics, check alerts, analyze system performance.</div>
              </div>
            </div>
          </div>

          {/* Installation Process */}
          <div id="installation">
            <h3 className="text-base font-semibold text-gray-900 mb-3">Installing MCP Servers</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-4">
              <div className="space-y-3">
                <div className="flex items-start gap-3">
                  <div className="w-6 h-6 rounded bg-[#0084FF] flex items-center justify-center font-mono text-xs text-white flex-shrink-0">1</div>
                  <div>
                    <div className="font-mono text-sm text-gray-900 mb-1">Browse the Directory</div>
                    <div className="text-xs text-gray-600">Search or filter by category. Each server shows description, required credentials, and available tools.</div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="w-6 h-6 rounded bg-[#0084FF] flex items-center justify-center font-mono text-xs text-white flex-shrink-0">2</div>
                  <div>
                    <div className="font-mono text-sm text-gray-900 mb-1">Select Environment</div>
                    <div className="text-xs text-gray-600">Choose which environment (dev/staging/prod) gets this server. Environment isolation keeps development separate from production.</div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="w-6 h-6 rounded bg-[#0084FF] flex items-center justify-center font-mono text-xs text-white flex-shrink-0">3</div>
                  <div>
                    <div className="font-mono text-sm text-gray-900 mb-1">Configure Variables</div>
                    <div className="text-xs text-gray-600">Set API keys, regions, paths via template variables. Values stored in variables.yml (gitignored for security).</div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="w-6 h-6 rounded bg-[#0084FF] flex items-center justify-center font-mono text-xs text-white flex-shrink-0">4</div>
                  <div>
                    <div className="font-mono text-sm text-gray-900 mb-1">Sync Environment</div>
                    <div className="text-xs text-gray-600">Station starts the MCP server, discovers available tools, and makes them accessible to agents in that environment.</div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Official vs Community */}
          <div id="official-vs-community">
            <h3 className="text-base font-semibold text-gray-900 mb-3">Official vs Community Servers</h3>
            <div className="grid grid-cols-2 gap-3">
              <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
                <div className="font-mono text-sm text-blue-900 mb-2">Official MCP Servers</div>
                <div className="text-xs text-gray-700 space-y-2">
                  <div>• Built and maintained by Anthropic</div>
                  <div>• Filesystem, PostgreSQL, GitHub, Fetch</div>
                  <div>• Install via: <code className="bg-white px-1 rounded">npx -y @modelcontextprotocol/server-*</code></div>
                  <div>• Well-documented and stable</div>
                </div>
              </div>

              <div className="bg-purple-50 border border-purple-200 rounded-lg p-4">
                <div className="font-mono text-sm text-purple-900 mb-2">Community Servers</div>
                <div className="text-xs text-gray-700 space-y-2">
                  <div>• Built by open source community</div>
                  <div>• AWS, security tools, monitoring, databases</div>
                  <div>• Install via npm, docker, or ship CLI</div>
                  <div>• Check GitHub for docs and support</div>
                </div>
              </div>
            </div>
          </div>

          {/* Security Best Practices */}
          <div id="security">
            <h3 className="text-base font-semibold text-gray-900 mb-3">Security Best Practices</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-4">
              <ul className="space-y-2 text-xs text-gray-700">
                <li className="flex items-start gap-2">
                  <span className="text-green-600 font-bold">✓</span>
                  <div><strong>Principle of Least Privilege</strong> - Only grant agents the specific tools they need. Read-only tools for analysis agents, write tools only when necessary.</div>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-green-600 font-bold">✓</span>
                  <div><strong>Environment Isolation</strong> - Use separate environments for dev/staging/prod. Dev agents can't access production MCP servers.</div>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-green-600 font-bold">✓</span>
                  <div><strong>Never Hardcode Secrets</strong> - Use template variables for API keys and credentials. Values go in variables.yml which is gitignored.</div>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-green-600 font-bold">✓</span>
                  <div><strong>Audit Logging</strong> - All tool calls are tracked in run metadata. Review agent runs to see which tools were called with what parameters.</div>
                </li>
              </ul>
            </div>
          </div>

          {/* Common Use Cases */}
          <div id="use-cases">
            <h3 className="text-base font-semibold text-gray-900 mb-3">Common Use Cases</h3>
            <div className="space-y-2">
              <div className="bg-white border border-gray-200 rounded p-3">
                <div className="font-mono text-sm text-gray-900 mb-1">FinOps Cost Analysis</div>
                <div className="text-xs text-gray-600">Install AWS MCP server with cost explorer tools. Agent analyzes spend, identifies savings opportunities, forecasts budgets.</div>
              </div>
              <div className="bg-white border border-gray-200 rounded p-3">
                <div className="font-mono text-sm text-gray-900 mb-1">Security Scanning</div>
                <div className="text-xs text-gray-600">Install Ship security tools (Trivy, Semgrep, GitLeaks, Checkov). Agent scans infrastructure, containers, and code for vulnerabilities.</div>
              </div>
              <div className="bg-white border border-gray-200 rounded p-3">
                <div className="font-mono text-sm text-gray-900 mb-1">Code Analysis</div>
                <div className="text-xs text-gray-600">Install Filesystem MCP server. Agent reads source code, analyzes patterns, suggests refactoring, documents codebases.</div>
              </div>
              <div className="bg-white border border-gray-200 rounded p-3">
                <div className="font-mono text-sm text-gray-900 mb-1">Database Analytics</div>
                <div className="text-xs text-gray-600">Install PostgreSQL MCP server with read-only access. Agent queries data, generates reports, identifies trends without modifying data.</div>
              </div>
            </div>
          </div>

          {/* AI Faker Section */}
          <div id="faker">
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <Wand2 className="h-5 w-5 text-purple-600" />
              AI Faker: Mock Tools for Rapid Prototyping
            </h3>
            <div className="bg-purple-50 border border-purple-200 rounded-lg p-4">
              <div className="text-sm text-gray-900 mb-3">
                <strong>Faker</strong> generates AI-powered mock MCP tools when you don't have real API credentials or want to prototype quickly.
              </div>
              <div className="space-y-3">
                <div className="bg-white border border-purple-200 rounded p-3">
                  <div className="font-mono text-sm text-gray-900 mb-1">What is Faker?</div>
                  <div className="text-xs text-gray-600">
                    An AI-powered mock tool generator that creates realistic MCP server responses without requiring real API keys or making external calls. Perfect for development, testing, and demos.
                  </div>
                </div>
                <div className="bg-white border border-purple-200 rounded p-3">
                  <div className="font-mono text-sm text-gray-900 mb-1">When to Use Faker</div>
                  <div className="text-xs text-gray-600 space-y-1">
                    <div>• <strong>No Credentials:</strong> Develop AWS/GCP agents without cloud access</div>
                    <div>• <strong>Rapid Prototyping:</strong> Test agent workflows instantly before production integration</div>
                    <div>• <strong>Cost-Free Testing:</strong> No API usage charges during development</div>
                    <div>• <strong>Safe Demos:</strong> Show stakeholders agent capabilities without exposing real credentials</div>
                  </div>
                </div>
                <div className="bg-white border border-purple-200 rounded p-3">
                  <div className="font-mono text-sm text-gray-900 mb-1">How It Works</div>
                  <div className="text-xs text-gray-600">
                    Click "Create Faker" → Choose a template (AWS FinOps, GCP, Stripe, etc.) or write custom instructions → Faker generates 15-25 mock tools → Use with agents for testing → Swap with real MCP server for production
                  </div>
                </div>
                <div className="bg-amber-50 border border-amber-300 rounded p-3">
                  <div className="flex items-start gap-2">
                    <AlertCircle className="h-4 w-4 text-amber-600 flex-shrink-0 mt-0.5" />
                    <div className="text-xs text-amber-800">
                      <strong>Important:</strong> Faker responses are AI-generated simulations, not real data. Always replace with actual MCP servers for production workflows.
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </HelpModal>
    </div>
  );
};