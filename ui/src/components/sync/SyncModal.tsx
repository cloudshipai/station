import React, { useState, useEffect, useCallback, useRef } from 'react';
import { X, Play, CheckCircle, AlertCircle, Clock, Database, Loader2 } from 'lucide-react';
import { syncApi } from '../../api/station';

interface SyncModalProps {
  isOpen: boolean;
  onClose: () => void;
  environment: string;
  onSyncComplete?: () => void;
  onSyncFailed?: () => void;
  autoStart?: boolean;
  syncId?: string;
  standalone?: boolean;
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

export const SyncModal: React.FC<SyncModalProps> = ({ 
  isOpen, 
  onClose, 
  environment, 
  onSyncComplete, 
  onSyncFailed,
  autoStart = false,
  syncId: externalSyncId,
  standalone = false
}) => {
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
    const autoStartKey = `${isOpen}-${environment}-${externalSyncId || 'new'}`;

    if (isOpen && autoStart && environment && lastAutoStartKey.current !== autoStartKey) {
      console.log('[SyncModal] Auto-starting sync for environment:', environment, 'externalSyncId:', externalSyncId);
      lastAutoStartKey.current = autoStartKey;
      
      if (externalSyncId) {
        // Attach to existing sync session - just start polling
        attachToExistingSync(externalSyncId);
      } else {
        // Start a new sync
        startSync();
      }
    }
  }, [isOpen, autoStart, environment, externalSyncId]);

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

  const startPolling = (syncId: string) => {
    const interval = setInterval(async () => {
      try {
        const statusResponse = await syncApi.getStatus(syncId);
        const newStatus = statusResponse.data;
        console.log('[SyncModal] Poll result - status:', newStatus.status, 'progress:', newStatus.progress);

        setSyncStatus(prevStatus => {
          if (!prevStatus ||
              prevStatus.status !== newStatus.status ||
              prevStatus.updated_at !== newStatus.updated_at ||
              JSON.stringify(prevStatus.variables) !== JSON.stringify(newStatus.variables)) {
            return newStatus;
          }
          return prevStatus;
        });

        if (newStatus.status !== 'waiting_for_input' && isSubmittingVariables) {
          setIsSubmittingVariables(false);
        }

        if (newStatus.status === 'waiting_for_input' && newStatus.variables && !variablesInitialized) {
          const initialVars: Record<string, string> = {};
          newStatus.variables.variables.forEach(variable => {
            if (variable.default) {
              initialVars[variable.name] = variable.default;
              if (inputRefs.current[variable.name]) {
                inputRefs.current[variable.name].value = variable.default;
              }
            }
          });
          setVariables(initialVars);
          setVariablesInitialized(true);
        }

        if (newStatus.status === 'completed' || newStatus.status === 'failed') {
          console.log('[SyncModal] Sync finished with status:', newStatus.status);
          clearInterval(interval);
          setPollInterval(null);

          if (newStatus.status === 'completed' && onSyncComplete) {
            onSyncComplete();
          }
          if (newStatus.status === 'failed' && onSyncFailed) {
            onSyncFailed();
          }
        }
      } catch (error: any) {
        if (error.response?.status === 404) {
          console.log('[SyncModal] Sync operation not found (404), assuming completed and stopping polling');
          clearInterval(interval);
          setPollInterval(null);
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
          if (onSyncComplete) {
            onSyncComplete();
          }
        } else {
          console.error('[SyncModal] Failed to poll sync status:', error);
        }
      }
    }, 1000);

    setPollInterval(interval);
    return interval;
  };

  const attachToExistingSync = async (syncId: string) => {
    setIsLoading(true);
    try {
      console.log('[SyncModal] Attaching to existing sync:', syncId);
      const statusResponse = await syncApi.getStatus(syncId);
      setSyncStatus(statusResponse.data);
      startPolling(syncId);
    } catch (error) {
      console.error('[SyncModal] Failed to attach to sync:', error);
      onSyncFailed?.();
    } finally {
      setIsLoading(false);
    }
  };

  const startSync = async () => {
    setIsLoading(true);
    try {
      console.log('[SyncModal] Starting sync for environment:', environment);
      const response = await syncApi.startInteractive(environment);
      const { sync_id, status } = response.data;
      console.log('[SyncModal] Sync started with ID:', sync_id, 'status:', status);
      setSyncStatus(status);

      startPolling(sync_id);
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
    if (!syncStatus) return <Database className="h-6 w-6 text-cyan-600" />;
    
    switch (syncStatus.status) {
      case 'running':
        return <Clock className="h-6 w-6 text-blue-600 animate-spin" />;
      case 'waiting_for_input':
        return <AlertCircle className="h-6 w-6 text-yellow-600" />;
      case 'completed':
        return <CheckCircle className="h-6 w-6 text-green-600" />;
      case 'failed':
        return <AlertCircle className="h-6 w-6 text-red-600" />;
      default:
        return <Database className="h-6 w-6 text-cyan-600" />;
    }
  };

  const getStatusColor = () => {
    if (!syncStatus) return 'text-cyan-600';
    
    switch (syncStatus.status) {
      case 'running':
        return 'text-blue-600';
      case 'waiting_for_input':
        return 'text-yellow-600';
      case 'completed':
        return 'text-green-600';
      case 'failed':
        return 'text-red-600';
      default:
        return 'text-cyan-600';
    }
  };

  if (!isOpen) return null;

  const content = (
    <div 
      className={standalone 
        ? "bg-white border border-gray-200 rounded-lg p-6 w-full shadow-lg" 
        : "bg-white border border-gray-200 rounded-lg p-6 max-w-2xl w-full mx-4 max-h-[80vh] overflow-y-auto shadow-lg animate-in zoom-in-95 fade-in slide-in-from-bottom-4 duration-300"
      }
      onClick={(e) => e.stopPropagation()}
    >
      {!standalone && (
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            {getStatusIcon()}
            <div>
              <h2 className="text-xl font-semibold text-gray-900">
                Sync Environment
              </h2>
              <p className="text-sm text-gray-600">{environment}</p>
            </div>
          </div>
          <button 
            onClick={handleClose}
            className="p-2 hover:bg-gray-100 rounded transition-colors"
          >
            <X className="h-5 w-5 text-gray-500 hover:text-gray-900" />
          </button>
        </div>
      )}

        {/* Content */}
        <div className="space-y-6">
          {!syncStatus ? (
            /* Initial State */
            <div className="text-center py-8">
              <Database className="h-16 w-16 text-cyan-600 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">
                Ready to Sync
              </h3>
              <p className="text-gray-600 text-sm mb-6">
                This will sync all MCP server configurations for the {environment} environment
              </p>
              <button
                onClick={startSync}
                disabled={isLoading}
                className="px-6 py-3 bg-cyan-600 text-white rounded hover:bg-cyan-700 transition-colors disabled:opacity-50 flex items-center gap-2 mx-auto shadow-sm"
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
                  <span className={`font-semibold ${getStatusColor()}`}>
                    {syncStatus.status === 'running' && 'Syncing...'}
                    {syncStatus.status === 'waiting_for_input' && 'Waiting for Input'}
                    {syncStatus.status === 'completed' && 'Completed'}
                    {syncStatus.status === 'failed' && 'Failed'}
                  </span>
                  <span className="text-gray-600 text-sm">
                    {syncStatus.progress.steps_complete} / {syncStatus.progress.steps_total}
                  </span>
                </div>
                
                {/* Progress Bar */}
                <div className="w-full bg-gray-200 rounded-full h-2">
                  <div 
                    className="bg-cyan-600 h-2 rounded-full transition-all duration-300"
                    style={{ 
                      width: `${(syncStatus.progress.steps_complete / syncStatus.progress.steps_total) * 100}%` 
                    }}
                  />
                </div>
                
                <div className="text-sm text-gray-600">
                  <div className="font-medium text-gray-900">{syncStatus.progress.current_step}</div>
                  <div>{syncStatus.progress.message}</div>
                </div>
              </div>

              {/* Variable Input Form */}
              {syncStatus.status === 'waiting_for_input' && syncStatus.variables && (
                <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-5 space-y-4">
                  <div className="flex items-center gap-2 mb-3">
                    <AlertCircle className="h-5 w-5 text-yellow-600" />
                    <h3 className="text-lg font-semibold text-yellow-900">
                      Configuration Variables Required
                    </h3>
                  </div>
                  
                  <div className="text-sm text-gray-700 mb-4">
                    Config: <span className="font-semibold text-cyan-700">{syncStatus.variables.config_name}</span>
                  </div>
                  
                  {syncStatus.variables.message && (
                    <div className="text-sm text-gray-700 mb-4 p-3 bg-white rounded border border-yellow-200">
                      {syncStatus.variables.message}
                    </div>
                  )}

                  <div className="space-y-4">
                    {syncStatus.variables.variables.map((variable) => (
                      <div key={variable.name} className="space-y-1.5">
                        <label className="block text-sm font-semibold text-gray-900">
                          {variable.name}
                          {variable.required && <span className="text-red-600 ml-1">*</span>}
                        </label>
                        {variable.description && (
                          <p className="text-xs text-gray-600">
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
                          className={`w-full px-3 py-2 bg-white border rounded text-gray-900 focus:outline-none focus:ring-2 text-sm ${
                            validationErrors[variable.name] 
                              ? 'border-red-300 focus:border-red-500 focus:ring-red-200' 
                              : 'border-gray-300 focus:border-cyan-500 focus:ring-cyan-200'
                          }`}
                          autoComplete="off"
                        />
                        {validationErrors[variable.name] && (
                          <p className="text-xs text-red-600 mt-1">
                            {validationErrors[variable.name]}
                          </p>
                        )}
                      </div>
                    ))}
                  </div>

                  <button
                    onClick={submitVariables}
                    disabled={isSubmittingVariables || Object.keys(validationErrors).length > 0}
                    className={`w-full px-4 py-2.5 rounded font-medium transition-colors flex items-center justify-center gap-2 shadow-sm ${
                      isSubmittingVariables || Object.keys(validationErrors).length > 0
                        ? 'bg-gray-300 text-gray-500 cursor-not-allowed'
                        : 'bg-cyan-600 text-white hover:bg-cyan-700'
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
                <div className="bg-green-50 border border-green-200 rounded-lg p-4">
                  <div className="flex items-center gap-2 mb-3">
                    <CheckCircle className="h-5 w-5 text-green-600" />
                    <h3 className="text-lg font-semibold text-green-900">
                      Sync Completed Successfully
                    </h3>
                  </div>
                  
                  <p className="text-green-800 text-sm mb-3">
                    Environment '<span className="font-semibold">{environment}</span>' has been synchronized successfully. 
                    {syncStatus.variables ? ' All required variables have been configured.' : ''}
                  </p>
                  
                  {syncStatus.result && (
                    <div className="space-y-2 text-sm">
                      <div className="text-gray-700 font-semibold">
                        Sync Results:
                      </div>
                      <div className="ml-4 space-y-1 text-gray-800">
                        <div>
                          • MCP Servers: <span className="font-semibold">{syncStatus.result.MCPServersProcessed}</span> processed, <span className="font-semibold">{syncStatus.result.MCPServersConnected}</span> connected
                        </div>
                        <div>
                          • Agents: <span className="font-semibold">{syncStatus.result.AgentsProcessed}</span> processed, <span className="font-semibold">{syncStatus.result.AgentsSynced}</span> synced
                        </div>
                        {syncStatus.result.AgentsSkipped > 0 && (
                          <div className="text-gray-600">
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
                <div className="bg-red-50 border border-red-200 rounded-lg p-4">
                  <div className="flex items-center gap-2 mb-3">
                    <AlertCircle className="h-5 w-5 text-red-600" />
                    <h3 className="text-lg font-semibold text-red-900">
                      Sync Failed
                    </h3>
                  </div>
                  
                  {syncStatus.error && (
                    <div className="text-sm text-red-800">
                      {syncStatus.error}
                    </div>
                  )}
                </div>
              )}
            </>
          )}
        </div>

        {!standalone && syncStatus && (syncStatus.status === 'completed' || syncStatus.status === 'failed') && (
          <div className="flex justify-end mt-6">
            <button
              onClick={handleClose}
              className="px-4 py-2 bg-white border border-gray-300 text-gray-700 rounded hover:bg-gray-50 transition-colors shadow-sm"
            >
              Close
            </button>
          </div>
        )}
      </div>
  );

  if (standalone) {
    return content;
  }

  return (
    <div 
      className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 animate-in fade-in duration-200"
      onClick={onClose}
    >
      {content}
    </div>
  );
};