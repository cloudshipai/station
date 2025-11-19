import React, { useState } from 'react';
import type { TimelineRun } from '../../utils/timelineLayout';

interface TimelineBarProps {
  run: TimelineRun;
  left: number; // Percentage
  width: number; // Percentage
  height: number; // Pixels
  onClick: (runId: number) => void;
  onHover?: (run: TimelineRun | null) => void; // Callback for hover state
  onClickForPanel?: (run: TimelineRun) => void; // Click to pin panel
  isHighlighted?: boolean;
  laneIndex?: number; // Position in the list (0 = first/top)
  totalLanes?: number; // Total number of lanes
}

export const TimelineBar: React.FC<TimelineBarProps> = ({
  run,
  left,
  width,
  height,
  onClick,
  onHover,
  onClickForPanel,
  isHighlighted = false,
  laneIndex = 0,
  totalLanes = 1
}) => {
  const [isHovered, setIsHovered] = useState(false);
  const clickTimeoutRef = React.useRef<NodeJS.Timeout | null>(null);
  
  const handleClick = () => {
    // Use timeout to differentiate single vs double click
    if (clickTimeoutRef.current) {
      // This is a double-click - clear timeout and open modal
      clearTimeout(clickTimeoutRef.current);
      clickTimeoutRef.current = null;
      onClick(run.id); // Open full modal
    } else {
      // This is potentially a single click - wait to see if double-click follows
      clickTimeoutRef.current = setTimeout(() => {
        clickTimeoutRef.current = null;
        // Single click confirmed - pin to panel
        if (onClickForPanel) {
          onClickForPanel(run);
        }
      }, 250);
    }
  };

  const handleMouseEnter = () => {
    console.log('Bar hover enter:', run.id, run.agent_name);
    setIsHovered(true);
    if (onHover) {
      console.log('Calling onHover with run:', run.id);
      onHover(run);
    } else {
      console.log('No onHover callback provided!');
    }
  };

  const handleMouseLeave = () => {
    console.log('Bar hover leave');
    setIsHovered(false);
    if (onHover) {
      onHover(null);
    }
  };

  // Color by status - soft palette for visibility
  const getStatusColor = () => {
    switch (run.status) {
      case 'completed':
        return 'bg-green-500';
      case 'failed':
        return 'bg-red-500';
      case 'running':
        return 'bg-blue-500 animate-pulse';
      default:
        return 'bg-gray-400';
    }
  };

  const formatDuration = (seconds: number) => {
    if (seconds < 1) return `${(seconds * 1000).toFixed(0)}ms`;
    if (seconds < 60) return `${seconds.toFixed(1)}s`;
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}m ${secs}s`;
  };

  const formatTokens = (tokens: number) => {
    if (tokens >= 1000000) return `${(tokens / 1000000).toFixed(2)}M`;
    if (tokens >= 1000) return `${(tokens / 1000).toFixed(1)}K`;
    return tokens.toString();
  };

  const formatCost = (cost: number) => {
    return `$${cost.toFixed(4)}`;
  };

  return (
    <>
      <div
        className={`absolute cursor-pointer transition-all duration-150 rounded shadow-sm ${getStatusColor()} ${
          isHighlighted ? 'ring-2 ring-primary' : ''
        } ${isHovered ? 'opacity-100 z-10 scale-105' : 'opacity-90'}`}
        style={{
          left: `${left}%`,
          width: `${Math.max(width, 0.5)}%`, // Minimum width for visibility
          height: `${height}px`,
          top: '50%',
          transform: 'translateY(-50%)',
          minWidth: '3px'
        }}
        onClick={handleClick}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      />

      {/* Tooltip removed - now using fixed info panel */}
      {false && (
        <div className="absolute z-50 pointer-events-none left-0 bottom-full mb-2">
          <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-lg p-2 text-xs font-mono whitespace-nowrap">
            <div className="font-semibold text-tokyo-green mb-1 text-xs">
              Run #{run.id}
            </div>
            <div className="space-y-0.5 text-tokyo-fg text-xs">
              <div className="flex justify-between gap-4">
                <span className="text-tokyo-comment">Status:</span>
                <span className={
                  run.status === 'completed' ? 'text-tokyo-green' :
                  run.status === 'failed' ? 'text-tokyo-red' :
                  'text-tokyo-blue'
                }>
                  {run.status}
                </span>
              </div>
              <div className="flex justify-between gap-4">
                <span className="text-tokyo-comment">Duration:</span>
                <span>{formatDuration(run.duration_seconds || 0)}</span>
              </div>
              <div className="flex justify-between gap-4">
                <span className="text-tokyo-comment">Started:</span>
                <span>{new Date(run.started_at).toLocaleTimeString()}</span>
              </div>
              {run.total_tokens && (
                <div className="flex justify-between gap-4">
                  <span className="text-tokyo-comment">Tokens:</span>
                  <span>{formatTokens(run.total_tokens)}</span>
                </div>
              )}
              {run.cost && (
                <div className="flex justify-between gap-4">
                  <span className="text-tokyo-comment">Cost:</span>
                  <span>{formatCost(run.cost)}</span>
                </div>
              )}
              {run.error && (
                <div className="mt-2 pt-2 border-t border-tokyo-red/30">
                  <div className="text-tokyo-red text-xs max-w-xs truncate">
                    Error: {run.error}
                  </div>
                </div>
              )}
            </div>
            {/* Tooltip arrow - always pointing down since tooltip is above */}
            <div className="absolute left-4 w-2 h-2 bg-tokyo-bg-dark transform rotate-45 bottom-0 translate-y-1/2 border-r border-b border-tokyo-blue7" />
          </div>
        </div>
      )}
    </>
  );
};
