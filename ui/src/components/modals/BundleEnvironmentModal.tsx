import React, { useState, useEffect } from 'react';
import { X, Package, Copy, Cloud, HelpCircle, Upload } from 'lucide-react';
import { bundlesApi } from '../../api/station';

interface BundleEnvironmentModalProps {
  isOpen: boolean;
  onClose: () => void;
  environmentName: string;
  cloudShipConnected?: boolean;
}

export const BundleEnvironmentModal: React.FC<BundleEnvironmentModalProps> = ({
  isOpen,
  onClose,
  environmentName,
  cloudShipConnected = false
}) => {
  const [bundleType, setBundleType] = useState<'cloudship' | 'local' | 'public'>('local');
  const [publicEndpoint, setPublicEndpoint] = useState('https://share.cloudshipai.com/upload');
  const [isLoading, setIsLoading] = useState(false);
  const [response, setResponse] = useState<any>(null);

  // Auto-select local if CloudShip not connected
  useEffect(() => {
    if (!cloudShipConnected && bundleType === 'cloudship') {
      setBundleType('local');
    }
  }, [cloudShipConnected, bundleType]);

  // Set default based on connection status when modal opens
  useEffect(() => {
    if (isOpen) {
      setBundleType(cloudShipConnected ? 'cloudship' : 'local');
      setResponse(null);
    }
  }, [isOpen, cloudShipConnected]);

  const handleBundle = async () => {
    setIsLoading(true);
    setResponse(null);

    try {
      // Call the bundles API
      // - local=true for local bundles
      // - local=false with endpoint for public share
      // - local=false without endpoint for CloudShip
      const endpoint = bundleType === 'public' ? publicEndpoint : undefined;
      const result = await bundlesApi.create(environmentName, bundleType === 'local', endpoint);
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
    <div 
      className="fixed inset-0 bg-black/80 flex items-center justify-center z-[9999]"
      onClick={onClose}
    >
      <div 
        className="bg-white border border-gray-200 rounded-lg shadow-xl max-w-md w-full mx-4 z-[10000] relative max-h-[90vh] overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-gray-200 bg-white rounded-t-lg">
          <h2 className="text-lg font-semibold text-gray-900 z-10 relative">
            Publish Bundle: {environmentName}
          </h2>
          <button onClick={onClose} className="text-gray-600 hover:text-gray-900 transition-colors z-10 relative">
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-4 space-y-4 overflow-y-auto flex-1">
          {/* Warning */}
          <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-3">
            <p className="text-sm text-yellow-800">
              Note: Make sure your MCP servers are templates. Your variables.yml will not be part of this bundle.
            </p>
          </div>

          {/* Bundle Type Selection */}
          <div className="space-y-3">
            <label className="text-sm font-medium text-gray-900">Bundle Destination:</label>
            <div className="space-y-2">
              {/* CloudShip Option */}
              <div className={`flex items-center gap-3 ${!cloudShipConnected ? 'opacity-50' : ''}`}>
                <input
                  type="radio"
                  id="cloudship-bundle"
                  checked={bundleType === 'cloudship'}
                  onChange={() => setBundleType('cloudship')}
                  disabled={!cloudShipConnected}
                  className="w-4 h-4 text-station-blue focus:ring-station-blue focus:ring-2 disabled:cursor-not-allowed"
                />
                <label 
                  htmlFor="cloudship-bundle" 
                  className={`text-sm flex items-center gap-2 ${!cloudShipConnected ? 'text-gray-400 cursor-not-allowed' : 'text-gray-900 cursor-pointer'}`}
                >
                  <Cloud className="h-4 w-4" />
                  Upload to CloudShip (Organization Bundle)
                  {!cloudShipConnected && (
                    <div className="relative group">
                      <HelpCircle className="h-4 w-4 text-gray-400" />
                      <div className="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 hidden group-hover:block w-48 p-2 bg-gray-900 text-white text-xs rounded shadow-lg z-50">
                        CloudShip API key required. Set it in Settings → CloudShip Integration.
                        <div className="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-gray-900"></div>
                      </div>
                    </div>
                  )}
                </label>
              </div>
              
              {/* Public Share Option */}
              <div className="flex items-center gap-3">
                <input
                  type="radio"
                  id="public-bundle"
                  checked={bundleType === 'public'}
                  onChange={() => setBundleType('public')}
                  className="w-4 h-4 text-station-blue focus:ring-station-blue focus:ring-2"
                />
                <label htmlFor="public-bundle" className="text-sm text-gray-900 cursor-pointer">
                  Upload to Public Share
                </label>
              </div>
              
              {/* Local Option */}
              <div className="flex items-center gap-3">
                <input
                  type="radio"
                  id="local-bundle"
                  checked={bundleType === 'local'}
                  onChange={() => setBundleType('local')}
                  className="w-4 h-4 text-station-blue focus:ring-station-blue focus:ring-2"
                />
                <label htmlFor="local-bundle" className="text-sm text-gray-900 cursor-pointer">
                  Save Locally
                </label>
              </div>
            </div>
          </div>

          {/* Public Endpoint Input - Show when public is selected */}
          {bundleType === 'public' && (
            <div className="space-y-2">
              <label className="text-sm font-medium text-gray-900">Upload Endpoint:</label>
              <input
                type="text"
                value={publicEndpoint}
                onChange={(e) => setPublicEndpoint(e.target.value)}
                className="w-full px-3 py-2 bg-white border border-gray-300 rounded font-mono text-sm text-gray-900 focus:outline-none focus:border-station-blue focus:ring-1 focus:ring-station-blue"
                placeholder="https://share.cloudshipai.com/upload"
              />
            </div>
          )}

          {/* Response Display */}
          {response && (
            <div className="space-y-3">
              {/* CloudShip upload success */}
              {response.success && response.cloudship_info && (
                <div className="bg-green-50 border border-green-200 rounded-lg p-4">
                  <div className="flex items-center justify-between mb-3">
                    <h4 className="text-sm text-gray-900 font-medium">Uploaded to CloudShip</h4>
                    <button
                      onClick={() => navigator.clipboard.writeText(response.share_url)}
                      className="p-1 text-green-600 hover:text-green-700 transition-colors"
                      title="Copy download URL"
                    >
                      <Copy className="h-4 w-4" />
                    </button>
                  </div>

                  <div className="space-y-3">
                    <div>
                      <div className="text-xs text-gray-700 mb-1 font-medium">Organization:</div>
                      <div className="p-2 bg-white border border-gray-200 rounded font-mono text-xs text-gray-900">
                        {response.cloudship_info.organization}
                      </div>
                    </div>

                    <div>
                      <div className="text-xs text-gray-700 mb-1 font-medium">Bundle ID:</div>
                      <div className="p-2 bg-white border border-gray-200 rounded font-mono text-xs text-gray-900">
                        {response.cloudship_info.bundle_id}
                      </div>
                    </div>

                    <div>
                      <div className="text-xs text-gray-700 mb-1 font-medium">Download URL:</div>
                      <div className="p-2 bg-white border border-gray-200 rounded font-mono text-xs text-gray-900 break-all">
                        {response.share_url}
                      </div>
                    </div>

                    {response.cloudship_info.uploaded_at && (
                      <div>
                        <div className="text-xs text-gray-700 mb-1 font-medium">Uploaded:</div>
                        <div className="p-2 bg-white border border-gray-200 rounded font-mono text-xs text-gray-900">
                          {response.cloudship_info.uploaded_at}
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Public share success */}
              {response.success && response.share_url && !response.cloudship_info && (
                <div className="bg-green-50 border border-green-200 rounded-lg p-4">
                  <div className="flex items-center justify-between mb-3">
                    <h4 className="text-sm text-gray-900 font-medium">Bundle Shared Successfully</h4>
                    <button
                      onClick={() => navigator.clipboard.writeText(response.share_url)}
                      className="p-1 text-green-600 hover:text-green-700 transition-colors"
                      title="Copy share URL"
                    >
                      <Copy className="h-4 w-4" />
                    </button>
                  </div>

                  <div className="space-y-3">
                    {response.share_id && (
                      <div>
                        <div className="text-xs text-gray-700 mb-1 font-medium">Share ID:</div>
                        <div className="p-2 bg-white border border-gray-200 rounded font-mono text-xs text-gray-900">
                          {response.share_id}
                        </div>
                      </div>
                    )}

                    <div>
                      <div className="text-xs text-gray-700 mb-1 font-medium">Share URL:</div>
                      <div className="p-2 bg-white border border-gray-200 rounded font-mono text-xs text-gray-900 break-all">
                        {response.share_url}
                      </div>
                    </div>

                    {response.expires && (
                      <div>
                        <div className="text-xs text-gray-700 mb-1 font-medium">Expires:</div>
                        <div className="p-2 bg-white border border-gray-200 rounded font-mono text-xs text-gray-900">
                          {response.expires}
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Local bundle success */}
              {response.success && response.local_path && (
                <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
                  <h4 className="text-sm text-gray-900 font-medium mb-3">Bundle Saved Locally</h4>
                  <div>
                    <div className="text-xs text-gray-700 mb-1 font-medium">Local Path:</div>
                    <div className="p-2 bg-white border border-gray-200 rounded font-mono text-xs text-gray-900 break-all">
                      {response.local_path}
                    </div>
                  </div>
                </div>
              )}

              {/* Error response */}
              {response.error && (
                <div className="bg-red-50 border border-red-200 rounded-lg p-4">
                  <h4 className="text-sm text-red-600 font-medium mb-2">Error</h4>
                  <div className="text-xs text-red-600">
                    {response.error}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-gray-200">
          {!response?.success ? (
            <button
              onClick={handleBundle}
              disabled={isLoading}
              className="w-full px-4 py-2 bg-station-blue text-white rounded font-medium hover:bg-blue-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            >
              {isLoading ? (
                <>
                  <div className="animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent"></div>
                  Publishing Bundle...
                </>
              ) : (
                <>
                  <Upload className="h-4 w-4" />
                  Publish Bundle
                </>
              )}
            </button>
          ) : (
            <button
              onClick={onClose}
              className="w-full px-4 py-2 bg-gray-100 text-gray-900 rounded font-medium hover:bg-gray-200 transition-colors"
            >
              Close
            </button>
          )}
        </div>
      </div>
    </div>
  );
};
