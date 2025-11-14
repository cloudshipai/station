// Timeline layout calculation utilities

export interface TimelineRun {
  id: number;
  agent_name: string;
  agent_id: number;
  status: 'completed' | 'running' | 'failed';
  started_at: string;
  duration_seconds: number;
  total_tokens?: number;
  cost?: number;
  parent_run_id?: number;
  error?: string;
}

export interface TimelineMetrics {
  tokens: number;
  cost: number;
}

export interface AgentLaneStats {
  agent_id: number;
  agent_name: string;
  total_runs: number;
  success_rate: number;
  avg_duration: number;
  p50_duration: number;
  p95_duration: number;
  total_cost: number;
  total_tokens: number;
}

export interface TimeRange {
  start: number;
  end: number;
}

// Group runs by agent into lanes
export function groupRunsByAgent(runs: TimelineRun[]): Map<string, TimelineRun[]> {
  const lanes = new Map<string, TimelineRun[]>();
  
  runs.forEach(run => {
    const key = run.agent_name;
    if (!lanes.has(key)) {
      lanes.set(key, []);
    }
    lanes.get(key)!.push(run);
  });
  
  // Sort runs within each lane by start time
  lanes.forEach(runs => runs.sort((a, b) => 
    new Date(a.started_at).getTime() - new Date(b.started_at).getTime()
  ));
  
  return lanes;
}

// Calculate time axis bounds
export function calculateTimeBounds(runs: TimelineRun[]): TimeRange {
  if (runs.length === 0) {
    const now = Date.now();
    return { start: now - 3600000, end: now }; // Last hour
  }
  
  const times = runs.map(r => {
    const start = new Date(r.started_at).getTime();
    const end = start + (r.duration_seconds || 0) * 1000;
    return { start, end };
  });
  
  const minStart = Math.min(...times.map(t => t.start));
  const maxEnd = Math.max(...times.map(t => t.end));
  
  // Add 5% padding on each side
  const padding = (maxEnd - minStart) * 0.05;
  
  return {
    start: minStart - padding,
    end: maxEnd + padding
  };
}

// Calculate bar position and width as percentages
export function calculateBarLayout(
  run: TimelineRun,
  timeBounds: TimeRange
): { left: number; width: number } {
  const startTime = new Date(run.started_at).getTime();
  const duration = (run.duration_seconds || 0) * 1000;
  const endTime = startTime + duration;
  
  const totalRange = timeBounds.end - timeBounds.start;
  
  const left = ((startTime - timeBounds.start) / totalRange) * 100;
  const width = (duration / totalRange) * 100;
  
  return {
    left: Math.max(0, Math.min(100, left)),
    width: Math.max(0.1, Math.min(100 - left, width)) // Minimum 0.1% width for visibility
  };
}

// Calculate bar height based on metric (tokens or cost)
export function calculateBarHeight(
  run: TimelineRun,
  metric: 'tokens' | 'cost',
  maxValue: number
): number {
  const value = metric === 'tokens' 
    ? (run.total_tokens || 0) 
    : (run.cost || 0);
  
  if (maxValue === 0) return 20; // Default height
  
  const MIN_HEIGHT = 8;
  const MAX_HEIGHT = 40;
  
  const normalizedHeight = (value / maxValue) * MAX_HEIGHT;
  return Math.max(MIN_HEIGHT, Math.min(MAX_HEIGHT, normalizedHeight));
}

// Calculate agent lane statistics
export function calculateLaneStats(runs: TimelineRun[]): AgentLaneStats[] {
  const lanes = groupRunsByAgent(runs);
  const stats: AgentLaneStats[] = [];
  
  lanes.forEach((laneRuns, agentName) => {
    const completedRuns = laneRuns.filter(r => r.status === 'completed' || r.status === 'failed');
    const successfulRuns = laneRuns.filter(r => r.status === 'completed');
    
    const durations = completedRuns
      .map(r => r.duration_seconds || 0)
      .filter(d => d > 0)
      .sort((a, b) => a - b);
    
    const p50_index = Math.floor(durations.length * 0.5);
    const p95_index = Math.floor(durations.length * 0.95);
    
    stats.push({
      agent_id: laneRuns[0].agent_id,
      agent_name: agentName,
      total_runs: laneRuns.length,
      success_rate: completedRuns.length > 0 
        ? (successfulRuns.length / completedRuns.length) * 100 
        : 0,
      avg_duration: durations.length > 0
        ? durations.reduce((sum, d) => sum + d, 0) / durations.length
        : 0,
      p50_duration: durations[p50_index] || 0,
      p95_duration: durations[p95_index] || 0,
      total_cost: laneRuns.reduce((sum, r) => sum + (r.cost || 0), 0),
      total_tokens: laneRuns.reduce((sum, r) => sum + (r.total_tokens || 0), 0)
    });
  });
  
  return stats;
}

// Format time for display
export function formatTime(timestamp: number, format: 'short' | 'medium' | 'long' = 'medium'): string {
  const date = new Date(timestamp);
  
  if (format === 'short') {
    return date.toLocaleTimeString('en-US', { 
      hour: '2-digit', 
      minute: '2-digit' 
    });
  }
  
  if (format === 'long') {
    return date.toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    });
  }
  
  return date.toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
}

// Format duration for display
export function formatDuration(seconds: number): string {
  if (seconds < 1) return `${(seconds * 1000).toFixed(0)}ms`;
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const mins = Math.floor(seconds / 60);
  const secs = Math.floor(seconds % 60);
  return `${mins}m ${secs}s`;
}

// Get time range presets
export interface TimeRangePreset {
  label: string;
  value: 'hour' | 'day' | 'week' | 'month' | 'all';
  milliseconds: number | null;
}

export const TIME_RANGE_PRESETS: TimeRangePreset[] = [
  { label: 'Last Hour', value: 'hour', milliseconds: 3600000 },
  { label: 'Last Day', value: 'day', milliseconds: 86400000 },
  { label: 'Last Week', value: 'week', milliseconds: 604800000 },
  { label: 'Last Month', value: 'month', milliseconds: 2592000000 },
  { label: 'All Time', value: 'all', milliseconds: null }
];

// Filter runs by time range
export function filterRunsByTimeRange(
  runs: TimelineRun[],
  range: TimeRangePreset
): TimelineRun[] {
  if (range.milliseconds === null) return runs;
  
  const cutoff = Date.now() - range.milliseconds;
  return runs.filter(run => new Date(run.started_at).getTime() >= cutoff);
}

// Find parent-child relationships
export interface RunRelationship {
  parent: TimelineRun;
  child: TimelineRun;
}

export function findRunRelationships(runs: TimelineRun[]): RunRelationship[] {
  const relationships: RunRelationship[] = [];
  const runMap = new Map(runs.map(r => [r.id, r]));
  
  runs.forEach(run => {
    if (run.parent_run_id) {
      const parent = runMap.get(run.parent_run_id);
      if (parent) {
        relationships.push({ parent, child: run });
      }
    }
  });
  
  return relationships;
}
