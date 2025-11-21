import React, { useState, useEffect } from 'react';
import { X, Database } from 'lucide-react';
import { mcpServersApi } from '../../api/station';
import { apiClient } from '../../api/client';

interface MCPServerDetailsModalProps {
  serverId: number | null;
  isOpen: boolean;
  onClose: () => void;
}

export const MCPServerDetailsModal: React.FC<MCPServerDetailsModalProps> = ({ serverId, isOpen, onClose }) => {
  const [serverDetails, setServerDetails] = useState<any>(null);
  const [serverTools, setServerTools] = useState<any[]>([]);

  useEffect(() => {
    if (isOpen && serverId) {
      const fetchServerDetails = async () => {
        try {
          // Fetch server details
          const serverResponse = await mcpServersApi.getById(serverId);
          setServerDetails(serverResponse.data);

          // Fetch tools for this server
          try {
            const toolsResponse = await apiClient.get(`/mcp-servers/${serverId}/tools`);
            setServerTools(Array.isArray(toolsResponse.data) ? toolsResponse.data : []);
          } catch (toolsError) {
            console.error('Failed to fetch server tools:', toolsError);
            setServerTools([]);
          }
        } catch (error) {
          console.error('Failed to fetch server details:', error);
          setServerDetails(null);
          setServerTools([]);
        }
      };
      fetchServerDetails();
    }
  }, [isOpen, serverId]);

  if (!isOpen || !serverId) return null;

  return (
    <div className="fixed inset-0 bg-black/30 flex items-center justify-center z-50 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-white border border-gray-200 rounded-xl shadow-lg w-full max-w-4xl mx-4 max-h-[90vh] flex flex-col animate-in zoom-in-95 fade-in slide-in-from-bottom-4 duration-300">
        <div className="flex items-center justify-between p-6 border-b border-gray-200">
          <div className="flex items-center gap-3">
            <Database className="h-6 w-6 text-primary" />
            <h2 className="text-xl font-semibold text-gray-900">
              MCP Server Details: {serverDetails?.name}
            </h2>
          </div>
          <button
            onClick={onClose}
            className="p-2 hover:bg-gray-100 rounded transition-colors"
          >
            <X className="h-5 w-5 text-gray-500 hover:text-gray-900" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-6 bg-gray-50">
          {serverDetails && (
            <div className="space-y-6">
              {/* Server Configuration */}
              <div className="bg-white border border-gray-200 rounded-lg p-5">
                <h3 className="text-lg font-semibold text-gray-900 mb-4 flex items-center gap-2">
                  <Database className="h-5 w-5 text-primary" />
                  Configuration
                </h3>
                <div className="grid gap-3">
                  <div className="flex flex-col gap-1 p-3 bg-gray-50 border border-gray-200 rounded-lg">
                    <span className="text-sm font-medium text-gray-600">Command:</span>
                    <span className="text-gray-900 font-mono text-sm">{serverDetails.command}</span>
                  </div>
                  <div className="flex flex-col gap-1 p-3 bg-gray-50 border border-gray-200 rounded-lg">
                    <span className="text-sm font-medium text-gray-600">Arguments:</span>
                    <span className="text-gray-900 font-mono text-sm break-all">{serverDetails.args ? serverDetails.args.join(' ') : 'None'}</span>
                  </div>
                  <div className="flex justify-between items-center p-3 bg-gray-50 border border-gray-200 rounded-lg">
                    <span className="text-sm font-medium text-gray-600">Environment ID:</span>
                    <span className="text-gray-900 font-medium">{serverDetails.environment_id}</span>
                  </div>
                  <div className="flex justify-between items-center p-3 bg-gray-50 border border-gray-200 rounded-lg">
                    <span className="text-sm font-medium text-gray-600">Created:</span>
                    <span className="text-gray-900 font-medium">{new Date(serverDetails.created_at).toLocaleString()}</span>
                  </div>
                  <div className="flex justify-between items-center p-3 bg-gray-50 border border-gray-200 rounded-lg">
                    <span className="text-sm font-medium text-gray-600">Timeout:</span>
                    <span className="text-gray-900 font-medium">{serverDetails.timeout_seconds || 30}s</span>
                  </div>
                  <div className="flex justify-between items-center p-3 bg-gray-50 border border-gray-200 rounded-lg">
                    <span className="text-sm font-medium text-gray-600">Auto Restart:</span>
                    <span className="text-gray-900 font-medium">{serverDetails.auto_restart ? 'Yes' : 'No'}</span>
                  </div>
                </div>
              </div>

              {/* Available Tools */}
              <div className="bg-white border border-gray-200 rounded-lg p-5">
                <h3 className="text-lg font-semibold text-gray-900 mb-4">
                  Available Tools ({serverTools.length})
                </h3>
                {serverTools.length === 0 ? (
                  <div className="text-center p-8 bg-gray-50 border border-gray-200 rounded-lg">
                    <Database className="h-12 w-12 text-gray-400 mx-auto mb-3" />
                    <div className="text-gray-600">No tools found for this server</div>
                  </div>
                ) : (
                  <div className="grid gap-3">
                    {serverTools.map((tool, index) => (
                      <div key={tool.id || index} className="p-4 bg-gray-50 border border-gray-200 rounded-lg hover:border-gray-300 transition-colors">
                        <h4 className="font-semibold text-gray-900 mb-2">{tool.name}</h4>
                        <p className="text-sm text-gray-600 mb-2">{tool.description}</p>
                        {tool.input_schema && (
                          <div className="mt-3">
                            <div className="text-xs font-medium text-gray-600 mb-2">Input Schema:</div>
                            <pre className="text-xs bg-white p-3 rounded border border-gray-200 overflow-x-auto text-gray-900">
                              {JSON.stringify(JSON.parse(tool.input_schema), null, 2)}
                            </pre>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
