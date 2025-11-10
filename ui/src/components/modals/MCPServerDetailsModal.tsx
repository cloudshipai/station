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
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 max-w-4xl w-full mx-4 max-h-[80vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-mono font-semibold text-tokyo-cyan">
            MCP Server Details: {serverDetails?.name}
          </h2>
          <button
            onClick={onClose}
            className="p-2 hover:bg-tokyo-bg-highlight rounded transition-colors"
          >
            <X className="h-5 w-5 text-tokyo-comment hover:text-tokyo-fg" />
          </button>
        </div>

        {serverDetails && (
          <div className="space-y-6">
            {/* Server Configuration */}
            <div>
              <h3 className="text-lg font-mono font-medium text-tokyo-blue mb-3">Configuration</h3>
              <div className="grid gap-3">
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Command:</span>
                  <span className="font-mono text-tokyo-fg">{serverDetails.command}</span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Arguments:</span>
                  <span className="font-mono text-tokyo-fg">{serverDetails.args ? serverDetails.args.join(' ') : 'None'}</span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Environment ID:</span>
                  <span className="font-mono text-tokyo-fg">{serverDetails.environment_id}</span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Created:</span>
                  <span className="font-mono text-tokyo-fg">{new Date(serverDetails.created_at).toLocaleString()}</span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Timeout:</span>
                  <span className="font-mono text-tokyo-fg">{serverDetails.timeout_seconds || 30}s</span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Auto Restart:</span>
                  <span className="font-mono text-tokyo-fg">{serverDetails.auto_restart ? 'Yes' : 'No'}</span>
                </div>
              </div>
            </div>

            {/* Available Tools */}
            <div>
              <h3 className="text-lg font-mono font-medium text-tokyo-green mb-3">
                Available Tools ({serverTools.length})
              </h3>
              {serverTools.length === 0 ? (
                <div className="text-center p-6 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <Database className="h-12 w-12 text-tokyo-comment mx-auto mb-3" />
                  <div className="text-tokyo-comment font-mono">No tools found for this server</div>
                </div>
              ) : (
                <div className="grid gap-3">
                  {serverTools.map((tool, index) => (
                    <div key={tool.id || index} className="p-4 bg-tokyo-bg border border-tokyo-blue7 rounded">
                      <h4 className="font-mono font-medium text-tokyo-green mb-2">{tool.name}</h4>
                      <p className="text-sm text-tokyo-comment font-mono mb-2">{tool.description}</p>
                      {tool.input_schema && (
                        <div className="mt-2">
                          <div className="text-xs text-tokyo-comment font-mono mb-1">Input Schema:</div>
                          <pre className="text-xs bg-tokyo-bg-dark p-2 rounded border border-tokyo-blue7 overflow-x-auto text-tokyo-fg font-mono">
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
  );
};
