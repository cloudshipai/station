import React, { useState } from 'react';
import { X, Plus, Copy } from 'lucide-react';
import { apiClient } from '../../api/client';

interface AddServerModalProps {
  isOpen: boolean;
  onClose: () => void;
  environmentName: string;
  onSuccess?: () => void;
}

export const AddServerModal: React.FC<AddServerModalProps> = ({
  isOpen,
  onClose,
  environmentName,
  onSuccess
}) => {
  const [activeTab, setActiveTab] = useState<'mcp' | 'openapi'>('mcp');
  const [serverName, setServerName] = useState('');
  const [serverConfig, setServerConfig] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [response, setResponse] = useState<any>(null);
  const [showSuccess, setShowSuccess] = useState(false);

  // Generate default MCP config dynamically based on server name
  const getDefaultConfig = (name: string) => `{
  "mcpServers": {
    "${name || 'server'}": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-${name || 'example'}@latest"
      ]
    }
  }
}`;

  // Generate default OpenAPI spec template
  const getDefaultOpenAPISpec = (name: string) => `{
  "openapi": "3.0.0",
  "info": {
    "title": "${name || 'My API'}",
    "version": "1.0.0",
    "description": "API integration for ${name || 'My Service'}"
  },
  "servers": [
    {
      "url": "https://api.example.com",
      "description": "Production API"
    }
  ],
  "components": {
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "description": "Bearer token authentication"
      }
    }
  },
  "paths": {
    "/example": {
      "get": {
        "operationId": "getExample",
        "summary": "Get example data",
        "security": [{"bearerAuth": []}],
        "responses": {
          "200": {
            "description": "Success"
          }
        }
      }
    }
  }
}`;

  // Auto-populate config ONLY when server name or tab changes and config is empty
  // This prevents overwriting user-pasted configs
  React.useEffect(() => {
    if (serverName && !serverConfig.trim()) {
      // Only populate if config is empty
      const newConfig = activeTab === 'mcp'
        ? getDefaultConfig(serverName)
        : getDefaultOpenAPISpec(serverName);
      setServerConfig(newConfig);
      console.log(`[AddServerModal] Auto-populated ${activeTab} config for: ${serverName}`);
    } else if (!serverName) {
      // Clear config if server name is empty
      setServerConfig('');
    }
  }, [serverName, activeTab]); // Depend on both serverName and activeTab

  const handleSubmit = async () => {
    if (!serverName.trim() || !serverConfig.trim()) {
      setResponse({ error: 'Server name and config are required' });
      return;
    }

    setIsLoading(true);
    setResponse(null);

    try {
      // Route to correct endpoint based on active tab
      const endpoint = activeTab === 'mcp' ? '/mcp-servers' : '/openapi/specs';
      const payload = activeTab === 'mcp'
        ? { name: serverName, config: serverConfig, environment: environmentName }
        : { name: serverName, spec: serverConfig, environment: environmentName };

      const result = await apiClient.post(endpoint, payload);
      setResponse(result.data);

      // Check if variables are needed
      if (result.data.error === 'VARIABLES_NEEDED') {
        console.log('[AddServerModal] Variables needed, closing modal and triggering sync');
        // Close this modal and trigger sync (which will open sync modal)
        handleClose();
        if (onSuccess) {
          onSuccess(); // This triggers the sync modal in parent
        }
        return;
      }

      setShowSuccess(true);

      // Trigger success callback (for auto-sync)
      if (onSuccess) {
        onSuccess();
      }
    } catch (error) {
      console.error('Failed to create MCP server:', error);
      // Extract error message from API response if available
      const errorMessage = error.response?.data?.error || error.message || 'Failed to create MCP server';
      setResponse({ error: errorMessage });
    } finally {
      setIsLoading(false);
    }
  };

  const resetModal = () => {
    setServerName('');
    setServerConfig('');
    setResponse(null);
    setShowSuccess(false);
    setIsLoading(false);
  };

  const handleClose = () => {
    resetModal();
    onClose();
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-[9999]">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo-glow max-w-4xl w-full mx-4 z-[10000] relative max-h-[90vh] overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark rounded-t-lg">
          <h2 className="text-lg font-mono font-semibold text-white z-10 relative">
            Add MCP Server: {environmentName}
          </h2>
          <button onClick={handleClose} className="text-tokyo-comment hover:text-tokyo-fg transition-colors z-10 relative">
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Tabs */}
        <div className="flex border-b border-tokyo-blue7 bg-tokyo-bg">
          <button
            onClick={() => setActiveTab('mcp')}
            className={`flex-1 px-4 py-3 font-mono text-sm font-medium transition-colors ${
              activeTab === 'mcp'
                ? 'text-tokyo-cyan border-b-2 border-tokyo-cyan bg-tokyo-bg-dark'
                : 'text-tokyo-comment hover:text-tokyo-fg'
            }`}
          >
            MCP Config
          </button>
          <button
            onClick={() => setActiveTab('openapi')}
            className={`flex-1 px-4 py-3 font-mono text-sm font-medium transition-colors ${
              activeTab === 'openapi'
                ? 'text-tokyo-cyan border-b-2 border-tokyo-cyan bg-tokyo-bg-dark'
                : 'text-tokyo-comment hover:text-tokyo-fg'
            }`}
          >
            OpenAPI Spec
          </button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-6 overflow-y-auto flex-1">
          {!showSuccess ? (
            <>
              {/* Server Name Input */}
              <div className="space-y-2">
                <label className="text-sm font-mono text-tokyo-cyan font-medium">
                  {activeTab === 'mcp' ? 'Server Name:' : 'Spec Name:'}
                </label>
                <input
                  type="text"
                  value={serverName}
                  onChange={(e) => setServerName(e.target.value)}
                  className="w-full px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 rounded font-mono text-tokyo-fg focus:outline-none focus:border-tokyo-cyan"
                  placeholder="e.g., filesystem, database, etc."
                />
              </div>

              {/* Server Config Input */}
              <div className="space-y-2">
                <label className="text-sm font-mono text-tokyo-cyan font-medium">
                  {activeTab === 'mcp' ? 'Server Configuration:' : 'OpenAPI Specification (JSON):'}
                </label>
                <textarea
                  value={serverConfig}
                  onChange={(e) => setServerConfig(e.target.value)}
                  className="w-full h-80 px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 rounded font-mono text-tokyo-fg focus:outline-none focus:border-tokyo-cyan text-xs"
                  placeholder={activeTab === 'mcp' ? getDefaultConfig(serverName) : getDefaultOpenAPISpec(serverName)}
                />
              </div>

              {/* Documentation Note */}
              <div className="bg-blue-900 bg-opacity-30 border border-blue-500 border-opacity-50 rounded p-4">
                <p className="text-sm text-blue-300 font-mono">
                  {activeTab === 'mcp' ? (
                    <>
                      <strong>Note:</strong> Replace any arguments you want as variables with <code className="bg-gray-800 px-1 rounded">{'{{ .VAR }}'}</code> Go variable notation.{' '}
                      <a
                        href="https://cloudshipai.github.io/station/en/mcp/overview/"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-blue-400 underline hover:text-blue-300"
                      >
                        More info here
                      </a>
                    </>
                  ) : (
                    <>
                      <strong>Note:</strong> Use <code className="bg-gray-800 px-1 rounded">{'{{ .VAR }}'}</code> for template variables in your spec.
                      {' '}For authentication, add security schemes in the <code className="bg-gray-800 px-1 rounded">components.securitySchemes</code> section.
                      {' '}
                      <a
                        href="https://cloudshipai.github.io/station/en/mcp/openapi/"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-blue-400 underline hover:text-blue-300"
                      >
                        More info here
                      </a>
                    </>
                  )}
                </p>
              </div>

              {/* Error Display */}
              {response?.error && (
                <div className="bg-red-900 bg-opacity-30 border border-red-500 border-opacity-50 rounded p-4">
                  <h4 className="text-sm font-mono text-red-400 font-medium mb-2">Error</h4>
                  <div className="text-xs text-red-400 font-mono">
                    {response.error}
                  </div>
                </div>
              )}
            </>
          ) : (
            /* Success Card */
            <div className="space-y-4">
              <div className="bg-green-900 bg-opacity-30 border border-green-500 border-opacity-50 rounded p-6 text-center">
                <h3 className="text-lg font-mono text-white font-medium mb-4">MCP Server Created Successfully!</h3>

                <div className="space-y-3 text-left">
                  <div>
                    <span className="text-xs text-green-400 font-mono font-medium">Server Name:</span>
                    <div className="mt-1 p-2 bg-gray-900 border border-gray-600 rounded font-mono text-xs text-gray-200">
                      {serverName}
                    </div>
                  </div>

                  <div>
                    <span className="text-xs text-green-400 font-mono font-medium">Environment:</span>
                    <div className="mt-1 p-2 bg-gray-900 border border-gray-600 rounded font-mono text-xs text-gray-200">
                      {environmentName}
                    </div>
                  </div>
                </div>
              </div>

              {/* Next Steps */}
              <div className="bg-blue-900 bg-opacity-30 border border-blue-500 border-opacity-50 rounded p-4">
                <h4 className="text-sm font-mono text-blue-400 font-medium mb-3">Next Steps</h4>
                <p className="text-xs text-blue-300 font-mono mb-3">
                  Sync this config and input your variables:
                </p>

                <div className="bg-gray-900 border border-gray-600 rounded p-3 flex items-center justify-between">
                  <code className="text-xs text-gray-200 font-mono">stn sync</code>
                  <button
                    onClick={() => navigator.clipboard.writeText('stn sync')}
                    className="p-1 text-blue-400 hover:text-blue-300 transition-colors"
                    title="Copy command"
                  >
                    <Copy className="h-4 w-4" />
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        {!showSuccess && (
          <div className="p-4 border-t border-tokyo-blue7">
            <button
              onClick={handleSubmit}
              disabled={isLoading || !serverName.trim() || !serverConfig.trim()}
              className="w-full px-4 py-2 bg-tokyo-cyan text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-blue1 transition-colors shadow-tokyo-glow disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            >
              {isLoading ? (
                <>
                  <div className="animate-spin rounded-full h-4 w-4 border-2 border-tokyo-bg border-t-transparent"></div>
                  {activeTab === 'mcp' ? 'Creating Server...' : 'Creating Spec...'}
                </>
              ) : (
                <>
                  <Plus className="h-4 w-4" />
                  {activeTab === 'mcp' ? 'Create Server' : 'Create OpenAPI Spec'}
                </>
              )}
            </button>
          </div>
        )}
      </div>
    </div>
  );
};