import React, { useState, useEffect } from 'react';
import { X, Package, Download, Copy, AlertCircle, Cloud } from 'lucide-react';
import { apiClient } from '../../api/client';

interface InstallBundleModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: (environmentName: string) => void;
}

export const InstallBundleModal: React.FC<InstallBundleModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
}) => {
  const [bundleSource, setBundleSource] = useState<'url' | 'file' | 'cloudship'>('cloudship');
  const [bundleLocation, setBundleLocation] = useState('');
  const [environmentName, setEnvironmentName] = useState('');
  const [installing, setInstalling] = useState(false);
  const [installSuccess, setInstallSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [installDetails, setInstallDetails] = useState<any>(null);
  const [cloudShipBundles, setCloudShipBundles] = useState<any[]>([]);
  const [loadingBundles, setLoadingBundles] = useState(false);
  const [selectedCloudShipBundle, setSelectedCloudShipBundle] = useState<string>('');

  // Load CloudShip bundles when modal opens and CloudShip source is selected
  useEffect(() => {
    if (isOpen && bundleSource === 'cloudship') {
      loadCloudShipBundles();
    }
  }, [isOpen, bundleSource]);

  const loadCloudShipBundles = async () => {
    try {
      setLoadingBundles(true);
      setError(null);
      const response = await apiClient.get('/bundles/cloudship');
      setCloudShipBundles(response.data.bundles || []);
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to load CloudShip bundles');
      setCloudShipBundles([]);
    } finally {
      setLoadingBundles(false);
    }
  };

  const handleClose = () => {
    if (!installing) {
      setBundleLocation('');
      setEnvironmentName('');
      setInstallSuccess(false);
      setError(null);
      setInstallDetails(null);
      setSelectedCloudShipBundle('');
      onClose();
    }
  };

  const handleInstall = async () => {
    let finalBundleLocation = bundleLocation;

    // If CloudShip source, use the selected bundle's download URL
    if (bundleSource === 'cloudship') {
      if (!selectedCloudShipBundle) {
        setError('Please select a bundle from your organization');
        return;
      }
      const selectedBundle = cloudShipBundles.find(b =>
        b.bundle_id === selectedCloudShipBundle || b.id === selectedCloudShipBundle
      );
      if (!selectedBundle) {
        console.log('Available bundles:', cloudShipBundles);
        console.log('Selected bundle ID:', selectedCloudShipBundle);
        setError(`Selected bundle not found. Available: ${cloudShipBundles.length} bundles`);
        return;
      }
      // Use the absolute download_url from backend (backend converts relative to absolute)
      finalBundleLocation = selectedBundle.download_url;
    }

    if (!finalBundleLocation.trim() || !environmentName.trim()) {
      setError('Please provide both bundle location and environment name');
      return;
    }

    try {
      setInstalling(true);
      setError(null);

      const response = await apiClient.post('/bundles/install', {
        bundle_location: finalBundleLocation,
        environment_name: environmentName,
        source: bundleSource === 'cloudship' ? 'url' : bundleSource,
      });

      setInstallDetails(response.data);
      setInstallSuccess(true);
      onSuccess?.(environmentName);
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to install bundle');
    } finally {
      setInstalling(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white border border-gray-200 rounded-lg w-full max-w-md p-6 shadow-xl">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-lg font-semibold text-gray-900">Install Bundle</h2>
          <button
            onClick={handleClose}
            disabled={installing}
            className="text-gray-500 hover:text-gray-900 transition-colors disabled:opacity-50"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {installSuccess ? (
          <div className="text-center py-8">
            <div className="bg-green-50 border border-green-200 rounded-lg p-4 mb-4">
              <Package className="h-8 w-8 text-green-600 mx-auto mb-2" />
              <p className="text-green-600 font-semibold">Bundle installed successfully!</p>
              {installDetails && (
                <div className="mt-3 text-sm text-gray-600 space-y-1">
                  <p>Environment: <span className="text-gray-900 font-medium">{installDetails.environment_name}</span></p>
                  <p>Agents: <span className="text-blue-600">{installDetails.installed_agents || 0}</span></p>
                  <p>MCP Servers: <span className="text-cyan-600">{installDetails.installed_mcps || 0}</span></p>
                </div>
              )}
            </div>
            <div className="bg-blue-50 border border-blue-200 rounded-lg p-3 relative group">
              <button
                onClick={() => navigator.clipboard.writeText(`stn sync ${environmentName}`)}
                className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200 p-1 rounded bg-station-blue hover:bg-blue-600 text-white"
                title={`Copy sync command: stn sync ${environmentName}`}
              >
                <Copy className="h-3 w-3" />
              </button>
              <p className="text-sm text-gray-700 pr-8">
                Next: Run <code className="bg-white px-1 rounded text-orange-600 font-mono">stn sync {environmentName}</code>
              </p>
            </div>
            <button
              onClick={handleClose}
              className="mt-4 px-4 py-2 bg-station-blue text-white hover:bg-blue-600 rounded text-sm transition-colors"
            >
              Close
            </button>
          </div>
        ) : (
          <div className="space-y-4">
            {error && (
              <div className="bg-red-50 border border-red-200 rounded-lg p-3 flex items-start space-x-2">
                <AlertCircle className="h-5 w-5 text-red-600 flex-shrink-0 mt-0.5" />
                <p className="text-sm text-red-600">{error}</p>
              </div>
            )}

            <div>
              <label className="block text-sm text-gray-700 mb-2 font-medium">
                Bundle Source
              </label>
              <div className="flex space-x-4">
                <label className="flex items-center space-x-2 cursor-pointer">
                  <input
                    type="radio"
                    value="cloudship"
                    checked={bundleSource === 'cloudship'}
                    onChange={(e) => setBundleSource(e.target.value as 'url' | 'file' | 'cloudship')}
                    className="text-station-blue focus:ring-station-blue"
                  />
                  <span className="text-sm text-gray-900 flex items-center gap-1">
                    <Cloud className="h-3 w-3" />
                    CloudShip
                  </span>
                </label>
                <label className="flex items-center space-x-2 cursor-pointer">
                  <input
                    type="radio"
                    value="url"
                    checked={bundleSource === 'url'}
                    onChange={(e) => setBundleSource(e.target.value as 'url' | 'file' | 'cloudship')}
                    className="text-station-blue focus:ring-station-blue"
                  />
                  <span className="text-sm text-gray-900">URL</span>
                </label>
                <label className="flex items-center space-x-2 cursor-pointer">
                  <input
                    type="radio"
                    value="file"
                    checked={bundleSource === 'file'}
                    onChange={(e) => setBundleSource(e.target.value as 'url' | 'file' | 'cloudship')}
                    className="text-station-blue focus:ring-station-blue"
                  />
                  <span className="text-sm text-gray-900">File Path</span>
                </label>
              </div>
            </div>

            {bundleSource === 'cloudship' ? (
              <div>
                <label htmlFor="cloudship-bundle" className="block text-sm text-gray-700 mb-2 font-medium">
                  Organization Bundle
                </label>
                {loadingBundles ? (
                  <div className="w-full bg-gray-50 border border-gray-300 text-gray-600 px-3 py-2 rounded flex items-center gap-2">
                    <div className="animate-spin h-4 w-4 border-2 border-station-blue border-t-transparent rounded-full"></div>
                    Loading bundles...
                  </div>
                ) : cloudShipBundles.length === 0 ? (
                  <div className="w-full bg-red-50 border border-red-200 text-red-600 px-3 py-2 rounded text-sm">
                    No bundles found in your organization
                  </div>
                ) : (
                  <select
                    id="cloudship-bundle"
                    value={selectedCloudShipBundle}
                    onChange={(e) => setSelectedCloudShipBundle(e.target.value)}
                    className="w-full bg-white border border-gray-300 text-gray-900 px-3 py-2 rounded focus:outline-none focus:border-station-blue focus:ring-1 focus:ring-station-blue transition-colors"
                    disabled={installing}
                  >
                    <option value="">Select a bundle...</option>
                    {cloudShipBundles.map((bundle) => {
                      // Try multiple fields for name: name, filename (without .tar.gz), or fallback to version
                      const rawName = bundle.name || bundle.filename?.replace('.tar.gz', '').replace('.tar', '') || `Bundle v${bundle.version || '1.0.0'}`;
                      const uploadDate = bundle.uploaded_at ? new Date(bundle.uploaded_at).toLocaleDateString() : '';
                      const displayText = `${rawName} - ${uploadDate}`;

                      return (
                        <option key={bundle.bundle_id || bundle.id} value={bundle.bundle_id || bundle.id}>
                          {displayText}
                        </option>
                      );
                    })}
                  </select>
                )}
              </div>
            ) : (
              <div>
                <label htmlFor="bundle-location" className="block text-sm text-gray-700 mb-2 font-medium">
                  {bundleSource === 'url' ? 'Bundle URL' : 'Bundle File Path'}
                </label>
                <input
                  id="bundle-location"
                  type="text"
                  value={bundleLocation}
                  onChange={(e) => setBundleLocation(e.target.value)}
                  placeholder={bundleSource === 'url' ? 'https://example.com/bundle.tar.gz' : '/path/to/bundle.tar.gz'}
                  className="w-full bg-white border border-gray-300 text-gray-900 px-3 py-2 rounded focus:outline-none focus:border-station-blue focus:ring-1 focus:ring-station-blue placeholder:text-gray-400 transition-colors"
                  disabled={installing}
                />
              </div>
            )}

            <div>
              <label htmlFor="environment-name" className="block text-sm text-gray-700 mb-2 font-medium">
                Environment Name
              </label>
              <input
                id="environment-name"
                type="text"
                value={environmentName}
                onChange={(e) => setEnvironmentName(e.target.value)}
                placeholder="my-environment"
                className="w-full bg-white border border-gray-300 text-gray-900 px-3 py-2 rounded focus:outline-none focus:border-station-blue focus:ring-1 focus:ring-station-blue placeholder:text-gray-400 transition-colors"
                disabled={installing}
              />
            </div>

            <div className="flex space-x-3 mt-6">
              <button
                onClick={handleClose}
                disabled={installing}
                className="flex-1 px-4 py-2 bg-white border border-gray-300 text-gray-700 hover:bg-gray-50 rounded text-sm transition-colors disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                onClick={handleInstall}
                disabled={
                  installing ||
                  !environmentName.trim() ||
                  (bundleSource === 'cloudship' ? !selectedCloudShipBundle : !bundleLocation.trim())
                }
                className="flex-1 flex items-center justify-center space-x-2 px-4 py-2 bg-pink-600 text-white hover:bg-pink-700 rounded text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {installing ? (
                  <>
                    <div className="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full"></div>
                    <span>Installing...</span>
                  </>
                ) : (
                  <>
                    <Download className="h-4 w-4" />
                    <span>Install Bundle</span>
                  </>
                )}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
