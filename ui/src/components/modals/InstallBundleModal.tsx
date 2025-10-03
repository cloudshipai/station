import React, { useState, useEffect } from 'react';
import { X, Package, Download, Copy, AlertCircle, Cloud } from 'lucide-react';
import { apiClient } from '../../api/client';

interface InstallBundleModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
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
      const selectedBundle = cloudShipBundles.find(b => b.bundle_id === selectedCloudShipBundle);
      if (!selectedBundle) {
        setError('Selected bundle not found');
        return;
      }
      finalBundleLocation = `https://api.cloudshipai.com${selectedBundle.download_url}`;
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
      onSuccess?.();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to install bundle');
    } finally {
      setInstalling(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg w-full max-w-md p-6">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-lg font-mono font-semibold text-tokyo-magenta">Install Bundle</h2>
          <button
            onClick={handleClose}
            disabled={installing}
            className="text-tokyo-comment hover:text-tokyo-fg transition-colors disabled:opacity-50"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {installSuccess ? (
          <div className="text-center py-8">
            <div className="bg-transparent border border-tokyo-green border-opacity-50 rounded-lg p-4 mb-4">
              <Package className="h-8 w-8 text-tokyo-green mx-auto mb-2" />
              <p className="text-tokyo-green font-mono">Bundle installed successfully!</p>
              {installDetails && (
                <div className="mt-3 text-sm text-tokyo-comment space-y-1">
                  <p>Environment: <span className="text-tokyo-fg">{installDetails.environment_name}</span></p>
                  <p>Agents: <span className="text-tokyo-blue">{installDetails.installed_agents || 0}</span></p>
                  <p>MCP Servers: <span className="text-tokyo-cyan">{installDetails.installed_mcps || 0}</span></p>
                </div>
              )}
            </div>
            <div className="bg-transparent border border-tokyo-blue border-opacity-50 rounded-lg p-3 relative group">
              <button
                onClick={() => navigator.clipboard.writeText(`stn sync ${environmentName}`)}
                className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200 p-1 rounded bg-tokyo-blue hover:bg-tokyo-blue1 text-tokyo-bg"
                title={`Copy sync command: stn sync ${environmentName}`}
              >
                <Copy className="h-3 w-3" />
              </button>
              <p className="text-sm text-tokyo-blue font-mono pr-8">
                Next: Run <code className="bg-tokyo-bg px-1 rounded text-tokyo-orange">stn sync {environmentName}</code>
              </p>
            </div>
            <button
              onClick={handleClose}
              className="mt-4 px-4 py-2 bg-tokyo-blue text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm transition-colors"
            >
              Close
            </button>
          </div>
        ) : (
          <div className="space-y-4">
            {error && (
              <div className="bg-tokyo-red bg-opacity-10 border border-tokyo-red border-opacity-50 rounded-lg p-3 flex items-start space-x-2">
                <AlertCircle className="h-5 w-5 text-tokyo-red flex-shrink-0 mt-0.5" />
                <p className="text-sm text-tokyo-red font-mono">{error}</p>
              </div>
            )}

            <div>
              <label className="block text-sm font-mono text-tokyo-fg mb-2">
                Bundle Source
              </label>
              <div className="flex space-x-4">
                <label className="flex items-center space-x-2 cursor-pointer">
                  <input
                    type="radio"
                    value="cloudship"
                    checked={bundleSource === 'cloudship'}
                    onChange={(e) => setBundleSource(e.target.value as 'url' | 'file' | 'cloudship')}
                    className="text-tokyo-blue focus:ring-tokyo-blue"
                  />
                  <span className="text-sm font-mono text-tokyo-fg flex items-center gap-1">
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
                    className="text-tokyo-blue focus:ring-tokyo-blue"
                  />
                  <span className="text-sm font-mono text-tokyo-fg">URL</span>
                </label>
                <label className="flex items-center space-x-2 cursor-pointer">
                  <input
                    type="radio"
                    value="file"
                    checked={bundleSource === 'file'}
                    onChange={(e) => setBundleSource(e.target.value as 'url' | 'file' | 'cloudship')}
                    className="text-tokyo-blue focus:ring-tokyo-blue"
                  />
                  <span className="text-sm font-mono text-tokyo-fg">File Path</span>
                </label>
              </div>
            </div>

            {bundleSource === 'cloudship' ? (
              <div>
                <label htmlFor="cloudship-bundle" className="block text-sm font-mono text-tokyo-fg mb-2">
                  Organization Bundle
                </label>
                {loadingBundles ? (
                  <div className="w-full bg-tokyo-dark1 border border-tokyo-dark4 text-tokyo-comment font-mono px-3 py-2 rounded flex items-center gap-2">
                    <div className="animate-spin h-4 w-4 border-2 border-tokyo-blue border-t-transparent rounded-full"></div>
                    Loading bundles...
                  </div>
                ) : cloudShipBundles.length === 0 ? (
                  <div className="w-full bg-tokyo-dark1 border border-tokyo-red text-tokyo-red font-mono px-3 py-2 rounded text-sm">
                    No bundles found in your organization
                  </div>
                ) : (
                  <select
                    id="cloudship-bundle"
                    value={selectedCloudShipBundle}
                    onChange={(e) => setSelectedCloudShipBundle(e.target.value)}
                    className="w-full bg-tokyo-dark1 border border-tokyo-dark4 text-tokyo-fg font-mono px-3 py-2 rounded focus:outline-none focus:border-tokyo-orange hover:border-tokyo-blue5 transition-colors"
                    disabled={installing}
                  >
                    <option value="">Select a bundle...</option>
                    {cloudShipBundles.map((bundle) => (
                      <option key={bundle.bundle_id} value={bundle.bundle_id}>
                        {bundle.filename || bundle.bundle_id} ({new Date(bundle.uploaded_at).toLocaleDateString()})
                      </option>
                    ))}
                  </select>
                )}
              </div>
            ) : (
              <div>
                <label htmlFor="bundle-location" className="block text-sm font-mono text-tokyo-fg mb-2">
                  {bundleSource === 'url' ? 'Bundle URL' : 'Bundle File Path'}
                </label>
                <input
                  id="bundle-location"
                  type="text"
                  value={bundleLocation}
                  onChange={(e) => setBundleLocation(e.target.value)}
                  placeholder={bundleSource === 'url' ? 'https://example.com/bundle.tar.gz' : '/path/to/bundle.tar.gz'}
                  className="w-full bg-tokyo-dark1 border border-tokyo-dark4 text-tokyo-fg font-mono px-3 py-2 rounded focus:outline-none focus:border-tokyo-orange hover:border-tokyo-blue5 transition-colors"
                  disabled={installing}
                />
              </div>
            )}

            <div>
              <label htmlFor="environment-name" className="block text-sm font-mono text-tokyo-fg mb-2">
                Environment Name
              </label>
              <input
                id="environment-name"
                type="text"
                value={environmentName}
                onChange={(e) => setEnvironmentName(e.target.value)}
                placeholder="my-environment"
                className="w-full bg-tokyo-dark1 border border-tokyo-dark4 text-tokyo-fg font-mono px-3 py-2 rounded focus:outline-none focus:border-tokyo-orange hover:border-tokyo-blue5 transition-colors"
                disabled={installing}
              />
            </div>

            <div className="flex space-x-3 mt-6">
              <button
                onClick={handleClose}
                disabled={installing}
                className="flex-1 px-4 py-2 bg-tokyo-dark2 text-tokyo-fg hover:bg-tokyo-dark4 rounded font-mono text-sm transition-colors disabled:opacity-50"
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
                className="flex-1 flex items-center justify-center space-x-2 px-4 py-2 bg-tokyo-magenta text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {installing ? (
                  <>
                    <div className="animate-spin h-4 w-4 border-2 border-tokyo-bg border-t-transparent rounded-full"></div>
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
