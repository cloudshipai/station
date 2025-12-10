import React, { useState, useEffect } from 'react';
import { Download, CheckCircle, RefreshCw, ExternalLink, AlertCircle } from 'lucide-react';
import { versionApi, type VersionInfo, type UpdateResult } from '../api/station';

const VersionStatus: React.FC = () => {
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null);
  const [isUpdating, setIsUpdating] = useState(false);
  const [updateResult, setUpdateResult] = useState<UpdateResult | null>(null);
  const [showTooltip, setShowTooltip] = useState(false);
  const [isChecking, setIsChecking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const checkForUpdates = async () => {
    setIsChecking(true);
    setError(null);
    try {
      const response = await versionApi.checkForUpdates();
      setVersionInfo(response.data);
    } catch (err) {
      console.error('Failed to check for updates:', err);
      setError('Failed to check for updates');
    } finally {
      setIsChecking(false);
    }
  };

  useEffect(() => {
    // Check for updates on mount
    checkForUpdates();

    // Poll every 30 minutes
    const interval = setInterval(checkForUpdates, 30 * 60 * 1000);
    return () => clearInterval(interval);
  }, []);

  const handleUpdate = async () => {
    if (isUpdating) return;
    
    setIsUpdating(true);
    setUpdateResult(null);
    
    try {
      const response = await versionApi.performUpdate();
      setUpdateResult(response.data);
      
      if (response.data.success) {
        // Refresh version info after successful update
        setTimeout(() => {
          checkForUpdates();
        }, 2000);
      }
    } catch (err) {
      console.error('Failed to perform update:', err);
      setUpdateResult({
        success: false,
        message: 'Update failed',
        error: 'Network error or server unavailable'
      });
    } finally {
      setIsUpdating(false);
    }
  };

  const getVersionBadgeColor = () => {
    if (error) return 'bg-gray-100 border-gray-300';
    if (versionInfo?.update_available) return 'bg-amber-50 border-amber-200';
    return 'bg-emerald-50 border-emerald-200';
  };

  const getVersionTextColor = () => {
    if (error) return 'text-gray-600';
    if (versionInfo?.update_available) return 'text-amber-700';
    return 'text-emerald-700';
  };

  return (
    <div
      className="relative cursor-pointer"
      onMouseEnter={() => setShowTooltip(true)}
      onMouseLeave={() => setShowTooltip(false)}
    >
      {/* Version Badge */}
      <div className={`flex items-center gap-2 px-3 py-2 rounded-lg border transition-colors ${getVersionBadgeColor()}`}>
        <div className="flex flex-col flex-1 min-w-0">
          <span className="text-xs font-semibold text-gray-700">Station</span>
          <span className={`text-[10px] font-mono ${getVersionTextColor()}`}>
            {isChecking ? (
              <span className="flex items-center gap-1">
                <RefreshCw className="h-2.5 w-2.5 animate-spin" />
                checking...
              </span>
            ) : versionInfo ? (
              versionInfo.current_version
            ) : error ? (
              'error'
            ) : (
              'loading...'
            )}
          </span>
        </div>
        
        {/* Update indicator */}
        {versionInfo?.update_available && !isChecking && (
          <div className="flex items-center gap-1">
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-2 w-2 bg-amber-500"></span>
            </span>
          </div>
        )}
        
        {!versionInfo?.update_available && !isChecking && !error && (
          <CheckCircle className="h-3.5 w-3.5 text-emerald-500" />
        )}
      </div>

      {/* Tooltip */}
      {showTooltip && (
        <div className="absolute bottom-full left-0 mb-2 p-3 bg-white border border-gray-200 rounded-lg shadow-lg z-50 min-w-72">
          <div className="text-xs text-gray-700 space-y-3">
            {/* Header */}
            <div className="flex items-center justify-between pb-2 border-b border-gray-200">
              <div className="flex items-center gap-2">
                <img 
                  src="/station-logo.png" 
                  alt="Station" 
                  className="h-5 w-5 object-contain"
                />
                <span className="font-semibold text-gray-900">Station Version</span>
              </div>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  checkForUpdates();
                }}
                className="p-1 hover:bg-gray-100 rounded transition-colors"
                title="Check for updates"
              >
                <RefreshCw className={`h-3.5 w-3.5 text-gray-500 ${isChecking ? 'animate-spin' : ''}`} />
              </button>
            </div>

            {/* Version Info */}
            {versionInfo && (
              <div className="space-y-2">
                <div className="flex justify-between items-center">
                  <span className="text-gray-500">Current:</span>
                  <span className="font-mono font-medium text-gray-900">{versionInfo.current_version}</span>
                </div>
                
                {versionInfo.update_available && (
                  <>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-500">Latest:</span>
                      <span className="font-mono font-medium text-emerald-600">{versionInfo.latest_version}</span>
                    </div>
                    
                    {versionInfo.release_url && (
                      <a
                        href={versionInfo.release_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-center gap-1 text-blue-600 hover:text-blue-700 hover:underline"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <ExternalLink className="h-3 w-3" />
                        View release notes
                      </a>
                    )}
                  </>
                )}
                
                {!versionInfo.update_available && (
                  <div className="flex items-center gap-1.5 text-emerald-600">
                    <CheckCircle className="h-3.5 w-3.5" />
                    <span>You're on the latest version</span>
                  </div>
                )}
              </div>
            )}

            {/* Update Button */}
            {versionInfo?.update_available && (
              <div className="pt-2 border-t border-gray-200">
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    handleUpdate();
                  }}
                  disabled={isUpdating}
                  className={`w-full flex items-center justify-center gap-2 px-3 py-2 rounded-md text-sm font-medium transition-colors ${
                    isUpdating
                      ? 'bg-gray-100 text-gray-400 cursor-not-allowed'
                      : 'bg-amber-500 text-white hover:bg-amber-600'
                  }`}
                >
                  {isUpdating ? (
                    <>
                      <RefreshCw className="h-4 w-4 animate-spin" />
                      Updating...
                    </>
                  ) : (
                    <>
                      <Download className="h-4 w-4" />
                      Update to {versionInfo.latest_version}
                    </>
                  )}
                </button>
              </div>
            )}

            {/* Update Result */}
            {updateResult && (
              <div className={`p-2 rounded border ${
                updateResult.success 
                  ? 'bg-emerald-50 border-emerald-200 text-emerald-700' 
                  : 'bg-red-50 border-red-200 text-red-700'
              }`}>
                <div className="flex items-start gap-2">
                  {updateResult.success ? (
                    <CheckCircle className="h-4 w-4 mt-0.5 flex-shrink-0" />
                  ) : (
                    <AlertCircle className="h-4 w-4 mt-0.5 flex-shrink-0" />
                  )}
                  <div>
                    <p className="font-medium">{updateResult.message}</p>
                    {updateResult.error && (
                      <p className="text-xs mt-1 opacity-80">{updateResult.error}</p>
                    )}
                  </div>
                </div>
              </div>
            )}

            {/* Error Display */}
            {error && (
              <div className="p-2 rounded border bg-red-50 border-red-200 text-red-700">
                <div className="flex items-center gap-2">
                  <AlertCircle className="h-4 w-4" />
                  <span>{error}</span>
                </div>
              </div>
            )}

            {/* Last Checked */}
            {versionInfo?.checked_at && (
              <div className="text-[10px] text-gray-400 pt-1">
                Last checked: {new Date(versionInfo.checked_at).toLocaleTimeString()}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default VersionStatus;
