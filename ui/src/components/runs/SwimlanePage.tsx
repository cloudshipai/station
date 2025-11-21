import React, { useState, useMemo } from 'react';
import { AlertCircle, X, Pin, List, GitBranch, BarChart3 } from 'lucide-react';
import { TimelineControls } from './TimelineControls';
import { TimelineLane } from './TimelineLane';
import type { TimelineRun, TimeRangePreset } from '../../utils/timelineLayout';
import {
  groupRunsByAgent,
  calculateTimeBounds,
  calculateLaneStats,
  filterRunsByTimeRange,
  findRunRelationships,
  formatTime,
  TIME_RANGE_PRESETS
} from '../../utils/timelineLayout';

interface SwimlanePageProps {
  runs: TimelineRun[];
  onRunClick: (runId: number) => void;
  activeView?: 'list' | 'timeline' | 'stats';
  onViewChange?: (view: 'list' | 'timeline' | 'stats') => void;
  currentEnvironment?: any;
  environments?: any[];
  onEnvironmentChange?: (envName: string) => void;
}

export const SwimlanePage: React.FC<SwimlanePageProps> = ({ 
  runs, 
  onRunClick, 
  activeView = 'timeline', 
  onViewChange,
  currentEnvironment,
  environments = [],
  onEnvironmentChange
}) => {
  const [selectedTimeRange, setSelectedTimeRange] = useState<TimeRangePreset>(TIME_RANGE_PRESETS[1]); // Last Day
  const [densityMetric, setDensityMetric] = useState<'tokens' | 'cost'>('tokens');
  const [showP95, setShowP95] = useState(false);
  const [highlightedRuns, setHighlightedRuns] = useState<Set<number>>(new Set());
  const [hoveredRun, setHoveredRun] = useState<TimelineRun | null>(null);
  const [pinnedRun, setPinnedRun] = useState<TimelineRun | null>(null);
  const closeTimeoutRef = React.useRef<NodeJS.Timeout | null>(null);

  // Use pinned run if available, otherwise use hovered run
  const displayedRun = pinnedRun || hoveredRun;

  const handleRunHover = (run: TimelineRun | null) => {
    // Only update hover if nothing is pinned
    if (!pinnedRun) {
      console.log('SwimlanePage received hover:', run?.id || 'null');
      
      // Clear any pending close timeout
      if (closeTimeoutRef.current) {
        clearTimeout(closeTimeoutRef.current);
        closeTimeoutRef.current = null;
      }
      
      if (run) {
        // Show preview on hover
        setHoveredRun(run);
      } else {
        // Delay closing to allow mouse to reach panel
        closeTimeoutRef.current = setTimeout(() => {
          setHoveredRun(null);
        }, 200);
      }
    }
  };

  const handleRunClickForPanel = (run: TimelineRun) => {
    console.log('Run clicked - pinning:', run.id);
    // Toggle pin: if clicking the same run, unpin it
    if (pinnedRun?.id === run.id) {
      setPinnedRun(null);
    } else {
      // Pin this run to keep panel open
      setPinnedRun(run);
      setHoveredRun(null);
    }
  };

  const handleClosePanel = () => {
    console.log('Close button clicked');
    setPinnedRun(null);
    setHoveredRun(null);
  };

  // Filter runs by time range
  const filteredRuns = useMemo(() => {
    const filtered = filterRunsByTimeRange(runs, selectedTimeRange);
    console.log(`Timeline: ${selectedTimeRange.label} - ${runs.length} total runs -> ${filtered.length} filtered runs`);
    return filtered;
  }, [runs, selectedTimeRange]);

  // Group runs into lanes
  const lanes = useMemo(() => {
    return groupRunsByAgent(filteredRuns);
  }, [filteredRuns]);

  // Calculate time bounds
  const timeBounds = useMemo(() => {
    return calculateTimeBounds(filteredRuns);
  }, [filteredRuns]);

  // Calculate lane statistics
  const laneStats = useMemo(() => {
    const stats = calculateLaneStats(filteredRuns);
    const statsMap = new Map(stats.map(s => [s.agent_name, s]));
    return statsMap;
  }, [filteredRuns]);

  // Calculate max metric value for bar height normalization
  const maxMetricValue = useMemo(() => {
    return Math.max(
      ...filteredRuns.map(r => 
        densityMetric === 'tokens' 
          ? (r.total_tokens || 0) 
          : (r.cost || 0)
      ),
      1 // Prevent division by zero
    );
  }, [filteredRuns, densityMetric]);

  // Find parent-child relationships
  const relationships = useMemo(() => {
    return findRunRelationships(filteredRuns);
  }, [filteredRuns]);

  // Handle run click with relationship highlighting
  const handleRunClick = (runId: number) => {
    // Highlight related runs (parent and children)
    const related = new Set<number>([runId]);
    relationships.forEach(({ parent, child }) => {
      if (parent.id === runId) related.add(child.id);
      if (child.id === runId) related.add(parent.id);
    });
    setHighlightedRuns(related);
    
    // Open run details
    onRunClick(runId);
  };

  if (runs.length === 0) {
    return (
      <div className="h-full flex flex-col bg-gray-50">
        {/* Header with view tabs on left */}
        <div className="flex items-center gap-4 p-4 border-b border-gray-200 bg-white">
          <h1 className="text-xl font-semibold text-gray-900">Agent Runs</h1>
          
          {/* Environment Selector */}
          {currentEnvironment && environments.length > 0 && onEnvironmentChange && (
            <>
              <div className="h-4 w-px bg-gray-300"></div>
              <select
                value={currentEnvironment?.name?.toLowerCase() || ''}
                onChange={(e) => onEnvironmentChange(e.target.value)}
                className="px-3 py-1.5 bg-white border border-gray-300 text-gray-900 text-sm rounded-lg hover:border-gray-400 focus:ring-2 focus:ring-gray-900/10 focus:border-gray-400 transition-colors"
              >
                {environments.map((environment: any) => (
                  <option key={environment.id} value={environment.name.toLowerCase()}>
                    {environment.name}
                  </option>
                ))}
              </select>
            </>
          )}
          
          {onViewChange && (
            <div className="flex bg-gray-100 rounded-lg p-1">
              <button
                onClick={() => onViewChange('list')}
                className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                  activeView === 'list'
                    ? 'bg-white text-primary shadow-sm'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                <List className="h-4 w-4" />
                List
              </button>
              <button
                onClick={() => onViewChange('timeline')}
                className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                  activeView === 'timeline'
                    ? 'bg-white text-primary shadow-sm'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                <GitBranch className="h-4 w-4" />
                Timeline
              </button>
              <button
                onClick={() => onViewChange('stats')}
                className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                  activeView === 'stats'
                    ? 'bg-white text-primary shadow-sm'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                <BarChart3 className="h-4 w-4" />
                Stats
              </button>
            </div>
          )}
        </div>
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center">
            <AlertCircle className="h-16 w-16 text-gray-300 mx-auto mb-4" />
            <div className="text-gray-900 text-lg mb-2">No runs to display</div>
            <div className="text-gray-500 text-sm">
              Agent execution runs will appear here in a timeline view
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (filteredRuns.length === 0) {
    return (
      <div className="h-full flex flex-col bg-gray-50">
        {/* Header with view tabs on left */}
        <div className="flex items-center gap-4 p-4 border-b border-gray-200 bg-white">
          <h1 className="text-xl font-semibold text-gray-900">Agent Runs</h1>
          
          {/* Environment Selector */}
          {currentEnvironment && environments.length > 0 && onEnvironmentChange && (
            <>
              <div className="h-4 w-px bg-gray-300"></div>
              <select
                value={currentEnvironment?.name?.toLowerCase() || ''}
                onChange={(e) => onEnvironmentChange(e.target.value)}
                className="px-3 py-1.5 bg-white border border-gray-300 text-gray-900 text-sm rounded-lg hover:border-gray-400 focus:ring-2 focus:ring-gray-900/10 focus:border-gray-400 transition-colors"
              >
                {environments.map((environment: any) => (
                  <option key={environment.id} value={environment.name.toLowerCase()}>
                    {environment.name}
                  </option>
                ))}
              </select>
            </>
          )}
          
          {onViewChange && (
            <div className="flex bg-gray-100 rounded-lg p-1">
              <button
                onClick={() => onViewChange('list')}
                className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                  activeView === 'list'
                    ? 'bg-white text-primary shadow-sm'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                <List className="h-4 w-4" />
                List
              </button>
              <button
                onClick={() => onViewChange('timeline')}
                className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                  activeView === 'timeline'
                    ? 'bg-white text-primary shadow-sm'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                <GitBranch className="h-4 w-4" />
                Timeline
              </button>
              <button
                onClick={() => onViewChange('stats')}
                className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                  activeView === 'stats'
                    ? 'bg-white text-primary shadow-sm'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                <BarChart3 className="h-4 w-4" />
                Stats
              </button>
            </div>
          )}
        </div>
        <TimelineControls
          selectedTimeRange={selectedTimeRange}
          onTimeRangeChange={setSelectedTimeRange}
          densityMetric={densityMetric}
          onDensityMetricChange={setDensityMetric}
          showP95={showP95}
          onShowP95Change={setShowP95}
        />
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center">
            <AlertCircle className="h-12 w-12 text-gray-300 mx-auto mb-3" />
            <div className="text-gray-900 mb-2">No runs in selected time range</div>
            <div className="text-gray-500 text-sm">
              Try selecting a different time range
            </div>
          </div>
        </div>
      </div>
    );
  }

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
    <div className="h-full flex flex-col bg-gray-50">
      {/* Header with view tabs on left */}
      <div className="flex items-center gap-4 p-4 border-b border-gray-200 bg-white">
        <h1 className="text-xl font-semibold text-gray-900">Agent Runs</h1>
        {onViewChange && (
          <div className="flex bg-gray-100 rounded-lg p-1">
            <button
              onClick={() => onViewChange('list')}
              className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                activeView === 'list'
                  ? 'bg-white text-primary shadow-sm'
                  : 'text-gray-600 hover:text-gray-900'
              }`}
            >
              <List className="h-4 w-4" />
              List
            </button>
            <button
              onClick={() => onViewChange('timeline')}
              className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                activeView === 'timeline'
                  ? 'bg-white text-primary shadow-sm'
                  : 'text-gray-600 hover:text-gray-900'
              }`}
            >
              <GitBranch className="h-4 w-4" />
              Timeline
            </button>
            <button
              onClick={() => onViewChange('stats')}
              className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                activeView === 'stats'
                  ? 'bg-white text-primary shadow-sm'
                  : 'text-gray-600 hover:text-gray-900'
              }`}
            >
              <BarChart3 className="h-4 w-4" />
              Stats
            </button>
          </div>
        )}
      </div>

      {/* Main timeline content area */}
      <div className="flex-1 flex bg-white relative overflow-hidden">
        {/* Main content - always has space for right panel */}
        <div className="flex-1 flex flex-col mr-80">
          {/* Environment Selector and Controls */}
          <div className="flex items-center justify-between px-4 py-3 bg-white border-b border-gray-200">
            <select
              value={currentEnvironment?.name?.toLowerCase() || ''}
              onChange={(e) => onEnvironmentChange(e.target.value)}
              className="px-3 py-1.5 text-sm border border-gray-300 rounded-md bg-white text-gray-900 focus:outline-none focus:ring-2 focus:ring-cyan-500"
            >
              {environments.map((env) => (
                <option key={env.id} value={env.name.toLowerCase()}>
                  {env.name}
                </option>
              ))}
            </select>
            <TimelineControls
              selectedTimeRange={selectedTimeRange}
              onTimeRangeChange={setSelectedTimeRange}
              densityMetric={densityMetric}
              onDensityMetricChange={setDensityMetric}
              showP95={showP95}
              onShowP95Change={setShowP95}
            />
          </div>

      {/* Time Axis */}
      <div className="flex items-center gap-4 px-4 py-2 bg-gray-50 border-b border-gray-200">
        <div className="w-48 flex-shrink-0">
          <div className="text-xs text-gray-600">
            {filteredRuns.length} run{filteredRuns.length !== 1 ? 's' : ''} • {lanes.size} agent{lanes.size !== 1 ? 's' : ''}
          </div>
        </div>
        <div className="flex-1 flex justify-between text-xs text-gray-500">
          <span>{formatTime(timeBounds.start, 'medium')}</span>
          <span>{formatTime((timeBounds.start + timeBounds.end) / 2, 'medium')}</span>
          <span>{formatTime(timeBounds.end, 'medium')}</span>
        </div>
      </div>

      {/* Swimlanes */}
      <div className="flex-1 overflow-y-auto overflow-x-hidden">
        {Array.from(lanes.entries())
          .sort(([nameA], [nameB]) => nameA.localeCompare(nameB))
          .map(([agentName, laneRuns], index, arr) => (
            <TimelineLane
              key={agentName}
              agentName={agentName}
              runs={laneRuns}
              timeBounds={timeBounds}
              densityMetric={densityMetric}
              maxMetricValue={maxMetricValue}
              stats={laneStats.get(agentName)}
              showP95={showP95}
              onRunClick={handleRunClick}
              onRunHover={handleRunHover}
              onRunClickForPanel={handleRunClickForPanel}
              highlightedRuns={highlightedRuns}
              laneIndex={index}
              totalLanes={arr.length}
            />
          ))}
      </div>

        {/* Parent-Child Relationship Indicator */}
        {relationships.length > 0 && (
          <div className="p-2 bg-gray-50 border-t border-gray-200 text-xs text-gray-600 text-center">
            {relationships.length} parent-child relationship{relationships.length !== 1 ? 's' : ''} detected
            <span className="ml-2 text-cyan-600">(click runs to highlight)</span>
          </div>
        )}
      </div>

      {/* Right Side Panel - Always visible */}
      <div 
        className="fixed right-0 top-0 h-full w-80 bg-white border-l-2 border-gray-200 shadow-2xl z-50"
        onMouseEnter={() => {
          console.log('Panel mouse enter - clearing close timeout');
          // Clear any pending close timeout when mouse enters panel
          if (closeTimeoutRef.current) {
            clearTimeout(closeTimeoutRef.current);
            closeTimeoutRef.current = null;
          }
        }}
        onMouseLeave={() => {
          // Only close on mouse leave if not pinned
          if (!pinnedRun) {
            console.log('Panel mouse leave - closing (not pinned)');
            setHoveredRun(null);
          } else {
            console.log('Panel mouse leave - keeping open (pinned)');
          }
        }}
      >
        {displayedRun ? (
          <div className="h-full flex flex-col p-6">
            {/* Header with Close Button */}
            <div className="mb-6 relative">
              <div className="flex items-center justify-between mb-1">
                <div className="text-lg font-semibold text-gray-900">
                  Run #{displayedRun.id}
                </div>
                <div className="flex items-center gap-2">
                  {pinnedRun && (
                    <Pin className="h-4 w-4 text-cyan-600" />
                  )}
                  <button
                    onClick={handleClosePanel}
                    className="text-gray-500 hover:text-red-600 transition-colors p-1 hover:bg-gray-100 rounded"
                    title="Close panel"
                  >
                    <X className="h-5 w-5" />
                  </button>
                </div>
              </div>
              <div className="text-sm text-cyan-600">
                {displayedRun.agent_name}
              </div>
            </div>

            {/* Run Details */}
            <div className="flex-1 space-y-4 text-sm overflow-y-auto">
              {/* Status */}
              <div className="bg-gray-50 p-3 rounded border border-gray-200">
                <div className="text-xs text-gray-600 mb-1">Status</div>
                <div className={`text-lg font-semibold ${
                  displayedRun.status === 'completed' ? 'text-green-600' :
                  displayedRun.status === 'failed' ? 'text-red-600' :
                  'text-blue-600'
                }`}>
                  {displayedRun.status.toUpperCase()}
                </div>
              </div>

              {/* Timing */}
              <div className="bg-gray-50 p-3 rounded border border-gray-200">
                <div className="text-xs text-gray-600 mb-2">Timing</div>
                <div className="space-y-2">
                  <div className="flex justify-between">
                    <span className="text-gray-600">Duration:</span>
                    <span className="text-gray-900">{formatDuration(displayedRun.duration_seconds || 0)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-gray-600">Started:</span>
                    <span className="text-gray-900 text-xs">{new Date(displayedRun.started_at).toLocaleString()}</span>
                  </div>
                </div>
              </div>

              {/* Resources */}
              {(displayedRun.total_tokens || displayedRun.cost) && (
                <div className="bg-gray-50 p-3 rounded border border-gray-200">
                  <div className="text-xs text-gray-600 mb-2">Resources</div>
                  <div className="space-y-2">
                    {displayedRun.total_tokens && (
                      <div className="flex justify-between">
                        <span className="text-gray-600">Tokens:</span>
                        <span className="text-purple-600">{formatTokens(displayedRun.total_tokens)}</span>
                      </div>
                    )}
                    {displayedRun.cost && (
                      <div className="flex justify-between">
                        <span className="text-gray-600">Cost:</span>
                        <span className="text-yellow-600">{formatCost(displayedRun.cost)}</span>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Error */}
              {displayedRun.error && (
                <div className="bg-red-50 p-3 rounded border border-red-200">
                  <div className="text-xs text-red-600 font-semibold mb-2">Error</div>
                  <div className="text-red-600 text-xs break-words">
                    {displayedRun.error}
                  </div>
                </div>
              )}
            </div>

            {/* Footer */}
            <div className="mt-6 pt-4 border-t border-gray-200">
              <div className="text-xs text-gray-600 text-center">
                {pinnedRun 
                  ? 'Double-click the bar to view full details in modal'
                  : 'Click to pin • Double-click for full details'}
              </div>
            </div>
          </div>
        ) : (
          <div className="h-full flex items-center justify-center p-6">
            <div className="text-center">
              <div className="text-gray-600 text-sm mb-2">
                Run Details
              </div>
              <div className="text-gray-500 text-xs">
                Hover or click a run bar to view details
              </div>
            </div>
          </div>
        )}
      </div>
      </div>
    </div>
  );
};
