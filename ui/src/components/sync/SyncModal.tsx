import React, { useState, useEffect, useCallback, useRef } from 'react';
import { X, Play, CheckCircle, AlertCircle, Clock, Database, Loader2 } from 'lucide-react';
import { syncApi } from '../../api/station';

interface SyncModalProps {
  isOpen: boolean;
  onClose: () => void;
  environment: string;
  onSyncComplete?: () => void;
  autoStart?: boolean;
}

interface SyncStatus {
  id: string;
  status: 'running' | 'waiting_for_input' | 'completed' | 'failed';
  environment: string;
  progress: {
    current_step: string;
    steps_total: number;
    steps_complete: number;
    message: string;
  };
  variables?: {
    config_name: string;
    variables: Array<{
      name: string;
      description: string;
      required: boolean;
      secret: boolean;
      default?: string;
    }>;
    message: string;
  };
  result?: any;
  error?: string;
  created_at: string;
  updated_at: string;
}

export const SyncModal: React.FC<SyncModalProps> = ({ isOpen, onClose, environment, onSyncComplete, autoStart = false }) => {
  const [syncStatus, setSyncStatus] = useState<SyncStatus | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isSubmittingVariables, setIsSubmittingVariables] = useState(false);
  const [variables, setVariables] = useState<Record<string, string>>({});
  const [pollInterval, setPollInterval] = useState<NodeJS.Timeout | null>(null);
  const [variablesInitialized, setVariablesInitialized] = useState(false);
  const [validationErrors, setValidationErrors] = useState<Record<string, string>>({});
  const inputRefs = useRef<Record<string, HTMLInputElement>>({});
  const lastAutoStartKey = useRef<string>('');

  // Auto-start sync when modal opens if autoStart is true
  // Use a composite key of isOpen + environment to prevent re-triggering for the same environment
  useEffect(() => {
    const autoStartKey = `${isOpen}-${environment}`;

    if (isOpen && autoStart && environment && lastAutoStartKey.current !== autoStartKey) {
      console.log('[SyncModal] Auto-starting sync for environment:', environment);
      lastAutoStartKey.current = autoStartKey;
      startSync();
    }
  }, [isOpen, autoStart, environment]);

  // Clean up polling on unmount or modal close
  useEffect(() => {
    if (!isOpen) {
      if (pollInterval) {
        clearInterval(pollInterval);
        setPollInterval(null);
      }
      setSyncStatus(null);
      setVariables({});
      setVariablesInitialized(false);
      setValidationErrors({});
      setIsSubmittingVariables(false);
      lastAutoStartKey.current = '';
      inputRefs.current = {};
    }
  }, [isOpen, pollInterval]);

  const startSync = async () => {
    setIsLoading(true);
    try {
      console.log('[SyncModal] Starting sync for environment:', environment);
      const response = await syncApi.startInteractive(environment);
      const { sync_id, status } = response.data;
      console.log('[SyncModal] Sync started with ID:', sync_id, 'status:', status);
      setSyncStatus(status);

      // Start polling for status updates
      const interval = setInterval(async () => {
        try {
          const statusResponse = await syncApi.getStatus(sync_id);
          const newStatus = statusResponse.data;
          console.log('[SyncModal] Poll result - status:', newStatus.status, 'progress:', newStatus.progress);

          // Only update state if status actually changed to avoid unnecessary re-renders
          setSyncStatus(prevStatus => {
            if (!prevStatus ||
                prevStatus.status !== newStatus.status ||
                prevStatus.updated_at !== newStatus.updated_at ||
                JSON.stringify(prevStatus.variables) !== JSON.stringify(newStatus.variables)) {
              return newStatus;
            }
            return prevStatus;
          });

          // Clear submitting state when status changes away from waiting_for_input
          if (newStatus.status !== 'waiting_for_input' && isSubmittingVariables) {
            setIsSubmittingVariables(false);
          }

          // Initialize variables form when waiting for input (only once)
          if (newStatus.status === 'waiting_for_input' && newStatus.variables && !variablesInitialized) {
            const initialVars: Record<string, string> = {};
            newStatus.variables.variables.forEach(variable => {
              if (variable.default) {
                initialVars[variable.name] = variable.default;
                // Set default value in input ref if available
                if (inputRefs.current[variable.name]) {
                  inputRefs.current[variable.name].value = variable.default;
                }
              }
            });
            setVariables(initialVars);
            setVariablesInitialized(true);
          }

          // Stop polling when sync is complete or failed
          if (newStatus.status === 'completed' || newStatus.status === 'failed') {
            console.log('[SyncModal] Sync finished with status:', newStatus.status);
            clearInterval(interval);
            setPollInterval(null);

            // Call onSyncComplete callback when sync completes successfully
            if (newStatus.status === 'completed' && onSyncComplete) {
              onSyncComplete();
            }
          }
        } catch (error: any) {
          // Handle 404 errors gracefully - sync might have been cleaned up
          if (error.response?.status === 404) {
            console.log('[SyncModal] Sync operation not found (404), assuming completed and stopping polling');
            clearInterval(interval);
            setPollInterval(null);
            // Mark as completed since backend cleaned it up (means it finished)
            setSyncStatus(prev => {
              if (prev) {
                return {
                  ...prev,
                  status: 'completed',
                  progress: {
                    ...prev.progress,
                    steps_complete: prev.progress.steps_total,
                    current_step: 'Completed',
                    message: 'Sync completed successfully'
                  }
                };
              }
              return prev;
            });
            // Call onSyncComplete callback since sync completed
            if (onSyncComplete) {
              onSyncComplete();
            }
          } else {
            console.error('[SyncModal] Failed to poll sync status:', error);
          }
        }
      }, 1000);

      setPollInterval(interval);
    } catch (error) {
      console.error('[SyncModal] Failed to start sync:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const submitVariables = async () => {
    if (!syncStatus || isSubmittingVariables) return;

    // Validate required fields first (before setting loading state)
    const errors: Record<string, string> = {};
    const currentVariables: Record<string, string> = {};

    if (syncStatus.variables) {
      syncStatus.variables.variables.forEach(variable => {
        const input = inputRefs.current[variable.name];
        const value = input?.value?.trim() || '';

        if (variable.required && !value) {
          errors[variable.name] = `${variable.name} is required`;
        }

        currentVariables[variable.name] = value;
      });
    }

    setValidationErrors(errors);

    // Don't proceed if there are validation errors
    if (Object.keys(errors).length > 0) {
      return;
    }

    // Only set loading state after validation passes
    setIsSubmittingVariables(true);

    try {
      await syncApi.submitVariables(syncStatus.id, currentVariables);
      // Keep loading state on for 500ms to ensure user sees the spinner
      // The polling will pick up the status change and hide the form
      setValidationErrors({}); // Clear any previous errors

      // Don't clear isSubmittingVariables immediately - let the status change from polling clear it
      // This prevents the form from flashing before the status updates
    } catch (error) {
      console.error('Failed to submit variables:', error);
      setIsSubmittingVariables(false);
    }
  };

  // No longer needed - using uncontrolled inputs

  const handleClose = () => {
    if (pollInterval) {
      clearInterval(pollInterval);
      setPollInterval(null);
    }
    onClose();
  };

  const getStatusIcon = () => {
    if (!syncStatus) return <Database className="h-5 w-5 text-tokyo-cyan" />;
    
    switch (syncStatus.status) {
      case 'running':
        return <Clock className="h-5 w-5 text-tokyo-blue animate-spin" />;
      case 'waiting_for_input':
        return <AlertCircle className="h-5 w-5 text-tokyo-yellow" />;
      case 'completed':
        return <CheckCircle className="h-5 w-5 text-tokyo-green" />;
      case 'failed':
        return <AlertCircle className="h-5 w-5 text-tokyo-red" />;
      default:
        return <Database className="h-5 w-5 text-tokyo-cyan" />;
    }
  };

  const getStatusColor = () => {
    if (!syncStatus) return 'text-tokyo-cyan';
    
    switch (syncStatus.status) {
      case 'running':
        return 'text-tokyo-blue';
      case 'waiting_for_input':
        return 'text-tokyo-yellow';
      case 'completed':
        return 'text-tokyo-green';
      case 'failed':
        return 'text-tokyo-red';
      default:
        return 'text-tokyo-cyan';
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 max-w-2xl w-full mx-4 max-h-[80vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            {getStatusIcon()}
            <h2 className="text-xl font-mono font-semibold text-tokyo-cyan">
              Sync Environment: {environment}
            </h2>
          </div>
          <button 
            onClick={handleClose}
            className="text-tokyo-comment hover:text-tokyo-fg transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content */}
        <div className="space-y-6">
          {!syncStatus ? (
            /* Initial State */
            <div className="text-center py-8">
              <Database className="h-16 w-16 text-tokyo-cyan mx-auto mb-4" />
              <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-2">
                Ready to Sync
              </h3>
              <p className="text-tokyo-comment font-mono text-sm mb-6">
                This will sync all MCP server configurations for the {environment} environment
              </p>
              <button
                onClick={startSync}
                disabled={isLoading}
                className="px-6 py-3 bg-tokyo-cyan text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-blue transition-colors disabled:opacity-50 flex items-center gap-2 mx-auto"
              >
                {isLoading ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Starting...
                  </>
                ) : (
                  <>
                    <Play className="h-4 w-4" />
                    Start Sync
                  </>
                )}
              </button>
            </div>
          ) : (
            <>
              {/* Progress Indicator */}
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <span className={`font-mono font-medium ${getStatusColor()}`}>
                    {syncStatus.status === 'running' && 'Syncing...'}
                    {syncStatus.status === 'waiting_for_input' && 'Waiting for Input'}
                    {syncStatus.status === 'completed' && 'Completed'}
                    {syncStatus.status === 'failed' && 'Failed'}
                  </span>
                  <span className="text-tokyo-comment font-mono text-sm">
                    {syncStatus.progress.steps_complete} / {syncStatus.progress.steps_total}
                  </span>
                </div>
                
                {/* Progress Bar */}
                <div className="w-full bg-tokyo-bg border border-tokyo-blue7 rounded-full h-2">
                  <div 
                    className="bg-tokyo-cyan h-2 rounded-full transition-all duration-300"
                    style={{ 
                      width: `${(syncStatus.progress.steps_complete / syncStatus.progress.steps_total) * 100}%` 
                    }}
                  />
                </div>
                
                <div className="text-sm text-tokyo-comment font-mono">
                  <div className="font-medium">{syncStatus.progress.current_step}</div>
                  <div>{syncStatus.progress.message}</div>
                </div>
              </div>

              {/* Variable Input Form */}
              {syncStatus.status === 'waiting_for_input' && syncStatus.variables && (
                <div className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4 space-y-4">
                  <div className="flex items-center gap-2 mb-3">
                    <AlertCircle className="h-5 w-5 text-tokyo-yellow" />
                    <h3 className="text-lg font-mono font-medium text-tokyo-yellow">
                      Configuration Variables Required
                    </h3>
                  </div>
                  
                  <div className="text-sm text-tokyo-comment font-mono mb-4">
                    Config: <span className="text-tokyo-cyan">{syncStatus.variables.config_name}</span>
                  </div>
                  
                  {syncStatus.variables.message && (
                    <div className="text-sm text-tokyo-comment font-mono mb-4 p-3 bg-tokyo-bg-highlight rounded border border-tokyo-blue7">
                      {syncStatus.variables.message}
                    </div>
                  )}

                  <div className="space-y-3">
                    {syncStatus.variables.variables.map((variable) => (
                      <div key={variable.name} className="space-y-2">
                        <label className="block text-sm font-mono text-tokyo-cyan font-medium">
                          {variable.name}
                          {variable.required && <span className="text-tokyo-red ml-1">*</span>}
                        </label>
                        {variable.description && (
                          <p className="text-xs text-tokyo-comment font-mono">
                            {variable.description}
                          </p>
                        )}
                        <input
                          key={variable.name}
                          ref={(el) => {
                            if (el) inputRefs.current[variable.name] = el;
                          }}
                          type={variable.secret ? 'password' : 'text'}
                          defaultValue={variable.default || ''}
                          placeholder={variable.default || `Enter ${variable.name}...`}
                          className={`w-full px-3 py-2 bg-tokyo-bg border rounded font-mono text-tokyo-fg focus:outline-none text-sm ${
                            validationErrors[variable.name] 
                              ? 'border-tokyo-red focus:border-tokyo-red' 
                              : 'border-tokyo-blue7 focus:border-tokyo-cyan'
                          }`}
                          autoComplete="off"
                        />
                        {validationErrors[variable.name] && (
                          <p className="text-xs text-tokyo-red font-mono mt-1">
                            {validationErrors[variable.name]}
                          </p>
                        )}
                      </div>
                    ))}
                  </div>

                  <button
                    onClick={submitVariables}
                    disabled={isSubmittingVariables || Object.keys(validationErrors).length > 0}
                    className={`w-full px-4 py-2 rounded font-mono font-medium transition-colors flex items-center justify-center gap-2 ${
                      isSubmittingVariables || Object.keys(validationErrors).length > 0
                        ? 'bg-tokyo-comment text-tokyo-bg cursor-not-allowed'
                        : 'bg-tokyo-cyan text-tokyo-bg hover:bg-tokyo-blue'
                    }`}
                  >
                    {isSubmittingVariables ? (
                      <>
                        <Loader2 className="h-4 w-4 animate-spin" />
                        Submitting...
                      </>
                    ) : (
                      'Continue Sync'
                    )}
                  </button>
                </div>
              )}

              {/* Results */}
              {syncStatus.status === 'completed' && (
                <div className="bg-green-900 bg-opacity-30 border border-green-500 border-opacity-50 rounded p-4">
                  <div className="flex items-center gap-2 mb-3">
                    <CheckCircle className="h-5 w-5 text-tokyo-green" />
                    <h3 className="text-lg font-mono font-medium text-tokyo-green">
                      Sync Completed Successfully
                    </h3>
                  </div>
                  
                  <p className="text-tokyo-green font-mono text-sm mb-3">
                    Environment '{environment}' has been synchronized successfully. 
                    {syncStatus.variables ? ' All required variables have been configured.' : ''}
                  </p>
                  
                  {syncStatus.result && (
                    <div className="space-y-2 text-sm font-mono">
                      <div className="text-tokyo-comment">
                        📊 <strong>Sync Results:</strong>
                      </div>
                      <div className="ml-4 space-y-1">
                        <div className="text-tokyo-cyan">
                          • MCP Servers: {syncStatus.result.MCPServersProcessed} processed, {syncStatus.result.MCPServersConnected} connected
                        </div>
                        <div className="text-tokyo-green">
                          • Agents: {syncStatus.result.AgentsProcessed} processed, {syncStatus.result.AgentsSynced} synced
                        </div>
                        {syncStatus.result.AgentsSkipped > 0 && (
                          <div className="text-tokyo-comment">
                            • {syncStatus.result.AgentsSkipped} agents up-to-date (skipped)
                          </div>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              )}

              {/* Error */}
              {syncStatus.status === 'failed' && (
                <div className="bg-red-900 bg-opacity-30 border border-red-500 border-opacity-50 rounded p-4">
                  <div className="flex items-center gap-2 mb-3">
                    <AlertCircle className="h-5 w-5 text-tokyo-red" />
                    <h3 className="text-lg font-mono font-medium text-tokyo-red">
                      Sync Failed
                    </h3>
                  </div>
                  
                  {syncStatus.error && (
                    <div className="text-sm font-mono text-tokyo-red">
                      {syncStatus.error}
                    </div>
                  )}
                </div>
              )}
            </>
          )}
        </div>

        {/* Footer */}
        {syncStatus && (syncStatus.status === 'completed' || syncStatus.status === 'failed') && (
          <div className="flex justify-end mt-6">
            <button
              onClick={handleClose}
              className="px-4 py-2 bg-tokyo-bg border border-tokyo-blue7 text-tokyo-fg rounded font-mono hover:bg-tokyo-bg-highlight transition-colors"
            >
              Close
            </button>
          </div>
        )}
      </div>
    </div>
  );
};