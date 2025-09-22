import React, { useState } from 'react';
import { X, Package, Copy } from 'lucide-react';
import { bundlesApi } from '../../api/station';

interface BundleEnvironmentModalProps {
  isOpen: boolean;
  onClose: () => void;
  environmentName: string;
}

export const BundleEnvironmentModal: React.FC<BundleEnvironmentModalProps> = ({
  isOpen,
  onClose,
  environmentName
}) => {
  const [endpoint, setEndpoint] = useState('https://share.cloudshipai.com/upload');
  const [isLocal, setIsLocal] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [response, setResponse] = useState<any>(null);

  const handleBundle = async () => {
    setIsLoading(true);
    setResponse(null);

    try {
      // Call the bundles API using the correct method
      const result = await bundlesApi.create(environmentName, isLocal, isLocal ? undefined : endpoint);
      setResponse(result.data);
    } catch (error) {
      console.error('Failed to create bundle:', error);
      setResponse({ error: 'Failed to create bundle' });
    } finally {
      setIsLoading(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-[9999]">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo-glow max-w-md w-full mx-4 z-[10000] relative max-h-[90vh] overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark rounded-t-lg">
          <h2 className="text-lg font-mono font-semibold text-tokyo-fg z-10 relative">
            Bundle Environment: {environmentName}
          </h2>
          <button onClick={onClose} className="text-tokyo-comment hover:text-tokyo-fg transition-colors z-10 relative">
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-4 space-y-4 overflow-y-auto flex-1">
          {/* Warning */}
          <div className="bg-yellow-900 bg-opacity-30 border border-yellow-500 border-opacity-50 rounded p-3">
            <p className="text-sm text-yellow-300 font-mono">
              Note: Make sure your MCP servers are templates. Your variables.yml will not be part of this bundle.
            </p>
          </div>

          {/* Local Toggle */}
          <div className="flex items-center gap-3">
            <input
              type="checkbox"
              id="local-toggle"
              checked={isLocal}
              onChange={(e) => setIsLocal(e.target.checked)}
              className="w-4 h-4 text-tokyo-orange bg-tokyo-bg border-tokyo-blue7 rounded focus:ring-tokyo-orange focus:ring-2"
            />
            <label htmlFor="local-toggle" className="text-sm font-mono text-tokyo-fg">
              Save locally (skip upload)
            </label>
          </div>

          {/* Endpoint Input - Hidden when local is selected */}
          {!isLocal && (
            <div className="space-y-2">
              <label className="text-sm font-mono text-tokyo-comment">Upload Endpoint:</label>
              <input
                type="text"
                value={endpoint}
                onChange={(e) => setEndpoint(e.target.value)}
                className="w-full px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 rounded font-mono text-tokyo-fg focus:outline-none focus:border-tokyo-orange"
                placeholder="https://share.cloudshipai.com/upload"
              />
            </div>
          )}

          {/* Response Display */}
          {response && (
            <div className="space-y-3">
              {/* Success Response for share.cloudshipai.com */}
              {response.success && endpoint.includes('share.cloudshipai.com') && response.share_url && (
                <div className="bg-green-900 bg-opacity-30 border border-green-500 border-opacity-50 rounded p-4">
                  <div className="flex items-center justify-between mb-3">
                    <h4 className="text-sm font-mono text-white font-medium">Bundle Shared Successfully</h4>
                    <button
                      onClick={() => navigator.clipboard.writeText(response.share_url)}
                      className="p-1 text-green-400 hover:text-green-300 transition-colors"
                      title="Copy share URL"
                    >
                      <Copy className="h-4 w-4" />
                    </button>
                  </div>

                  <div className="space-y-3">
                    <div>
                      <div className="text-xs text-green-400 font-mono mb-1 font-medium">Share URL:</div>
                      <div className="p-2 bg-gray-900 border border-gray-600 rounded font-mono text-xs text-gray-200 break-all">
                        {response.share_url}
                      </div>
                    </div>

                    {response.share_id && (
                      <div>
                        <div className="text-xs text-green-400 font-mono mb-1 font-medium">Share ID:</div>
                        <div className="p-2 bg-gray-900 border border-gray-600 rounded font-mono text-xs text-gray-200">
                          {response.share_id}
                        </div>
                      </div>
                    )}

                    {response.expires && (
                      <div>
                        <div className="text-xs text-green-400 font-mono mb-1 font-medium">Expires:</div>
                        <div className="p-2 bg-gray-900 border border-gray-600 rounded font-mono text-xs text-gray-200">
                          {response.expires}
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Local bundle success */}
              {response.success && response.local_path && (
                <div className="bg-blue-900 bg-opacity-30 border border-blue-500 border-opacity-50 rounded p-4">
                  <h4 className="text-sm font-mono text-white font-medium mb-3">Bundle Saved Locally</h4>
                  <div>
                    <div className="text-xs text-blue-400 font-mono mb-1 font-medium">Local Path:</div>
                    <div className="p-2 bg-gray-900 border border-gray-600 rounded font-mono text-xs text-gray-200 break-all">
                      {response.local_path}
                    </div>
                  </div>
                </div>
              )}

              {/* Error response */}
              {response.error && (
                <div className="bg-red-900 bg-opacity-30 border border-red-500 border-opacity-50 rounded p-4">
                  <h4 className="text-sm font-mono text-red-400 font-medium mb-2">Error</h4>
                  <div className="text-xs text-red-400 font-mono">
                    {response.error}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-tokyo-blue7">
          <button
            onClick={handleBundle}
            disabled={isLoading}
            className="w-full px-4 py-2 bg-tokyo-orange text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-orange5 transition-colors shadow-tokyo-glow disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
          >
            {isLoading ? (
              <>
                <div className="animate-spin rounded-full h-4 w-4 border-2 border-tokyo-bg border-t-transparent"></div>
                Creating Bundle...
              </>
            ) : (
              <>
                <Package className="h-4 w-4" />
                Bundle
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
};