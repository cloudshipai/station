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
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-[9999]">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo-glow max-w-2xl w-full mx-4 z-[10000] relative max-h-[90vh] overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark rounded-t-lg">
          <div className="flex items-center gap-2">
            <Copy className="h-5 w-5 text-tokyo-orange" />
            <h2 className="text-lg font-mono font-semibold text-tokyo-fg z-10 relative">
              Copy Environment: {sourceEnvironmentName}
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
                  This will copy all agents and MCP servers from <span className="text-tokyo-orange font-semibold">{sourceEnvironmentName}</span> to the selected target environment.
                </p>
              </div>

              {/* Target Environment Selection */}
              <div className="space-y-3">
                <label className="text-sm font-mono text-tokyo-comment">Target Environment:</label>
                <select
                  value={selectedTargetEnv || ''}
                  onChange={(e) => setSelectedTargetEnv(Number(e.target.value))}
                  className="w-full px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 rounded text-tokyo-fg font-mono text-sm focus:outline-none focus:ring-2 focus:ring-tokyo-orange focus:border-transparent"
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

              {/* Warning about workflow */}
              <div className="bg-yellow-900 bg-opacity-30 border border-yellow-500 border-opacity-50 rounded p-3">
                <p className="text-sm text-yellow-300 font-mono">
                  <strong>Note:</strong> After copying, you must:
                  <br />
                  1. Run <code className="bg-black bg-opacity-30 px-1 rounded">stn sync [target]</code> to discover tools
                  <br />
                  2. Use the "Assign Tools" button to complete the copy
                </p>
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
                    {result.success ? 'Copy Completed' : 'Copy Failed'}
                  </span>
                </div>
                <p className="text-sm font-mono text-tokyo-fg mb-3">
                  {result.message}
                </p>
                <div className="grid grid-cols-2 gap-2 text-sm font-mono">
                  <div>
                    <span className="text-tokyo-comment">MCP Servers: </span>
                    <span className="text-tokyo-cyan font-semibold">{result.mcp_servers_copied}</span>
                  </div>
                  <div>
                    <span className="text-tokyo-comment">Agents: </span>
                    <span className="text-tokyo-blue font-semibold">{result.agents_copied}</span>
                  </div>
                </div>
              </div>

              {/* Conflicts Section */}
              {result.conflicts && result.conflicts.length > 0 && (
                <div className="space-y-2">
                  <button
                    onClick={() => setShowConflicts(!showConflicts)}
                    className="flex items-center gap-2 text-sm font-mono text-yellow-400 hover:text-yellow-300"
                  >
                    <AlertCircle className="h-4 w-4" />
                    {result.conflicts.length} conflict{result.conflicts.length > 1 ? 's' : ''} detected
                    <span className="text-xs">{showConflicts ? '▼' : '▶'}</span>
                  </button>
                  {showConflicts && (
                    <div className="bg-yellow-900 bg-opacity-20 border border-yellow-500 border-opacity-50 rounded p-3 max-h-48 overflow-y-auto">
                      {result.conflicts.map((conflict, idx) => (
                        <div key={idx} className="text-sm font-mono text-yellow-300 mb-2">
                          <span className="text-yellow-400 font-semibold">{conflict.type}:</span> {conflict.name}
                          <br />
                          <span className="text-yellow-200 text-xs">{conflict.reason}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* Errors Section */}
              {result.errors && result.errors.length > 0 && (
                <div className="bg-red-900 bg-opacity-20 border border-red-500 border-opacity-50 rounded p-3 max-h-48 overflow-y-auto">
                  {result.errors.map((error, idx) => (
                    <div key={idx} className="text-sm font-mono text-red-300 mb-1">
                      • {error}
                    </div>
                  ))}
                </div>
              )}

              {/* Next Steps */}
              {result.success && (
                <div className="bg-tokyo-blue7 bg-opacity-20 border border-tokyo-blue7 rounded p-3">
                  <p className="text-sm text-tokyo-fg font-mono mb-2 font-semibold">
                    Next Steps:
                  </p>
                  <ol className="text-sm text-tokyo-fg font-mono list-decimal list-inside space-y-1">
                    <li>Run <code className="bg-black bg-opacity-30 px-1 rounded text-tokyo-orange">stn sync {result.target_environment}</code></li>
                    <li>Use "Assign Tools" button to complete the copy</li>
                  </ol>
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
                disabled={copying}
              >
                Cancel
              </button>
              <button
                onClick={handleCopy}
                disabled={!selectedTargetEnv || copying}
                className="px-4 py-2 bg-tokyo-orange text-tokyo-bg rounded font-mono text-sm hover:bg-opacity-90 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
              >
                {copying && <Loader className="h-4 w-4 animate-spin" />}
                {copying ? 'Copying...' : 'Copy Environment'}
              </button>
            </>
          ) : (
            <>
              <button
                onClick={handleClose}
                className="px-4 py-2 bg-tokyo-green text-tokyo-bg rounded font-mono text-sm hover:bg-opacity-90"
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
