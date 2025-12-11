import React, { useState, useEffect } from 'react';
import { Cloud, CheckCircle, XCircle, AlertCircle, Key, X, ExternalLink, RefreshCw } from 'lucide-react';

interface CloudShipAPIStatus {
  authenticated: boolean;
  has_api_key: boolean;
  api_key_masked?: string;
  api_url: string;
  bundle_count: number;
  organization?: string;
  error?: string;
}

const CloudShipStatus: React.FC = () => {
  const [apiStatus, setApiStatus] = useState<CloudShipAPIStatus | null>(null);
  const [showModal, setShowModal] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const fetchStatus = async () => {
    try {
      setIsRefreshing(true);
      const response = await fetch('/api/v1/cloudship/status');
      if (response.ok) {
        const data = await response.json();
        setApiStatus(data);
      }
    } catch (error) {
      console.error('Failed to fetch CloudShip status:', error);
    } finally {
      setIsRefreshing(false);
    }
  };

  useEffect(() => {
    // Initial fetch
    fetchStatus();

    // Poll every 30 seconds
    const interval = setInterval(fetchStatus, 30000);

    return () => {
      clearInterval(interval);
    };
  }, []);

  const isAuthenticated = apiStatus?.authenticated || false;
  const hasAPIKey = apiStatus?.has_api_key || false;

  const getStatusText = () => {
    if (isAuthenticated) return 'Connected';
    if (hasAPIKey && apiStatus?.error) return 'Auth Error';
    if (hasAPIKey) return 'Validating...';
    return 'Not Connected';
  };

  const StatusIcon = isAuthenticated ? CheckCircle : 
                     (hasAPIKey && apiStatus?.error) ? XCircle : 
                     hasAPIKey ? AlertCircle : Cloud;

  return (
    <>
      {/* CloudShip Badge - Click to open modal */}
      <div
        className="relative cursor-pointer"
        onClick={() => setShowModal(true)}
      >
        <div className={`flex items-center space-x-2 px-3 py-2 rounded-lg transition-colors hover:opacity-80 ${
          isAuthenticated 
            ? 'bg-tokyo-green/10 border border-tokyo-green/30' 
            : 'bg-tokyo-dark3 border border-tokyo-dark4'
        }`}>
          <StatusIcon 
            size={16} 
            className={isAuthenticated ? 'text-tokyo-green' : 'text-tokyo-dark5'} 
          />
          <div className="flex flex-col">
            <span className="text-xs font-semibold text-tokyo-fg">CloudShip</span>
            <span className={`text-[10px] ${isAuthenticated ? 'text-tokyo-green' : 'text-tokyo-comment'}`}>
              {getStatusText()}
            </span>
          </div>
          {isAuthenticated && (
            <div className="w-2 h-2 rounded-full bg-tokyo-green animate-pulse ml-auto"></div>
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
            className="bg-tokyo-dark2 border border-tokyo-dark4 rounded-xl shadow-2xl w-full max-w-md mx-4"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Header */}
            <div className="flex items-center justify-between p-4 border-b border-tokyo-dark4">
              <div className="flex items-center space-x-3">
                <Cloud size={24} className="text-tokyo-blue" />
                <div>
                  <h2 className="text-lg font-semibold text-tokyo-fg">CloudShip Status</h2>
                  <p className="text-xs text-tokyo-comment">Bundle registry connection</p>
                </div>
              </div>
              <button
                onClick={() => setShowModal(false)}
                className="p-2 hover:bg-tokyo-dark3 rounded-lg transition-colors"
              >
                <X size={20} className="text-tokyo-comment" />
              </button>
            </div>

            {/* Content */}
            <div className="p-4 space-y-4">
              {/* Connection Status */}
              <div className={`p-4 rounded-lg ${
                isAuthenticated 
                  ? 'bg-tokyo-green/10 border border-tokyo-green/30' 
                  : hasAPIKey && apiStatus?.error 
                    ? 'bg-tokyo-red/10 border border-tokyo-red/30'
                    : 'bg-tokyo-dark3 border border-tokyo-dark4'
              }`}>
                <div className="flex items-center space-x-3">
                  <StatusIcon 
                    size={32} 
                    className={
                      isAuthenticated ? 'text-tokyo-green' : 
                      hasAPIKey && apiStatus?.error ? 'text-tokyo-red' : 
                      'text-tokyo-comment'
                    } 
                  />
                  <div>
                    <p className={`font-semibold ${
                      isAuthenticated ? 'text-tokyo-green' : 
                      hasAPIKey && apiStatus?.error ? 'text-tokyo-red' : 
                      'text-tokyo-comment'
                    }`}>
                      {getStatusText()}
                    </p>
                    <p className="text-xs text-tokyo-comment">
                      {isAuthenticated 
                        ? 'Your Station is connected to CloudShip' 
                        : hasAPIKey 
                          ? 'API key configured but not authenticated'
                          : 'No API key configured'}
                    </p>
                  </div>
                </div>
              </div>

              {/* Details */}
              {apiStatus && (
                <div className="space-y-3">
                  <div className="flex items-center justify-between py-2 border-b border-tokyo-dark4">
                    <span className="text-sm text-tokyo-comment flex items-center space-x-2">
                      <Key size={16} />
                      <span>API Key</span>
                    </span>
                    <span className={`text-sm font-mono ${apiStatus.has_api_key ? 'text-tokyo-fg' : 'text-tokyo-comment'}`}>
                      {apiStatus.has_api_key ? apiStatus.api_key_masked : 'Not configured'}
                    </span>
                  </div>

                  <div className="flex items-center justify-between py-2 border-b border-tokyo-dark4">
                    <span className="text-sm text-tokyo-comment">Authentication</span>
                    <span className={`text-sm flex items-center space-x-1 ${
                      apiStatus.authenticated ? 'text-tokyo-green' : 'text-tokyo-red'
                    }`}>
                      {apiStatus.authenticated ? (
                        <>
                          <CheckCircle size={14} />
                          <span>Valid</span>
                        </>
                      ) : (
                        <>
                          <XCircle size={14} />
                          <span>{apiStatus.has_api_key ? 'Invalid' : 'None'}</span>
                        </>
                      )}
                    </span>
                  </div>

                  {apiStatus.authenticated && apiStatus.bundle_count > 0 && (
                    <div className="flex items-center justify-between py-2 border-b border-tokyo-dark4">
                      <span className="text-sm text-tokyo-comment">Available Bundles</span>
                      <span className="text-sm text-tokyo-purple font-semibold">{apiStatus.bundle_count}</span>
                    </div>
                  )}

                  {apiStatus.organization && (
                    <div className="flex items-center justify-between py-2 border-b border-tokyo-dark4">
                      <span className="text-sm text-tokyo-comment">Organization</span>
                      <span className="text-sm text-tokyo-cyan">{apiStatus.organization}</span>
                    </div>
                  )}

                  <div className="flex items-center justify-between py-2">
                    <span className="text-sm text-tokyo-comment">API URL</span>
                    <span className="text-sm text-tokyo-comment font-mono text-xs">{apiStatus.api_url}</span>
                  </div>
                </div>
              )}

              {/* Error Display */}
              {apiStatus?.error && (
                <div className="p-3 bg-tokyo-red/10 border border-tokyo-red/30 rounded-lg">
                  <p className="text-sm text-tokyo-red">{apiStatus.error}</p>
                </div>
              )}

              {/* Help text when not authenticated */}
              {!isAuthenticated && (
                <div className="p-3 bg-tokyo-dark3 rounded-lg">
                  <p className="text-sm text-tokyo-comment">
                    To connect to CloudShip, run:
                  </p>
                  <code className="block mt-2 p-2 bg-tokyo-dark1 rounded text-sm text-tokyo-cyan font-mono">
                    stn auth login
                  </code>
                </div>
              )}
            </div>

            {/* Footer */}
            <div className="flex items-center justify-between p-4 border-t border-tokyo-dark4">
              <button
                onClick={fetchStatus}
                disabled={isRefreshing}
                className="flex items-center space-x-2 px-3 py-2 text-sm text-tokyo-comment hover:text-tokyo-fg hover:bg-tokyo-dark3 rounded-lg transition-colors disabled:opacity-50"
              >
                <RefreshCw size={16} className={isRefreshing ? 'animate-spin' : ''} />
                <span>Refresh</span>
              </button>
              
              {isAuthenticated && (
                <a
                  href={apiStatus?.api_url || 'https://app.cloudshipai.com'}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center space-x-2 px-4 py-2 bg-tokyo-blue text-white text-sm font-medium rounded-lg hover:bg-tokyo-blue/80 transition-colors"
                >
                  <span>Open CloudShip</span>
                  <ExternalLink size={14} />
                </a>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
};

export default CloudShipStatus;
