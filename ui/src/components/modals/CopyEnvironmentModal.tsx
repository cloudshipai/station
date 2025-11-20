import React, { useState, useEffect } from 'react';
import { X, Copy, CheckCircle, AlertCircle, Loader } from 'lucide-react';
import axios from 'axios';

interface Environment {
  id: number;
  name: string;
  description?: string;
}

interface CopyEnvironmentModalProps {
  isOpen: boolean;
  onClose: () => void;
  sourceEnvironmentId: number;
  sourceEnvironmentName: string;
  environments: Environment[];
  onCopyComplete: () => void;
}

interface CopyResult {
  success: boolean;
  source_environment: string;
  target_environment: string;
  mcp_servers_copied: number;
  agents_copied: number;
  mcp_servers_synced?: number;
  tools_assigned?: number;
  sync_failed?: boolean;
  sync_succeeded?: boolean;
  conflicts: Array<{
    type: string;
    name: string;
    reason: string;
  }>;
  errors: string[];
  message: string;
}

export const CopyEnvironmentModal: React.FC<CopyEnvironmentModalProps> = ({
  isOpen,
  onClose,
  sourceEnvironmentId,
  sourceEnvironmentName,
  environments,
  onCopyComplete
}) => {
  const [selectedTargetEnv, setSelectedTargetEnv] = useState<number | null>(null);
  const [copying, setCopying] = useState(false);
  const [result, setResult] = useState<CopyResult | null>(null);
  const [showConflicts, setShowConflicts] = useState(false);

  // Reset state when modal opens
  useEffect(() => {
    if (isOpen) {
      setResult(null);
      setSelectedTargetEnv(null);
      setShowConflicts(false);
      setCopying(false);
    }
  }, [isOpen]);

  // Filter out source environment from target list
  const availableTargets = environments.filter(env => env.id !== sourceEnvironmentId);

  const handleCopy = async () => {
    if (!selectedTargetEnv) return;

    setCopying(true);
    setResult(null);

    try {
      const response = await axios.post(
        `http://localhost:8585/api/v1/environments/${sourceEnvironmentId}/copy`,
        { target_environment_id: selectedTargetEnv }
      );
      setResult(response.data);
    } catch (error: any) {
      console.error('Failed to copy environment:', error);
      setResult({
        success: false,
        source_environment: sourceEnvironmentName,
        target_environment: '',
        mcp_servers_copied: 0,
        agents_copied: 0,
        conflicts: [],
        errors: [error.response?.data?.error || 'Failed to copy environment'],
        message: 'Copy failed'
      });
    } finally {
      setCopying(false);
    }
  };

  const handleClose = () => {
    setResult(null);
    setSelectedTargetEnv(null);
    setShowConflicts(false);
    onClose();
  };

  const handleComplete = () => {
    // Just close the modal directly
    onClose();
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-[9999]">
      <div className="bg-white border border-gray-200 rounded-lg shadow-xl max-w-2xl w-full mx-4 z-[10000] relative max-h-[90vh] overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-gray-200 bg-white rounded-t-lg">
          <div className="flex items-center gap-2">
            <Copy className="h-5 w-5 text-blue-600" />
            <h2 className="text-lg font-semibold text-gray-900 z-10 relative">
              Copy Environment: {sourceEnvironmentName}
            </h2>
          </div>
          <button onClick={handleClose} className="text-gray-600 hover:text-gray-900 transition-colors z-10 relative">
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-4 space-y-4 overflow-y-auto flex-1">
          {!result ? (
            <>
              {/* Info */}
              <div className="bg-blue-50 border border-blue-200 rounded-lg p-3">
                <p className="text-sm text-gray-900">
                  This will copy all agents and MCP servers from <span className="text-blue-600 font-semibold">{sourceEnvironmentName}</span> to the selected target environment, then automatically sync and assign tools.
                </p>
              </div>

              {/* Target Environment Selection */}
              <div className="space-y-3">
                <label className="text-sm font-medium text-gray-900">Target Environment:</label>
                <select
                  value={selectedTargetEnv || ''}
                  onChange={(e) => setSelectedTargetEnv(Number(e.target.value))}
                  className="w-full px-3 py-2 bg-white border border-gray-300 rounded text-gray-900 text-sm focus:outline-none focus:ring-2 focus:ring-station-blue focus:border-station-blue"
                  disabled={copying}
                >
                  <option value="">Select target environment...</option>
                  {availableTargets.map(env => (
                    <option key={env.id} value={env.id}>
                      {env.name} {env.description && `- ${env.description}`}
                    </option>
                  ))}
                </select>
              </div>

              {/* Info about automatic process */}
              <div className="bg-green-50 border border-green-200 rounded-lg p-3">
                <p className="text-sm text-gray-900">
                  <strong>Automatic Process:</strong>
                  <br />
                  1. Copy MCP servers and agents to target environment
                  <br />
                  2. Auto-sync target environment to discover tools
                  <br />
                  3. Auto-assign tools to agents based on source environment
                </p>
              </div>
            </>
          ) : (
            <>
              {/* Result Display */}
              <div className={`border rounded-lg p-4 ${result.success ? 'bg-green-50 border-green-200' : 'bg-red-50 border-red-200'}`}>
                <div className="flex items-center gap-2 mb-2">
                  {result.success ? (
                    <CheckCircle className="h-5 w-5 text-green-600" />
                  ) : (
                    <AlertCircle className="h-5 w-5 text-red-600" />
                  )}
                  <span className={`font-semibold ${result.success ? 'text-green-600' : 'text-red-600'}`}>
                    {result.success ? 'Copy Completed' : 'Copy Failed'}
                  </span>
                </div>
                <p className="text-sm text-gray-900 mb-3">
                  {result.message}
                </p>
                <div className="grid grid-cols-2 gap-2 text-sm">
                  <div>
                    <span className="text-gray-600">MCP Servers Copied: </span>
                    <span className="text-blue-600 font-semibold">{result.mcp_servers_copied}</span>
                  </div>
                  <div>
                    <span className="text-gray-600">Agents Copied: </span>
                    <span className="text-blue-600 font-semibold">{result.agents_copied}</span>
                  </div>
                  {result.mcp_servers_synced !== undefined && (
                    <div>
                      <span className="text-gray-600">MCP Servers Synced: </span>
                      <span className="text-green-600 font-semibold">{result.mcp_servers_synced}</span>
                    </div>
                  )}
                  {result.tools_assigned !== undefined && (
                    <div>
                      <span className="text-gray-600">Tools Assigned: </span>
                      <span className="text-purple-600 font-semibold">{result.tools_assigned}</span>
                    </div>
                  )}
                </div>
              </div>

              {/* Conflicts Section */}
              {result.conflicts && result.conflicts.length > 0 && (
                <div className="space-y-2">
                  <button
                    onClick={() => setShowConflicts(!showConflicts)}
                    className="flex items-center gap-2 text-sm text-yellow-600 hover:text-yellow-700"
                  >
                    <AlertCircle className="h-4 w-4" />
                    {result.conflicts.length} conflict{result.conflicts.length > 1 ? 's' : ''} detected
                    <span className="text-xs">{showConflicts ? '▼' : '▶'}</span>
                  </button>
                  {showConflicts && (
                    <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-3 max-h-48 overflow-y-auto">
                      {result.conflicts.map((conflict, idx) => (
                        <div key={idx} className="text-sm text-gray-900 mb-2">
                          <span className="text-yellow-700 font-semibold">{conflict.type}:</span> {conflict.name}
                          <br />
                          <span className="text-gray-600 text-xs">{conflict.reason}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* Errors Section */}
              {result.errors && result.errors.length > 0 && (
                <div className="bg-red-50 border border-red-200 rounded-lg p-3 max-h-48 overflow-y-auto">
                  {result.errors.map((error, idx) => (
                    <div key={idx} className="text-sm text-red-600 mb-1">
                      • {error}
                    </div>
                  ))}
                </div>
              )}

              {/* Success Summary */}
              {result.success && result.tools_assigned !== undefined && result.tools_assigned > 0 && (
                <div className="bg-green-50 border border-green-200 rounded-lg p-3">
                  <p className="text-sm text-green-700 font-semibold">
                    ✅ Complete! Environment copied, synced, and tools assigned automatically.
                  </p>
                  <p className="text-sm text-gray-900 mt-2">
                    Your agents in <span className="text-blue-600 font-semibold">{result.target_environment}</span> are ready to use!
                  </p>
                </div>
              )}

              {/* Partial Success - Sync Failed */}
              {result.sync_failed && (
                <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-3">
                  <p className="text-sm text-yellow-800 font-semibold">
                    ⚠️ Copied but sync failed
                  </p>
                  <p className="text-sm text-gray-900 mt-2">
                    MCP servers and agents were copied, but auto-sync failed. Run <code className="bg-gray-100 px-1 rounded">stn sync {result.target_environment}</code> manually.
                  </p>
                </div>
              )}

              {/* Partial Success - Tool Assignment Failed */}
              {result.sync_succeeded && result.tools_assigned === 0 && (
                <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-3">
                  <p className="text-sm text-yellow-800 font-semibold">
                    ⚠️ Copied and synced but tool assignment failed
                  </p>
                  <p className="text-sm text-gray-900 mt-2">
                    MCP servers connected successfully, but tool assignment failed. You may need to manually assign tools to agents.
                  </p>
                </div>
              )}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 p-4 border-t border-gray-200 bg-white rounded-b-lg">
          {!result ? (
            <>
              <button
                onClick={handleClose}
                className="px-4 py-2 text-sm text-gray-700 hover:text-gray-900 transition-colors"
                disabled={copying}
              >
                Cancel
              </button>
              <button
                onClick={handleCopy}
                disabled={!selectedTargetEnv || copying}
                className="px-4 py-2 bg-station-blue text-white rounded text-sm hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
              >
                {copying && <Loader className="h-4 w-4 animate-spin" />}
                {copying ? 'Copying...' : 'Copy Environment'}
              </button>
            </>
          ) : (
            <>
              <button
                onClick={handleClose}
                className="px-4 py-2 bg-station-blue text-white rounded text-sm hover:bg-blue-600"
              >
                {result?.success ? 'Done' : 'Close'}
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
};
