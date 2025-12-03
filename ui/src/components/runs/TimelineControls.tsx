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
    <div className="flex items-center gap-4">
      {/* Time Range Selector */}
      <div className="flex items-center gap-2">
        <Clock className="h-4 w-4 text-gray-500" />
        <span className="text-xs text-gray-600 font-medium">Time:</span>
        <div className="flex gap-1">
          {TIME_RANGE_PRESETS.map(range => {
            const isSelected = selectedTimeRange?.value === range.value;
            return (
              <button
                key={range.value}
                onClick={() => onTimeRangeChange(range)}
                className={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                  isSelected
                    ? 'bg-blue-600 text-white shadow-sm'
                    : 'bg-gray-100 text-gray-600 hover:text-gray-900 hover:bg-gray-200'
                }`}
              >
                {range.label}
              </button>
            );
          })}
        </div>
      </div>

      {/* Vertical Divider */}
      <div className="h-6 w-px bg-gray-200" />

      {/* Density Metric Selector */}
      <div className="flex items-center gap-2">
        <BarChart2 className="h-4 w-4 text-gray-500" />
        <span className="text-xs text-gray-600 font-medium">Density:</span>
        <div className="flex gap-1">
          <button
            onClick={() => onDensityMetricChange('tokens')}
            className={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
              densityMetric === 'tokens'
                ? 'bg-green-600 text-white shadow-sm'
                : 'bg-gray-100 text-gray-600 hover:text-gray-900 hover:bg-gray-200'
            }`}
          >
            Tokens
          </button>
          <button
            onClick={() => onDensityMetricChange('cost')}
            className={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
              densityMetric === 'cost'
                ? 'bg-purple-600 text-white shadow-sm'
                : 'bg-gray-100 text-gray-600 hover:text-gray-900 hover:bg-gray-200'
            }`}
          >
            Cost
          </button>
        </div>
      </div>

      {/* Vertical Divider */}
      <div className="h-6 w-px bg-gray-200" />

      {/* P95 Overlay Toggle */}
      <div className="flex items-center gap-2">
        <TrendingUp className="h-4 w-4 text-gray-500" />
        <label className="flex items-center gap-2 cursor-pointer group relative">
          <input
            type="checkbox"
            checked={showP95}
            onChange={(e) => onShowP95Change(e.target.checked)}
            className="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500 focus:ring-offset-0"
          />
          <span className="text-xs text-gray-600 font-medium group-hover:text-gray-900">
            p95 Duration
          </span>
          <div className="relative group/tooltip">
            <span className="text-xs text-gray-400 hover:text-gray-600 cursor-help">ⓘ</span>
            <div className="absolute left-0 top-6 hidden group-hover/tooltip:block w-64 p-3 bg-gray-900 text-white rounded-lg shadow-lg z-50 text-xs">
              Shows a vertical dashed line at the 95th percentile duration for each agent.
              <span className="text-yellow-400 block mt-1">95% of runs complete faster than this threshold.</span>
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
