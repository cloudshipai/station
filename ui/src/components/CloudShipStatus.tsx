import React, { useState, useEffect } from 'react';

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

const CloudShipStatus: React.FC = () => {
  const [status, setStatus] = useState<LighthouseResponse | null>(null);
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

    // Initial fetch
    fetchStatus();

    // Poll every 5 seconds
    const interval = setInterval(fetchStatus, 5000);

    return () => clearInterval(interval);
  }, []);

  if (!status) {
    return (
      <div className="flex items-center space-x-2 text-tokyo-dark5">
        <div className="w-3 h-3 rounded-full bg-tokyo-dark5 opacity-50"></div>
        <span className="text-xs">CloudShip</span>
      </div>
    );
  }

  const getStatusColor = () => {
    if (status.is_healthy) return 'bg-tokyo-green';
    if (status.has_error) return 'bg-tokyo-red';
    if (status.status.connected) return 'bg-tokyo-yellow';
    return 'bg-tokyo-dark5';
  };

  const getStatusText = () => {
    if (status.is_healthy) return 'Connected';
    if (status.has_error) return 'Error';
    if (status.status.connected) return 'Auth Issue';
    return 'Disconnected';
  };

  return (
    <div
      className="relative flex items-center space-x-2 cursor-pointer"
      onMouseEnter={() => setShowTooltip(true)}
      onMouseLeave={() => setShowTooltip(false)}
    >
      {/* Status Icon */}
      <div className="relative">
        <div
          className={`w-3 h-3 rounded-full ${getStatusColor()} ${
            status.is_healthy ? 'animate-pulse' : ''
          }`}
        ></div>
        {/* Pulse ring for healthy status */}
        {status.is_healthy && (
          <div className="absolute inset-0 w-3 h-3 rounded-full bg-tokyo-green animate-ping opacity-20"></div>
        )}
      </div>

      {/* Label */}
      <span className="text-xs text-tokyo-fg font-mono">CloudShip</span>

      {/* Tooltip */}
      {showTooltip && (
        <div className="absolute bottom-full left-0 mb-2 p-3 bg-tokyo-dark2 border border-tokyo-dark4 rounded-lg shadow-lg z-50 min-w-64">
          <div className="text-xs text-tokyo-fg space-y-1">
            <div className="font-semibold text-tokyo-blue">{status.summary_message}</div>

            <div className="border-t border-tokyo-dark4 pt-2 mt-2">
              <div className="flex justify-between">
                <span className="text-tokyo-comment">Status:</span>
                <span className={status.is_healthy ? 'text-tokyo-green' : 'text-tokyo-red'}>
                  {getStatusText()}
                </span>
              </div>

              <div className="flex justify-between">
                <span className="text-tokyo-comment">Connected:</span>
                <span className={status.status.connected ? 'text-tokyo-green' : 'text-tokyo-red'}>
                  {status.status.connected ? 'Yes' : 'No'}
                </span>
              </div>

              <div className="flex justify-between">
                <span className="text-tokyo-comment">Registered:</span>
                <span className={status.status.registered ? 'text-tokyo-green' : 'text-tokyo-red'}>
                  {status.status.registered ? 'Yes' : 'No'}
                </span>
              </div>

              {status.status.telemetry_sent > 0 && (
                <div className="flex justify-between">
                  <span className="text-tokyo-comment">Telemetry Sent:</span>
                  <span className="text-tokyo-green">{status.status.telemetry_sent}</span>
                </div>
              )}

              {status.status.telemetry_failed > 0 && (
                <div className="flex justify-between">
                  <span className="text-tokyo-comment">Failed:</span>
                  <span className="text-tokyo-red">{status.status.telemetry_failed}</span>
                </div>
              )}
            </div>

            {status.has_error && status.status.last_error && (
              <div className="border-t border-tokyo-dark4 pt-2 mt-2">
                <div className="text-tokyo-comment text-xs mb-1">Error:</div>
                <div className="text-tokyo-red text-xs bg-tokyo-dark1 p-2 rounded border-l-2 border-tokyo-red">
                  {status.status.last_error}
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
