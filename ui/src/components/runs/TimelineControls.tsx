import React from 'react';
import { Clock, BarChart2, TrendingUp } from 'lucide-react';
import type { TimeRangePreset } from '../../utils/timelineLayout';
import { TIME_RANGE_PRESETS } from '../../utils/timelineLayout';

interface TimelineControlsProps {
  selectedTimeRange: TimeRangePreset;
  onTimeRangeChange: (range: TimeRangePreset) => void;
  densityMetric: 'tokens' | 'cost';
  onDensityMetricChange: (metric: 'tokens' | 'cost') => void;
  showP95: boolean;
  onShowP95Change: (show: boolean) => void;
}

export const TimelineControls: React.FC<TimelineControlsProps> = ({
  selectedTimeRange,
  onTimeRangeChange,
  densityMetric,
  onDensityMetricChange,
  showP95,
  onShowP95Change
}) => {
  return (
    <div className="flex items-center gap-4 p-4 bg-tokyo-bg-dark border-b border-tokyo-blue7">
      {/* Time Range Selector */}
      <div className="flex items-center gap-2">
        <Clock className="h-4 w-4 text-tokyo-comment" />
        <span className="text-xs text-tokyo-comment font-mono">Time:</span>
        <div className="flex gap-1">
          {TIME_RANGE_PRESETS.map(range => {
            const isSelected = selectedTimeRange?.value === range.value;
            return (
              <button
                key={range.value}
                onClick={() => onTimeRangeChange(range)}
                className={`px-3 py-1 text-xs font-mono rounded transition-colors ${
                  isSelected
                    ? 'bg-tokyo-blue text-tokyo-bg font-semibold'
                    : 'bg-tokyo-bg text-tokyo-comment hover:text-tokyo-blue hover:bg-tokyo-bg-highlight'
                }`}
              >
                {range.label}
              </button>
            );
          })}
        </div>
      </div>

      {/* Vertical Divider */}
      <div className="h-6 w-px bg-tokyo-blue7" />

      {/* Density Metric Selector */}
      <div className="flex items-center gap-2">
        <BarChart2 className="h-4 w-4 text-tokyo-comment" />
        <span className="text-xs text-tokyo-comment font-mono">Density:</span>
        <div className="flex gap-1">
          <button
            onClick={() => onDensityMetricChange('tokens')}
            className={`px-3 py-1 text-xs font-mono rounded transition-colors ${
              densityMetric === 'tokens'
                ? 'bg-tokyo-green text-tokyo-bg'
                : 'bg-tokyo-bg text-tokyo-comment hover:text-tokyo-green hover:bg-tokyo-bg-highlight'
            }`}
          >
            Tokens
          </button>
          <button
            onClick={() => onDensityMetricChange('cost')}
            className={`px-3 py-1 text-xs font-mono rounded transition-colors ${
              densityMetric === 'cost'
                ? 'bg-tokyo-purple text-tokyo-bg'
                : 'bg-tokyo-bg text-tokyo-comment hover:text-tokyo-purple hover:bg-tokyo-bg-highlight'
            }`}
          >
            Cost
          </button>
        </div>
      </div>

      {/* Vertical Divider */}
      <div className="h-6 w-px bg-tokyo-blue7" />

      {/* P95 Overlay Toggle */}
      <div className="flex items-center gap-2">
        <TrendingUp className="h-4 w-4 text-tokyo-comment" />
        <label className="flex items-center gap-2 cursor-pointer group relative">
          <input
            type="checkbox"
            checked={showP95}
            onChange={(e) => onShowP95Change(e.target.checked)}
            className="w-4 h-4 rounded border-tokyo-blue7 bg-tokyo-bg text-tokyo-yellow focus:ring-tokyo-yellow focus:ring-offset-0"
          />
          <span className="text-xs text-tokyo-comment font-mono group-hover:text-tokyo-fg">
            p95 Duration
          </span>
          <div className="relative group/tooltip">
            <span className="text-xs text-tokyo-comment hover:text-tokyo-cyan cursor-help">ⓘ</span>
            <div className="absolute left-0 top-6 hidden group-hover/tooltip:block w-64 p-2 bg-tokyo-bg-dark border border-tokyo-blue7 rounded shadow-lg z-50 text-xs text-tokyo-fg font-mono">
              Shows a vertical dashed line at the 95th percentile duration for each agent. 
              <span className="text-tokyo-yellow block mt-1">95% of runs complete faster than this threshold.</span>
            </div>
          </div>
        </label>
      </div>

      {/* Legend */}
      <div className="ml-auto flex items-center gap-4">
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-tokyo-green rounded" />
          <span className="text-xs text-tokyo-comment font-mono">Completed</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-tokyo-red rounded" />
          <span className="text-xs text-tokyo-comment font-mono">Failed</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-tokyo-blue rounded animate-pulse" />
          <span className="text-xs text-tokyo-comment font-mono">Running</span>
        </div>
      </div>
    </div>
  );
};
