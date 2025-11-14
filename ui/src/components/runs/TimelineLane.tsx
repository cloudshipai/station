import React, { useState } from 'react';
import { TimelineBar } from './TimelineBar';
import type { TimelineRun, AgentLaneStats, TimeRange } from '../../utils/timelineLayout';
import { calculateBarLayout, calculateBarHeight } from '../../utils/timelineLayout';

interface TimelineLaneProps {
  agentName: string;
  runs: TimelineRun[];
  timeBounds: TimeRange;
  densityMetric: 'tokens' | 'cost';
  maxMetricValue: number;
  stats?: AgentLaneStats;
  showP95?: boolean;
  onRunClick: (runId: number) => void;
  onRunHover?: (run: TimelineRun | null) => void;
  onRunClickForPanel?: (run: TimelineRun) => void;
  highlightedRuns?: Set<number>;
  laneIndex: number;
  totalLanes: number;
}

export const TimelineLane: React.FC<TimelineLaneProps> = ({
  agentName,
  runs,
  timeBounds,
  densityMetric,
  maxMetricValue,
  stats,
  showP95,
  onRunClick,
  onRunHover,
  onRunClickForPanel,
  highlightedRuns,
  laneIndex,
  totalLanes
}) => {
  const [isHovered, setIsHovered] = useState(false);

  const formatDuration = (seconds: number) => {
    if (seconds < 1) return `${(seconds * 1000).toFixed(0)}ms`;
    if (seconds < 60) return `${seconds.toFixed(1)}s`;
    return `${(seconds / 60).toFixed(1)}m`;
  };

  const formatNumber = (num: number) => {
    if (num >= 1000000) return `${(num / 1000000).toFixed(2)}M`;
    if (num >= 1000) return `${(num / 1000).toFixed(1)}K`;
    return num.toFixed(0);
  };

  return (
    <div className="border-b border-tokyo-blue7 hover:bg-tokyo-bg-highlight/30 transition-colors relative overflow-visible">
      {/* Lane Header */}
      <div
        className="flex items-center gap-4 p-3 bg-tokyo-bg-dark cursor-pointer relative"
        onMouseEnter={() => setIsHovered(true)}
        onMouseLeave={() => setIsHovered(false)}
      >
        <div className="w-48 flex-shrink-0">
          <div className="font-mono text-sm text-tokyo-green truncate">
            {agentName}
          </div>
          <div className="font-mono text-xs text-tokyo-comment">
            {runs.length} run{runs.length !== 1 ? 's' : ''}
          </div>
        </div>

        {/* Lane stats tooltip DISABLED - using right panel instead */}
        {false && isHovered && stats && (
          <div className="hidden"></div>
        )}

        {/* Timeline Area */}
        <div className="flex-1 relative h-16">
          {/* P95 Duration Overlay */}
          {showP95 && stats && stats.p95_duration > 0 && (
            <div className="absolute inset-0 pointer-events-none">
              <div
                className="absolute left-0 h-full border-r-2 border-dashed border-tokyo-yellow/40"
                style={{
                  width: `${((stats.p95_duration * 1000) / (timeBounds.end - timeBounds.start)) * 100}%`
                }}
              />
              <div
                className="absolute top-0 text-xs text-tokyo-yellow/70 font-mono"
                style={{
                  left: `${((stats.p95_duration * 1000) / (timeBounds.end - timeBounds.start)) * 100}%`,
                  transform: 'translateX(-50%)'
                }}
              >
                p95
              </div>
            </div>
          )}

          {/* Timeline Bars */}
          {runs.map(run => {
            const layout = calculateBarLayout(run, timeBounds);
            const height = calculateBarHeight(run, densityMetric, maxMetricValue);
            return (
              <TimelineBar
                key={run.id}
                run={run}
                left={layout.left}
                width={layout.width}
                height={height}
                onClick={onRunClick}
                onHover={onRunHover}
                onClickForPanel={onRunClickForPanel}
                isHighlighted={highlightedRuns?.has(run.id)}
                laneIndex={laneIndex}
                totalLanes={totalLanes}
              />
            );
          })}
        </div>
      </div>
    </div>
  );
};
