import React, { useState, useEffect } from 'react';
import { X, Plus, Copy, Trash2, ChevronDown, ChevronRight, Settings, Code } from 'lucide-react';
import { apiClient } from '../../api/client';
import Editor from '@monaco-editor/react';

interface AddServerModalProps {
  isOpen: boolean;
  onClose: () => void;
  environmentName: string;
  onSuccess?: () => void;
}

interface EnvVar {
  key: string;
  value: string;
}

export const AddServerModal: React.FC<AddServerModalProps> = ({
  isOpen,
  onClose,
  environmentName,
  onSuccess
}) => {
  const [activeTab, setActiveTab] = useState<'mcp' | 'openapi'>('mcp');
  const [mcpType, setMcpType] = useState<'command' | 'url'>('command');
  const [serverName, setServerName] = useState('');
  const [serverConfig, setServerConfig] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [response, setResponse] = useState<any>(null);
  const [showSuccess, setShowSuccess] = useState(false);
  
  const [isAdvancedMode, setIsAdvancedMode] = useState(false);
  const [formDescription, setFormDescription] = useState('');
  const [formCommand, setFormCommand] = useState('');
  const [formArgs, setFormArgs] = useState<string[]>([]);
  const [formEnv, setFormEnv] = useState<EnvVar[]>([]);
  const [formUrl, setFormUrl] = useState('');
  const [showJsonPreview, setShowJsonPreview] = useState(true);

  // Generate default MCP config dynamically based on server name
  const getDefaultConfig = (name: string, type: 'command' | 'url') => {
    if (type === 'url') {
      return `{
  "mcpServers": {
    "${name || 'server'}": {
      "url": "https://example.com/mcp"
    }
  }
}`;
    }
    return `{
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
  };

  useEffect(() => {
    if (isAdvancedMode || activeTab !== 'mcp') return;

    const key = serverName.trim() || 'server';
    const serverDef: any = {};
    
    if (formDescription.trim()) {
      serverDef.description = formDescription;
    }

    if (mcpType === 'command') {
      serverDef.command = formCommand;
      
      if (formArgs.length > 0) {
        serverDef.args = formArgs.filter(a => a.trim() !== '');
      }
      
      const validEnv = formEnv.filter(e => e.key.trim() !== '');
      if (validEnv.length > 0) {
        const envObj: Record<string, string> = {};
        validEnv.forEach(env => {
          envObj[env.key] = env.value;
        });
        serverDef.env = envObj;
      }
    } else {
      serverDef.url = formUrl;
    }

    const configObj = {
      mcpServers: {
        [key]: serverDef
      }
    };

    setServerConfig(JSON.stringify(configObj, null, 2));
  }, [
    isAdvancedMode, 
    activeTab, 
    mcpType, 
    serverName, 
    formDescription, 
    formCommand, 
    formArgs, 
    formEnv, 
    formUrl
  ]);

  const addArg = () => setFormArgs([...formArgs, '']);
  const updateArg = (index: number, value: string) => {
    const newArgs = [...formArgs];
    newArgs[index] = value;
    setFormArgs(newArgs);
  };
  const removeArg = (index: number) => {
    const newArgs = [...formArgs];
    newArgs.splice(index, 1);
    setFormArgs(newArgs);
  };

  const addEnv = () => setFormEnv([...formEnv, { key: '', value: '' }]);
  const updateEnv = (index: number, field: 'key' | 'value', val: string) => {
    const newEnv = [...formEnv];
    newEnv[index] = { ...newEnv[index], [field]: val };
    setFormEnv(newEnv);
  };
  const removeEnv = (index: number) => {
    const newEnv = [...formEnv];
    newEnv.splice(index, 1);
    setFormEnv(newEnv);
  };

  const insertVariable = (setter: (val: string) => void, currentVal: string, varName: string) => {
    setter(currentVal ? `${currentVal} {{ .${varName} }}` : `{{ .${varName} }}`);
  };

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
        ? getDefaultConfig(serverName, mcpType)
        : getDefaultOpenAPISpec(serverName);
      setServerConfig(newConfig);
      console.log(`[AddServerModal] Auto-populated ${activeTab} config for: ${serverName}`);
    } else if (!serverName) {
      // Clear config if server name is empty
      setServerConfig('');
    }
  }, [serverName, activeTab, mcpType]); // Depend on serverName, activeTab, and mcpType

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
    <div 
      className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-[9999]"
      onClick={onClose}
    >
      <div 
        className="bg-white border border-gray-200 rounded-lg shadow-xl max-w-4xl w-full mx-4 z-[10000] relative max-h-[90vh] flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-gray-200 bg-white rounded-t-lg">
          <h2 className="text-lg font-semibold text-gray-900 z-10 relative">
            Add MCP Server: {environmentName}
          </h2>
          <button onClick={handleClose} className="text-gray-500 hover:text-gray-900 transition-colors z-10 relative">
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Tabs */}
        <div className="flex border-b border-gray-200 bg-gray-50">
          <button
            onClick={() => setActiveTab('mcp')}
            className={`flex-1 px-4 py-3 text-sm font-medium transition-colors ${
              activeTab === 'mcp'
                ? 'text-cyan-600 border-b-2 border-cyan-600 bg-white'
                : 'text-gray-600 hover:text-gray-900'
            }`}
          >
            MCP Config
          </button>
          <button
            onClick={() => setActiveTab('openapi')}
            className={`flex-1 px-4 py-3 text-sm font-medium transition-colors ${
              activeTab === 'openapi'
                ? 'text-cyan-600 border-b-2 border-cyan-600 bg-white'
                : 'text-gray-600 hover:text-gray-900'
            }`}
          >
            OpenAPI Spec
          </button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-6 overflow-y-auto flex-1 min-h-0">
          {!showSuccess ? (
            <>
              <div className="space-y-2">
                <label className="text-sm text-cyan-600 font-medium">
                  {activeTab === 'mcp' ? 'Server Name:' : 'Spec Name:'}
                </label>
                <input
                  type="text"
                  value={serverName}
                  onChange={(e) => setServerName(e.target.value)}
                  className="w-full px-3 py-2 bg-white border border-gray-300 rounded font-mono text-gray-900 focus:outline-none focus:border-cyan-600 focus:ring-1 focus:ring-cyan-600"
                  placeholder="e.g., filesystem, database, etc."
                />
              </div>

              {!isAdvancedMode && activeTab === 'mcp' && (
                <div className="space-y-2">
                  <label className="text-sm text-cyan-600 font-medium">Description (Optional):</label>
                  <input
                    type="text"
                    value={formDescription}
                    onChange={(e) => setFormDescription(e.target.value)}
                    className="w-full px-3 py-2 bg-white border border-gray-300 rounded text-gray-900 focus:outline-none focus:border-cyan-600 focus:ring-1 focus:ring-cyan-600"
                    placeholder="e.g. My helpful assistant"
                  />
                </div>
              )}

              <div className="flex items-center justify-between">
                <label className="text-sm text-cyan-600 font-medium">
                  {activeTab === 'mcp' ? 'Configuration:' : 'OpenAPI Specification (JSON):'}
                </label>
                
                {activeTab === 'mcp' && (
                  <div className="flex items-center gap-4">
                     <div className="flex items-center gap-2">
                        <span className="text-xs text-gray-500">Advanced Mode</span>
                        <button
                          onClick={() => setIsAdvancedMode(!isAdvancedMode)}
                          className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:ring-offset-2 ${
                            isAdvancedMode ? 'bg-cyan-600' : 'bg-gray-200'
                          }`}
                        >
                          <span
                            className={`${
                              isAdvancedMode ? 'translate-x-5' : 'translate-x-1'
                            } inline-block h-3 w-3 transform rounded-full bg-white transition-transform`}
                          />
                        </button>
                     </div>

                     {!isAdvancedMode && (
                        <div className="flex bg-gray-100 rounded-lg p-1">
                          <button
                            onClick={() => setMcpType('command')}
                            className={`px-3 py-1 text-xs font-medium rounded-md transition-all ${
                              mcpType === 'command'
                                ? 'bg-white text-cyan-600 shadow-sm'
                                : 'text-gray-500 hover:text-gray-900'
                            }`}
                          >
                            Command
                          </button>
                          <button
                            onClick={() => setMcpType('url')}
                            className={`px-3 py-1 text-xs font-medium rounded-md transition-all ${
                              mcpType === 'url'
                                ? 'bg-white text-cyan-600 shadow-sm'
                                : 'text-gray-500 hover:text-gray-900'
                            }`}
                          >
                            URL
                          </button>
                        </div>
                     )}
                  </div>
                )}
              </div>

              {activeTab === 'mcp' && !isAdvancedMode ? (
                <div className="space-y-6">
                   {mcpType === 'command' ? (
                     <>
                        <div className="space-y-2">
                           <label className="text-xs text-gray-700 font-medium uppercase tracking-wide">Command</label>
                           <input
                             type="text"
                             value={formCommand}
                             onChange={(e) => setFormCommand(e.target.value)}
                             className="w-full px-3 py-2 bg-white border border-gray-300 rounded font-mono text-gray-900 focus:outline-none focus:border-cyan-600 focus:ring-1 focus:ring-cyan-600"
                             placeholder="e.g. npx, uvx, docker"
                           />
                        </div>

                        <div className="space-y-2">
                           <div className="flex items-center justify-between">
                             <label className="text-xs text-gray-700 font-medium uppercase tracking-wide">Arguments</label>
                             <button onClick={addArg} className="text-xs text-cyan-600 hover:text-cyan-700 flex items-center gap-1">
                               <Plus className="h-3 w-3" /> Add Arg
                             </button>
                           </div>
                           {formArgs.map((arg, idx) => (
                             <div key={idx} className="flex gap-2">
                               <input
                                 type="text"
                                 value={arg}
                                 onChange={(e) => updateArg(idx, e.target.value)}
                                 className="flex-1 px-3 py-2 bg-white border border-gray-300 rounded font-mono text-gray-900 focus:outline-none focus:border-cyan-600 focus:ring-1 focus:ring-cyan-600"
                                 placeholder={`Arg ${idx + 1}`}
                               />
                               <button onClick={() => removeArg(idx)} className="text-gray-400 hover:text-red-500">
                                 <Trash2 className="h-4 w-4" />
                               </button>
                             </div>
                           ))}
                           {formArgs.length === 0 && (
                             <div className="text-xs text-gray-400 italic">No arguments added.</div>
                           )}
                        </div>

                        <div className="space-y-2">
                           <div className="flex items-center justify-between">
                             <label className="text-xs text-gray-700 font-medium uppercase tracking-wide">Environment Variables</label>
                             <button onClick={addEnv} className="text-xs text-cyan-600 hover:text-cyan-700 flex items-center gap-1">
                               <Plus className="h-3 w-3" /> Add Env Var
                             </button>
                           </div>
                           {formEnv.map((env, idx) => (
                             <div key={idx} className="flex gap-2 items-start">
                               <input
                                 type="text"
                                 value={env.key}
                                 onChange={(e) => updateEnv(idx, 'key', e.target.value)}
                                 className="w-1/3 px-3 py-2 bg-white border border-gray-300 rounded font-mono text-gray-900 focus:outline-none focus:border-cyan-600 focus:ring-1 focus:ring-cyan-600"
                                 placeholder="KEY"
                               />
                               <div className="flex-1 relative">
                                  <input
                                    type="text"
                                    value={env.value}
                                    onChange={(e) => updateEnv(idx, 'value', e.target.value)}
                                    className="w-full px-3 py-2 bg-white border border-gray-300 rounded font-mono text-gray-900 focus:outline-none focus:border-cyan-600 focus:ring-1 focus:ring-cyan-600"
                                    placeholder="VALUE"
                                  />
                                  <button 
                                    onClick={() => insertVariable((val) => updateEnv(idx, 'value', val), env.value, "VAR_NAME")}
                                    className="absolute right-2 top-2 text-xs text-blue-500 hover:text-blue-700"
                                    title="Insert variable"
                                  >
                                    {"{{}}"}
                                  </button>
                               </div>
                               <button onClick={() => removeEnv(idx)} className="text-gray-400 hover:text-red-500 pt-2">
                                 <Trash2 className="h-4 w-4" />
                               </button>
                             </div>
                           ))}
                           {formEnv.length === 0 && (
                             <div className="text-xs text-gray-400 italic">No environment variables added.</div>
                           )}
                        </div>
                     </>
                   ) : (
                     <div className="space-y-2">
                        <label className="text-xs text-gray-700 font-medium uppercase tracking-wide">Server URL</label>
                        <div className="relative">
                          <input
                            type="text"
                            value={formUrl}
                            onChange={(e) => setFormUrl(e.target.value)}
                            className="w-full px-3 py-2 bg-white border border-gray-300 rounded font-mono text-gray-900 focus:outline-none focus:border-cyan-600 focus:ring-1 focus:ring-cyan-600"
                            placeholder="https://example.com/sse"
                          />
                           <button 
                              onClick={() => insertVariable(setFormUrl, formUrl, "MCP_URL")}
                              className="absolute right-2 top-2 text-xs text-blue-500 hover:text-blue-700"
                              title="Insert variable"
                            >
                              {"{{}}"}
                           </button>
                        </div>
                     </div>
                   )}
                   
                   <div className="border border-gray-200 rounded-lg overflow-hidden">
                     <button
                        onClick={() => setShowJsonPreview(!showJsonPreview)}
                        className="w-full px-4 py-2 bg-gray-50 flex items-center justify-between text-xs font-medium text-gray-600 hover:bg-gray-100"
                     >
                       <span>JSON Preview</span>
                       {showJsonPreview ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                     </button>
                     {showJsonPreview && (
                       <div className="bg-gray-50 p-3 border-t border-gray-200">
                         <pre className="text-xs font-mono text-gray-600 overflow-x-auto whitespace-pre-wrap">
                           {serverConfig}
                         </pre>
                       </div>
                     )}
                   </div>
                </div>
              ) : (
                <div className="border border-gray-300 rounded overflow-hidden">
                  <Editor
                    height="320px"
                    defaultLanguage="json"
                    value={serverConfig}
                    onChange={(value) => setServerConfig(value || '')}
                    theme="vs-light"
                    options={{
                      minimap: { enabled: false },
                      fontSize: 12,
                      lineNumbers: 'on',
                      scrollBeyondLastLine: false,
                      wordWrap: 'on',
                      automaticLayout: true,
                      tabSize: 2,
                      formatOnPaste: true,
                      formatOnType: true,
                      folding: true,
                      bracketPairColorization: { enabled: true },
                      padding: { top: 8, bottom: 8 },
                    }}
                  />
                </div>
              )}



              {/* Documentation Note */}
              <div className="bg-blue-50 border border-blue-200 rounded p-4">
                <p className="text-sm text-gray-700">
                  {activeTab === 'mcp' ? (
                    <>
                      <strong>Note:</strong> Replace any arguments you want as variables with <code className="bg-gray-100 px-1 rounded">{'{{ .VAR }}'}</code> Go variable notation.{' '}
                      <a
                        href="https://cloudshipai.github.io/station/en/mcp/overview/"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-blue-600 underline hover:text-blue-700"
                      >
                        More info here
                      </a>
                    </>
                  ) : (
                    <>
                      <strong>Note:</strong> Use <code className="bg-gray-100 px-1 rounded">{'{{ .VAR }}'}</code> for template variables in your spec.
                      {' '}For authentication, add security schemes in the <code className="bg-gray-100 px-1 rounded">components.securitySchemes</code> section.
                      {' '}
                      <a
                        href="https://cloudshipai.github.io/station/en/mcp/openapi/"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-blue-600 underline hover:text-blue-700"
                      >
                        More info here
                      </a>
                    </>
                  )}
                </p>
              </div>

              {/* Error Display */}
              {response?.error && (
                <div className="bg-red-50 border border-red-200 rounded p-4">
                  <h4 className="text-sm text-red-600 font-medium mb-2">Error</h4>
                  <div className="text-xs text-red-600">
                    {response.error}
                  </div>
                </div>
              )}
            </>
          ) : (
            /* Success Card */
            <div className="space-y-4">
              <div className="bg-green-50 border border-green-200 rounded p-6 text-center">
                <h3 className="text-lg text-gray-900 font-medium mb-4">MCP Server Created Successfully!</h3>

                <div className="space-y-3 text-left">
                  <div>
                    <span className="text-xs text-green-600 font-medium">Server Name:</span>
                    <div className="mt-1 p-2 bg-white border border-gray-200 rounded font-mono text-xs text-gray-900">
                      {serverName}
                    </div>
                  </div>

                  <div>
                    <span className="text-xs text-green-600 font-medium">Environment:</span>
                    <div className="mt-1 p-2 bg-white border border-gray-200 rounded font-mono text-xs text-gray-900">
                      {environmentName}
                    </div>
                  </div>
                </div>
              </div>

              {/* Next Steps */}
              <div className="bg-blue-50 border border-blue-200 rounded p-4">
                <h4 className="text-sm text-blue-600 font-medium mb-3">Next Steps</h4>
                <p className="text-xs text-gray-700 mb-3">
                  Sync this config and input your variables:
                </p>

                <div className="bg-white border border-gray-200 rounded p-3 flex items-center justify-between">
                  <code className="text-xs text-gray-900 font-mono">stn sync</code>
                  <button
                    onClick={() => navigator.clipboard.writeText('stn sync')}
                    className="p-1 text-blue-600 hover:text-blue-700 transition-colors"
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
          <div className="p-4 border-t border-gray-200">
            <button
              onClick={handleSubmit}
              disabled={
                isLoading || 
                !serverName.trim() || 
                (!isAdvancedMode && activeTab === 'mcp' && (
                  (mcpType === 'command' && !formCommand.trim()) || 
                  (mcpType === 'url' && !formUrl.trim())
                )) || 
                ((isAdvancedMode || activeTab === 'openapi') && !serverConfig.trim())
              }
              className="w-full px-4 py-2 bg-cyan-600 text-white rounded font-medium hover:bg-cyan-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            >
              {isLoading ? (
                <>
                  <div className="animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent"></div>
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