import React, { useState, useEffect, useMemo } from 'react';
import { X, Package, Download, Copy, AlertCircle, Cloud, Star, Building2, User, Search, CheckCircle, Globe } from 'lucide-react';
import { apiClient } from '../../api/client';

interface Bundle {
  id: string;
  bundle_id?: string;
  name: string;
  slug?: string;
  description?: string;
  version?: string;
  size?: number;
  download_count?: number;
  is_official?: boolean;
  is_public?: boolean;
  is_deprecated?: boolean;
  ownership?: 'org' | 'personal' | 'official' | 'other';
  uploaded_at?: string;
  download_url?: string;
  processing_status?: string;
  sha256?: string;
}

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
  const [cloudShipBundles, setCloudShipBundles] = useState<Bundle[]>([]);
  const [loadingBundles, setLoadingBundles] = useState(false);
  const [selectedCloudShipBundle, setSelectedCloudShipBundle] = useState<string>('');
  const [searchQuery, setSearchQuery] = useState('');
  const [activeTab, setActiveTab] = useState<'official' | 'organization' | 'all'>('official');

  // Load CloudShip bundles when modal opens and CloudShip source is selected
  useEffect(() => {
    if (isOpen && bundleSource === 'cloudship') {
      loadCloudShipBundles();
    }
  }, [isOpen, bundleSource]);

  // Auto-fill environment name when bundle is selected
  useEffect(() => {
    if (selectedCloudShipBundle && !environmentName) {
      const bundle = cloudShipBundles.find(b => (b.bundle_id || b.id) === selectedCloudShipBundle);
      if (bundle) {
        // Use bundle name as default environment name, sanitized
        const suggestedName = bundle.name
          .toLowerCase()
          .replace(/[^a-z0-9-]/g, '-')
          .replace(/-+/g, '-')
          .replace(/^-|-$/g, '');
        setEnvironmentName(suggestedName || 'my-environment');
      }
    }
  }, [selectedCloudShipBundle, cloudShipBundles]);

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

  // Filter and categorize bundles
  const { officialBundles, orgBundles, filteredBundles } = useMemo(() => {
    // Only show completed bundles (not failed or pending)
    const completedBundles = cloudShipBundles.filter(b => b.processing_status === 'completed');
    
    // Official bundles: is_official=true OR ownership='official'
    const official = completedBundles.filter(b => b.is_official || b.ownership === 'official');
    
    // Organization/Personal bundles: ownership='org' or 'personal' (and not official)
    const org = completedBundles.filter(b => 
      !b.is_official && 
      b.ownership !== 'official' && 
      (b.ownership === 'org' || b.ownership === 'personal')
    );
    
    // Apply search filter
    const searchLower = searchQuery.toLowerCase();
    const filterBySearch = (bundles: Bundle[]) => {
      if (!searchQuery) return bundles;
      return bundles.filter(b => 
        b.name?.toLowerCase().includes(searchLower) ||
        b.description?.toLowerCase().includes(searchLower)
      );
    };

    let filtered: Bundle[] = [];
    if (activeTab === 'official') {
      filtered = filterBySearch(official);
    } else if (activeTab === 'organization') {
      filtered = filterBySearch(org);
    } else {
      filtered = filterBySearch(completedBundles);
    }

    return {
      officialBundles: official,
      orgBundles: org,
      filteredBundles: filtered,
    };
  }, [cloudShipBundles, searchQuery, activeTab]);

  const handleClose = () => {
    if (!installing) {
      setBundleLocation('');
      setEnvironmentName('');
      setInstallSuccess(false);
      setError(null);
      setInstallDetails(null);
      setSelectedCloudShipBundle('');
      setSearchQuery('');
      setActiveTab('official');
      onClose();
    }
  };

  const handleInstall = async () => {
    let finalBundleLocation = bundleLocation;

    // If CloudShip source, use the selected bundle's download URL
    if (bundleSource === 'cloudship') {
      if (!selectedCloudShipBundle) {
        setError('Please select a bundle');
        return;
      }
      const selectedBundle = cloudShipBundles.find(b =>
        b.bundle_id === selectedCloudShipBundle || b.id === selectedCloudShipBundle
      );
      if (!selectedBundle) {
        setError('Selected bundle not found');
        return;
      }
      finalBundleLocation = selectedBundle.download_url || '';
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

  const formatSize = (bytes?: number) => {
    if (!bytes) return '';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '';
    return new Date(dateStr).toLocaleDateString();
  };

  const selectedBundle = cloudShipBundles.find(b => (b.bundle_id || b.id) === selectedCloudShipBundle);

  if (!isOpen) return null;

  return (
    <div 
      className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
      onClick={onClose}
    >
      <div 
        className="bg-white border border-gray-200 rounded-lg w-full max-w-lg p-6 shadow-xl max-h-[85vh] overflow-hidden flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between mb-4">
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
                className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200 p-1 rounded bg-blue-600 hover:bg-blue-700 text-white"
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
              className="mt-4 px-4 py-2 bg-gray-900 text-white hover:bg-gray-800 rounded text-sm transition-colors"
            >
              Close
            </button>
          </div>
        ) : (
          <div className="space-y-4 flex-1 overflow-hidden flex flex-col">
            {error && (
              <div className="bg-red-50 border border-red-200 rounded-lg p-3 flex items-start space-x-2">
                <AlertCircle className="h-5 w-5 text-red-600 flex-shrink-0 mt-0.5" />
                <p className="text-sm text-red-600">{error}</p>
              </div>
            )}

            {/* Source Selection */}
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
                    className="text-blue-600 focus:ring-blue-600"
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
                    className="text-blue-600 focus:ring-blue-600"
                  />
                  <span className="text-sm text-gray-900">URL</span>
                </label>
                <label className="flex items-center space-x-2 cursor-pointer">
                  <input
                    type="radio"
                    value="file"
                    checked={bundleSource === 'file'}
                    onChange={(e) => setBundleSource(e.target.value as 'url' | 'file' | 'cloudship')}
                    className="text-blue-600 focus:ring-blue-600"
                  />
                  <span className="text-sm text-gray-900">File Path</span>
                </label>
              </div>
            </div>

            {bundleSource === 'cloudship' ? (
              <div className="flex-1 overflow-hidden flex flex-col min-h-0">
                {/* Tabs */}
                <div className="flex border-b border-gray-200 mb-3">
                  <button
                    onClick={() => setActiveTab('official')}
                    className={`flex items-center gap-1.5 px-3 py-2 text-sm font-medium border-b-2 transition-colors ${
                      activeTab === 'official'
                        ? 'border-blue-600 text-blue-600'
                        : 'border-transparent text-gray-500 hover:text-gray-700'
                    }`}
                  >
                    <Star className="h-3.5 w-3.5" />
                    Official ({officialBundles.length})
                  </button>
                  <button
                    onClick={() => setActiveTab('organization')}
                    className={`flex items-center gap-1.5 px-3 py-2 text-sm font-medium border-b-2 transition-colors ${
                      activeTab === 'organization'
                        ? 'border-blue-600 text-blue-600'
                        : 'border-transparent text-gray-500 hover:text-gray-700'
                    }`}
                  >
                    <Building2 className="h-3.5 w-3.5" />
                    My Bundles ({orgBundles.length})
                  </button>
                  <button
                    onClick={() => setActiveTab('all')}
                    className={`flex items-center gap-1.5 px-3 py-2 text-sm font-medium border-b-2 transition-colors ${
                      activeTab === 'all'
                        ? 'border-blue-600 text-blue-600'
                        : 'border-transparent text-gray-500 hover:text-gray-700'
                    }`}
                  >
                    <Globe className="h-3.5 w-3.5" />
                    All
                  </button>
                </div>

                {/* Search */}
                <div className="relative mb-3">
                  <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder="Search bundles..."
                    className="w-full pl-9 pr-3 py-2 bg-gray-50 border border-gray-200 rounded-lg text-sm focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                  />
                </div>

                {/* Bundle List */}
                {loadingBundles ? (
                  <div className="flex items-center justify-center py-8 text-gray-500">
                    <div className="animate-spin h-5 w-5 border-2 border-blue-600 border-t-transparent rounded-full mr-2"></div>
                    Loading bundles...
                  </div>
                ) : filteredBundles.length === 0 ? (
                  <div className="text-center py-8 text-gray-500 text-sm">
                    {searchQuery ? 'No bundles match your search' : 'No bundles available'}
                  </div>
                ) : (
                  <div className="flex-1 overflow-y-auto space-y-2 min-h-0 pr-1">
                    {filteredBundles.map((bundle) => {
                      const bundleId = bundle.bundle_id || bundle.id;
                      const isSelected = selectedCloudShipBundle === bundleId;
                      
                      return (
                        <button
                          key={bundleId}
                          onClick={() => setSelectedCloudShipBundle(bundleId)}
                          className={`w-full text-left p-3 rounded-lg border transition-all ${
                            isSelected
                              ? 'border-blue-500 bg-blue-50 ring-1 ring-blue-500'
                              : 'border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50'
                          }`}
                        >
                          <div className="flex items-start justify-between gap-2">
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center gap-2">
                                <span className="font-medium text-gray-900 truncate">
                                  {bundle.name}
                                </span>
                                {bundle.is_official && (
                                  <span className="inline-flex items-center gap-0.5 px-1.5 py-0.5 bg-amber-100 text-amber-700 text-[10px] font-medium rounded">
                                    <Star className="h-2.5 w-2.5" />
                                    Official
                                  </span>
                                )}
                                {bundle.is_public && !bundle.is_official && (
                                  <span className="inline-flex items-center gap-0.5 px-1.5 py-0.5 bg-green-100 text-green-700 text-[10px] font-medium rounded">
                                    <Globe className="h-2.5 w-2.5" />
                                    Public
                                  </span>
                                )}
                              </div>
                              {bundle.description && (
                                <p className="text-xs text-gray-500 mt-0.5 line-clamp-1">
                                  {bundle.description}
                                </p>
                              )}
                              <div className="flex items-center gap-3 mt-1 text-[11px] text-gray-400">
                                {bundle.version && <span>v{bundle.version}</span>}
                                {bundle.size && <span>{formatSize(bundle.size)}</span>}
                                {bundle.uploaded_at && <span>{formatDate(bundle.uploaded_at)}</span>}
                                {bundle.download_count !== undefined && bundle.download_count > 0 && (
                                  <span>{bundle.download_count} downloads</span>
                                )}
                              </div>
                            </div>
                            {isSelected && (
                              <CheckCircle className="h-5 w-5 text-blue-600 flex-shrink-0" />
                            )}
                          </div>
                        </button>
                      );
                    })}
                  </div>
                )}

                {/* Selected Bundle Preview */}
                {selectedBundle && (
                  <div className="mt-3 p-3 bg-blue-50 border border-blue-200 rounded-lg">
                    <div className="text-xs text-blue-700">
                      <span className="font-medium">Selected:</span> {selectedBundle.name}
                      {selectedBundle.version && <span className="ml-1 text-blue-500">v{selectedBundle.version}</span>}
                    </div>
                  </div>
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
                  className="w-full bg-white border border-gray-300 text-gray-900 px-3 py-2 rounded focus:outline-none focus:border-blue-600 focus:ring-1 focus:ring-blue-600 placeholder:text-gray-400 transition-colors"
                  disabled={installing}
                />
              </div>
            )}

            {/* Environment Name */}
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
                className="w-full bg-white border border-gray-300 text-gray-900 px-3 py-2 rounded focus:outline-none focus:border-blue-600 focus:ring-1 focus:ring-blue-600 placeholder:text-gray-400 transition-colors"
                disabled={installing}
              />
              <p className="text-xs text-gray-500 mt-1">
                The bundle will be installed to this environment
              </p>
            </div>

            {/* Actions */}
            <div className="flex space-x-3 pt-2">
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
