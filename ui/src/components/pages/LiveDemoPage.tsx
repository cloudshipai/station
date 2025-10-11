import React, { useState, useEffect } from 'react';
import { Play, Package, Download, CheckCircle, AlertCircle, Loader, Sparkles, Lock, BarChart3, Server, Database, Brain, Shield, GitBranch } from 'lucide-react';
import { SyncModal } from '../sync/SyncModal';

interface DemoBundle {
  id: string;
  name: string;
  description: string;
  category: string;
  size: number;
}

type AppCategory = 'finops' | 'security' | 'reliability' | 'deployments' | 'data-platform' | 'mlops';

interface AppCategoryInfo {
  id: AppCategory;
  name: string;
  icon: React.ReactNode;
  description: string;
  color: string;
  available: boolean;
}

const APP_CATEGORIES: AppCategoryInfo[] = [
  {
    id: 'finops',
    name: 'FinOps',
    icon: <BarChart3 size={24} />,
    description: 'Cost optimization, forecasting, and financial governance',
    color: 'tokyo-green',
    available: true,
  },
  {
    id: 'security',
    name: 'Security',
    icon: <Shield size={24} />,
    description: 'Vulnerability management, compliance, and threat detection',
    color: 'tokyo-red',
    available: true,
  },
  {
    id: 'reliability',
    name: 'Reliability',
    icon: <Server size={24} />,
    description: 'SRE, observability, SLO tracking, and incident management',
    color: 'tokyo-blue',
    available: true,
  },
  {
    id: 'deployments',
    name: 'Deployments',
    icon: <GitBranch size={24} />,
    description: 'CI/CD pipeline optimization and release management',
    color: 'tokyo-purple',
    available: true,
  },
  {
    id: 'data-platform',
    name: 'Data Platform',
    icon: <Database size={24} />,
    description: 'DataOps, warehouse optimization, and lineage tracking',
    color: 'tokyo-cyan',
    available: false,
  },
  {
    id: 'mlops',
    name: 'MLOps',
    icon: <Brain size={24} />,
    description: 'Model lifecycle, training optimization, and inference monitoring',
    color: 'tokyo-orange',
    available: false,
  },
];

export const LiveDemoPage: React.FC = () => {
  const [activeTab, setActiveTab] = useState<AppCategory>('finops');
  const [demoBundles, setDemoBundles] = useState<DemoBundle[]>([]);
  const [loading, setLoading] = useState(true);
  const [installing, setInstalling] = useState<string | null>(null);
  const [installSuccess, setInstallSuccess] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [environmentName, setEnvironmentName] = useState('');
  const [syncModalOpen, setSyncModalOpen] = useState(false);
  const [syncEnvironment, setSyncEnvironment] = useState('');

  useEffect(() => {
    loadDemoBundles();
  }, []);

  const loadDemoBundles = async () => {
    try {
      setLoading(true);
      setError(null);

      // Call HTTP API to list demo bundles
      const response = await fetch('http://localhost:8585/api/v1/demo-bundles');
      const data = await response.json();

      if (data.success) {
        setDemoBundles(data.bundles || []);
      } else {
        setError('Failed to load demo bundles');
      }
    } catch (err: any) {
      console.error('Failed to load demo bundles:', err);
      setError(err.message || 'Failed to load demo bundles');
    } finally {
      setLoading(false);
    }
  };

  const handleInstall = async (bundleId: string) => {
    // Auto-generate environment name: demo-{bundle-id}
    const autoEnvName = `demo-${bundleId}`;

    try {
      setInstalling(bundleId);
      setError(null);
      setInstallSuccess(null);

      // Call HTTP API to install demo bundle
      const response = await fetch('http://localhost:8585/api/v1/demo-bundles/install', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          bundle_id: bundleId,
          environment_name: autoEnvName,
        }),
      });

      const data = await response.json();

      if (data.success) {
        setInstallSuccess(bundleId);
        setEnvironmentName(autoEnvName); // Store for display in success message

        // Trigger sync after successful installation
        setSyncEnvironment(autoEnvName);
        setSyncModalOpen(true);
      } else {
        setError(data.error || 'Installation failed');
      }
    } catch (err: any) {
      console.error('Failed to install demo bundle:', err);
      setError(err.message || 'Installation failed');
    } finally {
      setInstalling(null);
    }
  };

  const activeCategory = APP_CATEGORIES.find(cat => cat.id === activeTab);
  const filteredBundles = demoBundles.filter(bundle =>
    bundle.category.toLowerCase() === activeTab.toLowerCase()
  );

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center bg-tokyo-bg">
        <div className="flex items-center gap-3 text-tokyo-blue">
          <Loader className="h-6 w-6 animate-spin" />
          <span className="font-mono">Loading demo bundles...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full overflow-auto bg-tokyo-bg">
      <div className="max-w-7xl mx-auto p-6">
        {/* Header */}
        <div className="mb-8">
          <div className="flex items-center gap-3 mb-4">
            <Play size={36} className="text-tokyo-green" />
            <h1 className="text-3xl font-bold font-mono text-tokyo-blue">Live Demo</h1>
          </div>
          <p className="text-tokyo-comment text-lg">
            Try Station with interactive demo bundles. Each demo includes real agents with mock MCP tools that return realistic fake data.
          </p>
        </div>

        {/* Category Tabs */}
        <div className="mb-8 border-b border-tokyo-dark3">
          <div className="flex gap-1 overflow-x-auto">
            {APP_CATEGORIES.map((category) => (
              <button
                key={category.id}
                onClick={() => setActiveTab(category.id)}
                disabled={!category.available}
                className={`
                  flex items-center gap-2 px-4 py-3 font-mono text-sm whitespace-nowrap
                  transition-all border-b-2 relative
                  ${activeTab === category.id
                    ? `border-${category.color} text-${category.color}`
                    : 'border-transparent text-tokyo-comment hover:text-tokyo-fg'
                  }
                  ${!category.available ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
                `}
              >
                <span className={activeTab === category.id ? `text-${category.color}` : 'text-tokyo-comment'}>
                  {category.icon}
                </span>
                <span>{category.name}</span>
                {!category.available && (
                  <span className="ml-1 text-xs px-2 py-0.5 bg-tokyo-dark2 rounded text-tokyo-comment">
                    Coming Soon
                  </span>
                )}
              </button>
            ))}
          </div>
        </div>

        {/* Error Display */}
        {error && (
          <div className="mb-6 border border-tokyo-red rounded-lg p-4 flex items-start gap-3">
            <AlertCircle className="h-5 w-5 text-tokyo-red flex-shrink-0 mt-0.5" />
            <div className="flex-1">
              <p className="text-tokyo-red font-mono text-sm">{error}</p>
            </div>
            <button
              onClick={() => setError(null)}
              className="text-tokyo-red hover:text-tokyo-red opacity-70 hover:opacity-100"
            >
              ×
            </button>
          </div>
        )}

        {/* Category Description */}
        {activeCategory && (
          <div className={`mb-6 border-l-4 border-${activeCategory.color} bg-tokyo-dark1 rounded-r p-4`}>
            <div className="flex items-start gap-3">
              <span className={`text-${activeCategory.color} mt-1`}>
                {activeCategory.icon}
              </span>
              <div>
                <h2 className={`text-lg font-bold font-mono text-${activeCategory.color} mb-1`}>
                  {activeCategory.name}
                </h2>
                <p className="text-tokyo-fg text-sm">{activeCategory.description}</p>
              </div>
            </div>
          </div>
        )}

        {/* Content Area */}
        {activeCategory?.available ? (
          <>
            {/* Demo Bundles Grid */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              {filteredBundles.map((bundle) => {
                const isInstalling = installing === bundle.id;
                const isSuccess = installSuccess === bundle.id;

                return (
                  <div
                    key={bundle.id}
                    className="bg-tokyo-dark1 border border-tokyo-dark3 rounded-lg p-6 hover:border-tokyo-blue transition-colors"
                  >
                    <div className="flex items-start gap-4 mb-4">
                      <div className="p-3 bg-tokyo-bg rounded">
                        <Package size={32} className="text-tokyo-cyan" />
                      </div>
                      <div className="flex-1">
                        <h3 className="font-semibold font-mono text-tokyo-cyan text-xl mb-1">
                          {bundle.name}
                        </h3>
                        <span className="inline-block text-xs px-2 py-1 border border-tokyo-purple text-tokyo-purple rounded font-mono">
                          {bundle.category}
                        </span>
                      </div>
                    </div>

                    <p className="text-tokyo-fg text-sm mb-6 leading-relaxed">
                      {bundle.description}
                    </p>

                    <div className="space-y-3">
                      <div className="flex items-center justify-between text-xs text-tokyo-comment font-mono">
                        <span>Environment:</span>
                        <span className="text-tokyo-cyan">demo-{bundle.id}</span>
                      </div>

                      {isSuccess ? (
                        <div className="border border-tokyo-green rounded-lg p-3 flex items-center gap-2">
                          <CheckCircle className="h-5 w-5 text-tokyo-green flex-shrink-0" />
                          <div className="flex-1">
                            <p className="text-tokyo-green font-mono text-sm">Installed successfully!</p>
                            <p className="text-tokyo-comment text-xs mt-1 font-mono">
                              Next: Run <code className="bg-tokyo-bg px-1 rounded text-tokyo-green">stn sync demo-{bundle.id}</code>
                            </p>
                          </div>
                        </div>
                      ) : (
                        <button
                          onClick={() => handleInstall(bundle.id)}
                          disabled={isInstalling}
                          className="w-full flex items-center justify-center gap-2 px-4 py-3 bg-tokyo-magenta text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                          {isInstalling ? (
                            <>
                              <Loader className="h-4 w-4 animate-spin" />
                              <span>Installing...</span>
                            </>
                          ) : (
                            <>
                              <Play className="h-4 w-4" />
                              <span>Try Demo</span>
                            </>
                          )}
                        </button>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>

            {/* Help Section */}
            <div className="mt-12 bg-tokyo-dark1 border border-tokyo-dark3 rounded-lg p-6">
              <h2 className="text-xl font-bold font-mono text-tokyo-blue mb-4">How Live Demos Work</h2>
              <div className="space-y-4 text-sm text-tokyo-fg">
                <div className="flex items-start gap-3">
                  <span className="text-tokyo-green font-mono">1.</span>
                  <p>
                    <strong className="text-tokyo-cyan">Click "Try Demo"</strong> - Each bundle will be installed to <code className="bg-tokyo-dark2 px-1 rounded font-mono">demo-{'{bundle-id}'}</code> environment
                  </p>
                </div>
                <div className="flex items-start gap-3">
                  <span className="text-tokyo-green font-mono">2.</span>
                  <p>
                    <strong className="text-tokyo-cyan">Run sync</strong> - After installation, run <code className="bg-tokyo-dark2 px-1 rounded font-mono">stn sync demo-{'{bundle-id}'}</code> to load the agents and mock tools
                  </p>
                </div>
                <div className="flex items-start gap-3">
                  <span className="text-tokyo-green font-mono">3.</span>
                  <p>
                    <strong className="text-tokyo-cyan">Try the agents</strong> - Run agents via CLI or UI to see realistic demo results with mock data
                  </p>
                </div>
              </div>

              <div className="mt-6 bg-tokyo-bg border-l-4 border-tokyo-cyan rounded p-4">
                <p className="text-sm text-tokyo-cyan font-mono">
                  💡 <strong>Note:</strong> Demo bundles use mock MCP tools (like <code className="bg-tokyo-dark2 px-1 rounded">stn mock aws-cost-explorer</code>) that return realistic fake data. Perfect for trying Station without connecting to real services!
                </p>
              </div>
            </div>
          </>
        ) : (
          /* Coming Soon Section */
          <div className="max-w-3xl mx-auto py-12">
            <div className="bg-tokyo-dark1 border border-tokyo-dark3 rounded-lg p-12 text-center">
              <div className="inline-flex items-center justify-center w-20 h-20 rounded-full bg-tokyo-dark2 mb-6">
                <Sparkles size={40} className={`text-${activeCategory?.color || 'tokyo-blue'}`} />
              </div>
              <h2 className="text-2xl font-bold font-mono text-tokyo-blue mb-3">
                Coming Soon
              </h2>
              <p className="text-tokyo-comment text-lg mb-6">
                {activeCategory?.name} demo bundles are being prepared
              </p>
              <p className="text-tokyo-fg text-sm max-w-md mx-auto mb-8">
                {activeCategory?.description}
              </p>

              <div className="bg-tokyo-bg border border-tokyo-dark3 rounded-lg p-6 text-left">
                <h3 className="text-sm font-bold font-mono text-tokyo-cyan mb-3">
                  What's Being Built:
                </h3>
                <ul className="space-y-2 text-sm text-tokyo-fg">
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-green">•</span>
                    <span><strong className="text-tokyo-cyan">Investigations:</strong> Root cause analysis agents for {activeCategory?.name.toLowerCase()} issues</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-green">•</span>
                    <span><strong className="text-tokyo-cyan">Opportunities:</strong> Optimization and improvement recommendation agents</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-green">•</span>
                    <span><strong className="text-tokyo-cyan">Projections:</strong> Forecasting and predictive analysis agents</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-green">•</span>
                    <span><strong className="text-tokyo-cyan">Inventory:</strong> Resource cataloging and tracking agents</span>
                  </li>
                  <li className="flex items-start gap-2">
                    <span className="text-tokyo-green">•</span>
                    <span><strong className="text-tokyo-cyan">Events:</strong> Change tracking and event correlation agents</span>
                  </li>
                </ul>
              </div>

              <div className="mt-8 text-sm text-tokyo-comment font-mono">
                Stay tuned for updates on CloudShip Agent Platform expansion
              </div>
            </div>
          </div>
        )}
      </div>

      <SyncModal
        isOpen={syncModalOpen}
        onClose={() => setSyncModalOpen(false)}
        environment={syncEnvironment}
        onSyncComplete={() => loadDemoBundles()}
      />
    </div>
  );
};
