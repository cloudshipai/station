import React, { useState, useEffect } from 'react';
import { Cloud, CheckCircle, XCircle, AlertCircle } from 'lucide-react';

interface LighthouseStatus {
  connected: boolean;
  registered: boolean;
  last_error: string;
  last_error_time: string;
  last_success: string;
  registration_key: string;
  telemetry_sent: number;
  telemetry_failed: number;
  server_url: string;
}

interface LighthouseResponse {
  status: LighthouseStatus;
  is_healthy: boolean;
  has_error: boolean;
  summary_message: string;
}

interface BundleAuthStatus {
  authenticated: boolean;
  bundleCount: number;
  organization?: string;
}

const CloudShipStatus: React.FC = () => {
  const [status, setStatus] = useState<LighthouseResponse | null>(null);
  const [bundleAuth, setBundleAuth] = useState<BundleAuthStatus | null>(null);
  const [showTooltip, setShowTooltip] = useState(false);

  useEffect(() => {
    const fetchStatus = async () => {
      try {
        const response = await fetch('/api/v1/lighthouse/status');
        if (response.ok) {
          const data = await response.json();
          setStatus(data);
        }
      } catch (error) {
        console.error('Failed to fetch lighthouse status:', error);
      }
    };

    const fetchBundleAuth = async () => {
      try {
        const response = await fetch('/api/v1/cloudship/bundles');
        if (response.ok) {
          const data = await response.json();
          if (data.success && data.bundles) {
            setBundleAuth({
              authenticated: true,
              bundleCount: data.bundles.length,
              organization: data.bundles[0]?.organization
            });
          } else {
            setBundleAuth({ authenticated: false, bundleCount: 0 });
          }
        } else {
          setBundleAuth({ authenticated: false, bundleCount: 0 });
        }
      } catch (error) {
        console.error('Failed to fetch bundle auth status:', error);
        setBundleAuth({ authenticated: false, bundleCount: 0 });
      }
    };

    // Initial fetch
    fetchStatus();
    fetchBundleAuth();

    // Poll every 5 seconds for lighthouse, 30 seconds for bundles
    const lighthouseInterval = setInterval(fetchStatus, 5000);
    const bundleInterval = setInterval(fetchBundleAuth, 30000);

    return () => {
      clearInterval(lighthouseInterval);
      clearInterval(bundleInterval);
    };
  }, []);

  const isAuthenticated = bundleAuth?.authenticated || status?.is_healthy;
  
  const getStatusColor = () => {
    if (isAuthenticated) return 'bg-tokyo-green';
    if (status?.has_error) return 'bg-tokyo-red';
    if (status?.status.connected) return 'bg-tokyo-yellow';
    return 'bg-tokyo-dark5';
  };

  const getStatusText = () => {
    if (bundleAuth?.authenticated) return 'Logged In';
    if (status?.is_healthy) return 'Connected';
    if (status?.has_error) return 'Error';
    if (status?.status.connected) return 'Auth Issue';
    return 'Not Connected';
  };

  const StatusIcon = isAuthenticated ? CheckCircle : 
                     status?.has_error ? XCircle : 
                     status?.status.connected ? AlertCircle : Cloud;

  return (
    <div
      className="relative cursor-pointer"
      onMouseEnter={() => setShowTooltip(true)}
      onMouseLeave={() => setShowTooltip(false)}
    >
      {/* CloudShip Badge */}
      <div className={`flex items-center space-x-2 px-3 py-2 rounded-lg transition-colors ${
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

      {/* Tooltip */}
      {showTooltip && (
        <div className="absolute bottom-full left-0 mb-2 p-3 bg-tokyo-dark2 border border-tokyo-dark4 rounded-lg shadow-lg z-50 min-w-72">
          <div className="text-xs text-tokyo-fg space-y-2">
            {/* Header */}
            <div className="flex items-center space-x-2 pb-2 border-b border-tokyo-dark4">
              <Cloud size={16} className="text-tokyo-blue" />
              <span className="font-semibold text-tokyo-blue">CloudShip Status</span>
            </div>

            {/* Bundle Access */}
            {bundleAuth && (
              <div className="space-y-1">
                <div className="flex justify-between items-center">
                  <span className="text-tokyo-comment">Bundle Access:</span>
                  <span className={bundleAuth.authenticated ? 'text-tokyo-green flex items-center space-x-1' : 'text-tokyo-red'}>
                    {bundleAuth.authenticated ? (
                      <>
                        <CheckCircle size={12} />
                        <span>Authenticated</span>
                      </>
                    ) : (
                      <>
                        <XCircle size={12} />
                        <span>Not Authenticated</span>
                      </>
                    )}
                  </span>
                </div>
                {bundleAuth.authenticated && bundleAuth.bundleCount > 0 && (
                  <div className="flex justify-between">
                    <span className="text-tokyo-comment">Available Bundles:</span>
                    <span className="text-tokyo-purple">{bundleAuth.bundleCount}</span>
                  </div>
                )}
                {bundleAuth.organization && (
                  <div className="flex justify-between">
                    <span className="text-tokyo-comment">Organization:</span>
                    <span className="text-tokyo-cyan">{bundleAuth.organization}</span>
                  </div>
                )}
              </div>
            )}

            {/* Lighthouse Status */}
            {status && (
              <div className="border-t border-tokyo-dark4 pt-2 space-y-1">
                <div className="text-tokyo-comment text-[10px] uppercase tracking-wide mb-1">Lighthouse</div>
                <div className="flex justify-between">
                  <span className="text-tokyo-comment">Status:</span>
                  <span className={status.is_healthy ? 'text-tokyo-green' : 'text-tokyo-red'}>
                    {status.is_healthy ? 'Healthy' : 'Disconnected'}
                  </span>
                </div>

                {status.status.connected && (
                  <div className="flex justify-between">
                    <span className="text-tokyo-comment">Registered:</span>
                    <span className={status.status.registered ? 'text-tokyo-green' : 'text-tokyo-yellow'}>
                      {status.status.registered ? 'Yes' : 'No'}
                    </span>
                  </div>
                )}

                {status.status.telemetry_sent > 0 && (
                  <div className="flex justify-between">
                    <span className="text-tokyo-comment">Telemetry:</span>
                    <span className="text-tokyo-green">{status.status.telemetry_sent} sent</span>
                  </div>
                )}
              </div>
            )}

            {/* Error Display */}
            {status?.has_error && status.status.last_error && (
              <div className="border-t border-tokyo-dark4 pt-2">
                <div className="text-tokyo-red text-xs bg-tokyo-dark1 p-2 rounded border-l-2 border-tokyo-red">
                  {status.status.last_error}
                </div>
              </div>
            )}

            {/* Help text when not authenticated */}
            {!isAuthenticated && (
              <div className="border-t border-tokyo-dark4 pt-2">
                <div className="text-tokyo-comment text-[10px]">
                  Run <code className="bg-tokyo-dark3 px-1 rounded">stn auth login</code> to authenticate
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default CloudShipStatus;
