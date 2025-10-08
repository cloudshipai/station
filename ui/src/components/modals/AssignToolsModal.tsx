import React, { useState } from 'react';
import { X, Link, CheckCircle, AlertCircle, Loader } from 'lucide-react';
import axios from 'axios';

interface Environment {
  id: number;
  name: string;
  description?: string;
}

interface AssignToolsModalProps {
  isOpen: boolean;
  onClose: () => void;
  targetEnvironmentId: number;
  environments: Environment[];
  onAssignComplete: () => void;
}

interface AssignResult {
  success: boolean;
  source_environment: string;
  target_environment: string;
  tools_assigned: number;
  message: string;
}

export const AssignToolsModal: React.FC<AssignToolsModalProps> = ({
  isOpen,
  onClose,
  targetEnvironmentId,
  environments,
  onAssignComplete
}) => {
  const [selectedSourceEnv, setSelectedSourceEnv] = useState<number | null>(null);
  const [assigning, setAssigning] = useState(false);
  const [result, setResult] = useState<AssignResult | null>(null);

  const targetEnv = environments.find(e => e.id === targetEnvironmentId);

  const handleAssign = async () => {
    if (!selectedSourceEnv) return;

    setAssigning(true);
    setResult(null);

    try {
      const response = await axios.post(
        `http://localhost:8585/api/v1/environments/${targetEnvironmentId}/assign-tools-from/${selectedSourceEnv}`
      );
      setResult(response.data);

      // Call onAssignComplete after successful assignment
      if (response.data.success) {
        setTimeout(() => {
          onAssignComplete();
        }, 1500);
      }
    } catch (error: any) {
      console.error('Failed to assign tools:', error);
      setResult({
        success: false,
        source_environment: '',
        target_environment: targetEnv?.name || '',
        tools_assigned: 0,
        message: error.response?.data?.error || 'Failed to assign tools'
      });
    } finally {
      setAssigning(false);
    }
  };

  const handleClose = () => {
    setResult(null);
    setSelectedSourceEnv(null);
    onClose();
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-[9999]">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo-glow max-w-2xl w-full mx-4 z-[10000] relative max-h-[90vh] overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark rounded-t-lg">
          <div className="flex items-center gap-2">
            <Link className="h-5 w-5 text-tokyo-purple" />
            <h2 className="text-lg font-mono font-semibold text-tokyo-fg z-10 relative">
              Assign Tools to: {targetEnv?.name}
            </h2>
          </div>
          <button onClick={handleClose} className="text-tokyo-comment hover:text-tokyo-fg transition-colors z-10 relative">
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-4 space-y-4 overflow-y-auto flex-1">
          {!result ? (
            <>
              {/* Info */}
              <div className="bg-tokyo-blue7 bg-opacity-20 border border-tokyo-blue7 rounded p-3">
                <p className="text-sm text-tokyo-fg font-mono">
                  Assign tools to agents in <span className="text-tokyo-purple font-semibold">{targetEnv?.name}</span> by matching tool names from a source environment.
                </p>
                <p className="text-sm text-tokyo-comment font-mono mt-2">
                  This will find agents with matching names and assign their tools based on tool name and MCP server.
                </p>
              </div>

              {/* Source Environment Selection */}
              <div className="space-y-3">
                <label className="text-sm font-mono text-tokyo-comment">Source Environment (copy tools from):</label>
                <select
                  value={selectedSourceEnv || ''}
                  onChange={(e) => setSelectedSourceEnv(Number(e.target.value))}
                  className="w-full px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 rounded text-tokyo-fg font-mono text-sm focus:outline-none focus:ring-2 focus:ring-tokyo-purple focus:border-transparent"
                  disabled={assigning}
                >
                  <option value="">Select source environment...</option>
                  {environments
                    .filter(env => env.id !== targetEnvironmentId)
                    .map(env => (
                      <option key={env.id} value={env.id}>
                        {env.name} {env.description && `- ${env.description}`}
                      </option>
                    ))}
                </select>
              </div>

              {/* Prerequisites */}
              <div className="bg-yellow-900 bg-opacity-30 border border-yellow-500 border-opacity-50 rounded p-3">
                <p className="text-sm text-yellow-300 font-mono font-semibold mb-2">
                  Prerequisites:
                </p>
                <ul className="text-sm text-yellow-300 font-mono list-disc list-inside space-y-1">
                  <li>Target environment must have been synced (<code className="bg-black bg-opacity-30 px-1 rounded">stn sync {targetEnv?.name}</code>)</li>
                  <li>Agents must exist in both environments with matching names</li>
                  <li>MCP servers must exist in both environments with matching names</li>
                </ul>
              </div>
            </>
          ) : (
            <>
              {/* Result Display */}
              <div className={`border rounded p-4 ${result.success ? 'bg-green-900 bg-opacity-20 border-green-500' : 'bg-red-900 bg-opacity-20 border-red-500'}`}>
                <div className="flex items-center gap-2 mb-2">
                  {result.success ? (
                    <CheckCircle className="h-5 w-5 text-green-400" />
                  ) : (
                    <AlertCircle className="h-5 w-5 text-red-400" />
                  )}
                  <span className={`font-mono font-semibold ${result.success ? 'text-green-400' : 'text-red-400'}`}>
                    {result.success ? 'Tools Assigned Successfully' : 'Assignment Failed'}
                  </span>
                </div>
                <p className="text-sm font-mono text-tokyo-fg mb-3">
                  {result.message}
                </p>
                {result.success && (
                  <div className="text-sm font-mono">
                    <span className="text-tokyo-comment">Tools Assigned: </span>
                    <span className="text-tokyo-purple font-semibold">{result.tools_assigned}</span>
                  </div>
                )}
              </div>

              {/* Success Info */}
              {result.success && (
                <div className="bg-tokyo-blue7 bg-opacity-20 border border-tokyo-blue7 rounded p-3">
                  <p className="text-sm text-tokyo-fg font-mono">
                    ✅ Agent .prompt files have been regenerated with tools
                  </p>
                  <p className="text-sm text-tokyo-comment font-mono mt-1">
                    Refresh the page to see updated agent configurations
                  </p>
                </div>
              )}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 p-4 border-t border-tokyo-blue7 bg-tokyo-bg-dark rounded-b-lg">
          {!result ? (
            <>
              <button
                onClick={handleClose}
                className="px-4 py-2 text-sm font-mono text-tokyo-comment hover:text-tokyo-fg transition-colors"
                disabled={assigning}
              >
                Cancel
              </button>
              <button
                onClick={handleAssign}
                disabled={!selectedSourceEnv || assigning}
                className="px-4 py-2 bg-tokyo-purple text-tokyo-bg rounded font-mono text-sm hover:bg-opacity-90 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
              >
                {assigning && <Loader className="h-4 w-4 animate-spin" />}
                {assigning ? 'Assigning Tools...' : 'Assign Tools'}
              </button>
            </>
          ) : (
            <button
              onClick={handleClose}
              className="px-4 py-2 bg-tokyo-green text-tokyo-bg rounded font-mono text-sm hover:bg-opacity-90"
            >
              {result.success ? 'Done' : 'Close'}
            </button>
          )}
        </div>
      </div>
    </div>
  );
};
