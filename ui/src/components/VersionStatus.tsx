import React, { useState, useEffect } from 'react';
import { Download, CheckCircle, RefreshCw, ExternalLink, AlertCircle, X, Rocket } from 'lucide-react';
import { versionApi, type VersionInfo, type UpdateResult } from '../api/station';

const VersionStatus: React.FC = () => {
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null);
  const [isUpdating, setIsUpdating] = useState(false);
  const [updateResult, setUpdateResult] = useState<UpdateResult | null>(null);
  const [showModal, setShowModal] = useState(false);
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
    <>
      {/* Version Badge - Click to open modal */}
      <div
        className="relative cursor-pointer"
        onClick={() => setShowModal(true)}
      >
        <div className={`flex items-center gap-2 px-3 py-2 rounded-lg border transition-colors hover:opacity-80 ${getVersionBadgeColor()}`}>
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
      </div>

      {/* Modal */}
      {showModal && (
        <div 
          className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
          onClick={() => setShowModal(false)}
        >
          <div 
            className="bg-white border border-gray-200 rounded-xl shadow-2xl w-full max-w-md mx-4"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Header */}
            <div className="flex items-center justify-between p-4 border-b border-gray-200">
              <div className="flex items-center space-x-3">
                <Rocket size={24} className="text-blue-600" />
                <div>
                  <h2 className="text-lg font-semibold text-gray-900">Station Version</h2>
                  <p className="text-xs text-gray-500">Check for updates</p>
                </div>
              </div>
              <button
                onClick={() => setShowModal(false)}
                className="p-2 hover:bg-gray-100 rounded-lg transition-colors"
              >
                <X size={20} className="text-gray-500" />
              </button>
            </div>

            {/* Content */}
            <div className="p-4 space-y-4">
              {/* Version Status Card */}
              <div className={`p-4 rounded-lg ${
                versionInfo?.update_available 
                  ? 'bg-amber-50 border border-amber-200' 
                  : error 
                    ? 'bg-red-50 border border-red-200'
                    : 'bg-emerald-50 border border-emerald-200'
              }`}>
                <div className="flex items-center space-x-3">
                  {versionInfo?.update_available ? (
                    <Download size={32} className="text-amber-600" />
                  ) : error ? (
                    <AlertCircle size={32} className="text-red-600" />
                  ) : (
                    <CheckCircle size={32} className="text-emerald-600" />
                  )}
                  <div>
                    <p className={`font-semibold ${
                      versionInfo?.update_available ? 'text-amber-700' : 
                      error ? 'text-red-700' : 
                      'text-emerald-700'
                    }`}>
                      {versionInfo?.update_available 
                        ? 'Update Available' 
                        : error 
                          ? 'Error'
                          : 'Up to Date'}
                    </p>
                    <p className="text-xs text-gray-600">
                      {versionInfo?.update_available 
                        ? `Version ${versionInfo.latest_version} is available` 
                        : error
                          ? error
                          : "You're running the latest version"}
                    </p>
                  </div>
                </div>
              </div>

              {/* Version Details */}
              {versionInfo && (
                <div className="space-y-3">
                  <div className="flex items-center justify-between py-2 border-b border-gray-200">
                    <span className="text-sm text-gray-500">Current Version</span>
                    <span className="text-sm font-mono font-medium text-gray-900">{versionInfo.current_version}</span>
                  </div>

                  {versionInfo.update_available && (
                    <div className="flex items-center justify-between py-2 border-b border-gray-200">
                      <span className="text-sm text-gray-500">Latest Version</span>
                      <span className="text-sm font-mono font-medium text-emerald-600">{versionInfo.latest_version}</span>
                    </div>
                  )}

                  {versionInfo.checked_at && (
                    <div className="flex items-center justify-between py-2">
                      <span className="text-sm text-gray-500">Last Checked</span>
                      <span className="text-sm text-gray-600">{new Date(versionInfo.checked_at).toLocaleTimeString()}</span>
                    </div>
                  )}
                </div>
              )}

              {/* Update Result */}
              {updateResult && (
                <div className={`p-3 rounded-lg ${
                  updateResult.success 
                    ? 'bg-emerald-50 border border-emerald-200' 
                    : 'bg-red-50 border border-red-200'
                }`}>
                  <div className="flex items-start gap-2">
                    {updateResult.success ? (
                      <CheckCircle className="h-5 w-5 text-emerald-600 mt-0.5 flex-shrink-0" />
                    ) : (
                      <AlertCircle className="h-5 w-5 text-red-600 mt-0.5 flex-shrink-0" />
                    )}
                    <div>
                      <p className={`font-medium text-sm ${updateResult.success ? 'text-emerald-700' : 'text-red-700'}`}>
                        {updateResult.message}
                      </p>
                      {updateResult.error && (
                        <p className="text-xs text-red-600 mt-1">{updateResult.error}</p>
                      )}
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* Footer */}
            <div className="flex items-center justify-between p-4 border-t border-gray-200">
              <button
                onClick={checkForUpdates}
                disabled={isChecking}
                className="flex items-center space-x-2 px-3 py-2 text-sm text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors disabled:opacity-50"
              >
                <RefreshCw size={16} className={isChecking ? 'animate-spin' : ''} />
                <span>Check Now</span>
              </button>
              
              <div className="flex items-center gap-2">
                {versionInfo?.release_url && (
                  <a
                    href={versionInfo.release_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center space-x-2 px-3 py-2 text-sm text-blue-600 hover:text-blue-700 hover:bg-blue-50 rounded-lg transition-colors"
                  >
                    <ExternalLink size={14} />
                    <span>View release notes</span>
                  </a>
                )}
                
                {versionInfo?.update_available && (
                  <button
                    onClick={handleUpdate}
                    disabled={isUpdating}
                    className={`flex items-center space-x-2 px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
                      isUpdating
                        ? 'bg-gray-200 text-gray-400 cursor-not-allowed'
                        : 'bg-amber-500 text-white hover:bg-amber-600'
                    }`}
                  >
                    {isUpdating ? (
                      <>
                        <RefreshCw className="h-4 w-4 animate-spin" />
                        <span>Updating...</span>
                      </>
                    ) : (
                      <>
                        <Download className="h-4 w-4" />
                        <span>Update to {versionInfo.latest_version}</span>
                      </>
                    )}
                  </button>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </>
  );
};

export default VersionStatus;
