import React, { useState, useEffect } from 'react';
import { Play, Package, Download, CheckCircle, AlertCircle, Loader } from 'lucide-react';

interface DemoBundle {
  id: string;
  name: string;
  description: string;
  category: string;
  size: number;
}

export const LiveDemoPage: React.FC = () => {
  const [demoBundles, setDemoBundles] = useState<DemoBundle[]>([]);
  const [loading, setLoading] = useState(true);
  const [installing, setInstalling] = useState<string | null>(null);
  const [installSuccess, setInstallSuccess] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [environmentName, setEnvironmentName] = useState('');

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

        {/* Demo Bundles Grid */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {demoBundles.map((bundle) => {
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
      </div>
    </div>
  );
};
