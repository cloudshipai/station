# Swimlane Timeline View

## Overview
The Swimlane Timeline View provides a visual timeline representation of agent execution runs, making it easy to spot patterns, identify bottlenecks, and understand agent execution behavior over time.

## Features

### Visual Timeline
- **Horizontal Swimlanes**: Each agent gets its own horizontal lane showing all its runs
- **Time-based X-axis**: Runs are positioned based on their start time and duration
- **Color-coded Status**: 
  - Green = Completed
  - Red = Failed
  - Blue (pulsing) = Running
- **Proportional Bar Height**: Bar thickness reflects token usage or cost (configurable)

### Interactive Controls
- **Time Range Selector**: Last Hour | Last Day | Last Week | Last Month | All Time
- **Density Metric Toggle**: Switch between Tokens and Cost for bar height
- **Group By Options**: Agent (default) | Environment | Version
- **P95 Overlay**: Optional visualization of 95th percentile duration per lane

### Agent Statistics on Hover
Hovering over an agent lane header shows:
- Success Rate (%)
- Average Duration
- P50 Duration (median)
- P95 Duration (95th percentile)
- Total Tokens Used
- Total Cost

### Run Details on Hover
Hovering over a run bar shows tooltip with:
- Run ID and Status
- Duration
- Start Time
- Token Count
- Cost Estimate
- Error Message (if failed)

### Parent-Child Relationships
- Automatically detects `parent_run_id` relationships
- Click on a run to highlight its parent and children
- Visual indicator shows number of hierarchical relationships detected
- Useful for multi-agent workflows

## Implementation Details

### New Components

**`SwimlanePage.tsx`**
- Main container component for timeline view
- Manages state for time range, density metric, grouping, and highlighting
- Filters and groups runs into lanes
- Calculates time bounds and statistics
- Handles relationship highlighting

**`TimelineLane.tsx`**
- Renders individual agent lane with runs
- Displays agent statistics on hover
- Shows optional P95 duration overlay
- Positions run bars using calculated layout

**`TimelineBar.tsx`**
- Renders individual run bar with status-based coloring
- Shows tooltip with detailed run information on hover
- Handles click events to open run details modal
- Supports highlighting for parent-child relationships

**`TimelineControls.tsx`**
- Control panel for filtering and display options
- Time range selector
- Density metric toggle (Tokens/Cost)
- P95 overlay checkbox
- Group by selector
- Legend showing status colors

### Utility Functions (`timelineLayout.ts`)

**Layout Calculations**
- `groupRunsByAgent()` - Groups runs into lanes by agent name
- `calculateTimeBounds()` - Determines time axis min/max with padding
- `calculateBarLayout()` - Computes bar position (left %) and width (%)
- `calculateBarHeight()` - Normalizes bar height based on tokens/cost

**Statistics**
- `calculateLaneStats()` - Computes per-lane statistics (success rate, percentiles, etc.)
- `findRunRelationships()` - Identifies parent-child run hierarchies

**Filtering & Formatting**
- `filterRunsByTimeRange()` - Filters runs by selected time range preset
- `formatTime()` - Formats timestamps for display (short/medium/long)
- `formatDuration()` - Human-readable duration formatting

## Usage

### Accessing Timeline View
1. Navigate to the Runs page
2. Click the **Timeline** tab in the header
3. Use controls to filter and customize the view

### Finding Issues
- Look for **red bars** (failed runs) clustering together
- Check **unusually tall bars** (high token/cost usage)
- Hover lane headers to see success rates and identify problematic agents
- Enable **P95 overlay** to see if any runs exceed expected duration

### Analyzing Parent-Child Workflows
1. Click on any run bar
2. Related parent/child runs will be highlighted with cyan ring
3. Footer shows total number of hierarchical relationships
4. Use this to trace multi-agent execution flows

### Customizing Display
- **Reduce noise**: Select "Last Hour" or "Last Day" time range
- **Cost optimization**: Switch density metric to "Cost" to find expensive runs
- **Performance tuning**: Enable P95 overlay to identify outliers

## Technical Notes

### Performance
- Virtualization not yet implemented (consider for >50 agents)
- Time bounds calculated with 5% padding on each side
- Minimum bar width of 0.5% ensures visibility of short runs
- Bar height ranges from 8px (min) to 40px (max)

### Data Requirements
The timeline view expects runs with:
- `id`, `agent_id`, `agent_name`
- `status`: 'completed' | 'running' | 'failed'
- `started_at`: ISO timestamp
- `duration_seconds`: number
- Optional: `total_tokens`, `cost`, `parent_run_id`, `error`

### Styling
- Uses Tokyo Night theme colors throughout
- Responsive tooltips with proper positioning
- Smooth transitions and hover effects
- Consistent font-mono styling for data

## Future Enhancements

### Drag-to-Select Time Window
- Allow dragging across timeline to filter runs by custom time window
- Update runs list to show only selected runs

### Lane Virtualization
- Implement virtual scrolling for 100+ agent lanes
- Improve performance for large-scale deployments

### Parent-Child Connection Lines
- Draw dotted lines between parent and child run bars
- SVG overlay showing hierarchical execution flow

### Zoom & Pan
- Mouse wheel zoom for detailed time inspection
- Pan timeline left/right for exploring long time ranges

### Run Comparison
- Select multiple runs to compare side-by-side
- Diff view showing differences in execution

### Export Timeline
- Export timeline as PNG/SVG image
- Share timeline view with permalink (URL params)

## API Integration

The timeline view uses the existing `/api/v1/runs` endpoint. Ensure the API response includes:

```typescript
{
  runs: Array<{
    id: number;
    agent_id: number;
    agent_name: string;
    status: 'completed' | 'running' | 'failed';
    started_at: string; // ISO timestamp
    duration_seconds: number;
    total_tokens?: number;
    cost?: number;
    parent_run_id?: number;
    error?: string;
  }>
}
```

Cost can be calculated server-side or client-side based on token usage and model pricing.
