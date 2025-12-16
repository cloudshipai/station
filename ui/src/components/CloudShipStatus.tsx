import React, { useState, useEffect } from 'react';
import { Cloud, CheckCircle, XCircle, AlertCircle, Key, X, ExternalLink, RefreshCw, Link } from 'lucide-react';

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
            ? 'bg-emerald-50 border border-emerald-200' 
            : 'bg-gray-100 border border-gray-300'
        }`}>
          <StatusIcon 
            size={16} 
            className={isAuthenticated ? 'text-emerald-600' : 'text-gray-400'} 
          />
          <div className="flex flex-col">
            <span className="text-xs font-semibold text-gray-700">CloudShip</span>
            <span className={`text-[10px] ${isAuthenticated ? 'text-emerald-600' : 'text-gray-500'}`}>
              {getStatusText()}
            </span>
          </div>
          {isAuthenticated && (
            <div className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse ml-auto"></div>
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
                <Cloud size={24} className="text-blue-600" />
                <div>
                  <h2 className="text-lg font-semibold text-gray-900">CloudShip Status</h2>
                  <p className="text-xs text-gray-500">Bundle registry connection</p>
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
              {/* Connection Status */}
              <div className={`p-4 rounded-lg ${
                isAuthenticated 
                  ? 'bg-emerald-50 border border-emerald-200' 
                  : hasAPIKey && apiStatus?.error 
                    ? 'bg-red-50 border border-red-200'
                    : 'bg-gray-50 border border-gray-200'
              }`}>
                <div className="flex items-center space-x-3">
                  <StatusIcon 
                    size={32} 
                    className={
                      isAuthenticated ? 'text-emerald-600' : 
                      hasAPIKey && apiStatus?.error ? 'text-red-600' : 
                      'text-gray-400'
                    } 
                  />
                  <div>
                    <p className={`font-semibold ${
                      isAuthenticated ? 'text-emerald-700' : 
                      hasAPIKey && apiStatus?.error ? 'text-red-700' : 
                      'text-gray-600'
                    }`}>
                      {getStatusText()}
                    </p>
                    <p className="text-xs text-gray-600">
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
                  <div className="flex items-center justify-between py-2 border-b border-gray-200">
                    <span className="text-sm text-gray-500 flex items-center gap-2">
                      <Key size={16} />
                      <span>API Key</span>
                    </span>
                    <span className={`text-sm font-mono ${apiStatus.has_api_key ? 'text-gray-900' : 'text-gray-400'}`}>
                      {apiStatus.has_api_key ? apiStatus.api_key_masked : 'Not configured'}
                    </span>
                  </div>

                  <div className="flex items-center justify-between py-2 border-b border-gray-200">
                    <span className="text-sm text-gray-500">Authentication</span>
                    <span className={`text-sm flex items-center gap-1 ${
                      apiStatus.authenticated ? 'text-emerald-600' : 'text-red-600'
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
                    <div className="flex items-center justify-between py-2 border-b border-gray-200">
                      <span className="text-sm text-gray-500">Available Bundles</span>
                      <span className="text-sm text-purple-600 font-semibold">{apiStatus.bundle_count}</span>
                    </div>
                  )}

                  {apiStatus.organization && (
                    <div className="flex items-center justify-between py-2 border-b border-gray-200">
                      <span className="text-sm text-gray-500">Organization</span>
                      <span className="text-sm text-blue-600 font-medium">{apiStatus.organization}</span>
                    </div>
                  )}

                  <div className="flex items-center justify-between py-2">
                    <span className="text-sm text-gray-500 flex items-center gap-2">
                      <Link size={16} />
                      <span>API URL</span>
                    </span>
                    <span className="text-sm text-gray-600 font-mono text-xs">{apiStatus.api_url}</span>
                  </div>
                </div>
              )}

              {/* Error Display */}
              {apiStatus?.error && (
                <div className="p-3 bg-red-50 border border-red-200 rounded-lg">
                  <p className="text-sm text-red-700">{apiStatus.error}</p>
                </div>
              )}

              {/* Help text when not authenticated */}
              {!isAuthenticated && (
                <div className="p-3 bg-gray-50 border border-gray-200 rounded-lg">
                  <p className="text-sm text-gray-600 mb-2">
                    To connect to CloudShip, run:
                  </p>
                  <code className="block p-2 bg-gray-900 rounded text-sm text-green-400 font-mono">
                    stn auth login
                  </code>
                </div>
              )}
            </div>

            {/* Footer */}
            <div className="flex items-center justify-between p-4 border-t border-gray-200">
              <button
                onClick={fetchStatus}
                disabled={isRefreshing}
                className="flex items-center gap-2 px-3 py-2 text-sm text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors disabled:opacity-50"
              >
                <RefreshCw size={16} className={isRefreshing ? 'animate-spin' : ''} />
                <span>Refresh</span>
              </button>
              
              {isAuthenticated && (
                <a
                  href={apiStatus?.api_url || 'https://app.cloudshipai.com'}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 transition-colors"
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
