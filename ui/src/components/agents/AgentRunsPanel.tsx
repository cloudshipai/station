import React, { useState, useEffect } from 'react';
import { Play, Clock, CheckCircle, XCircle, Loader, TrendingUp, TrendingDown, DollarSign, Eye, Activity } from 'lucide-react';
import { agentRunsApi } from '../../api/station';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

interface Run {
  id: number;
  agent_id: number;
  agent_name: string;
  status: 'completed' | 'running' | 'failed';
  duration_seconds?: number;
  started_at: string;
  total_tokens?: number;
  input_tokens?: number;
  output_tokens?: number;
}

interface AgentRunsPanelProps {
  agentId: number | null;
  agentName: string;
  onRunClick: (runId: number, agentId: number) => void;
  onExecutionViewClick?: (runId: number, agentId: number) => void;
  selectedRunId?: number | null;
}

export const AgentRunsPanel: React.FC<AgentRunsPanelProps> = ({ agentId, agentName, onRunClick, onExecutionViewClick, selectedRunId }) => {
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!agentId) {
      setRuns([]);
      return;
    }

    const fetchRuns = async () => {
      setLoading(true);
      try {
        const response = await agentRunsApi.getAll();
        const allRuns = response.data.runs || [];
        // Filter by agent and take last 10
        const agentRuns = allRuns
          .filter((r: Run) => r.agent_id === agentId)
          .slice(0, 10);
        setRuns(agentRuns);
      } catch (error) {
        console.error('Failed to fetch runs:', error);
        setRuns([]);
      } finally {
        setLoading(false);
      }
    };

    fetchRuns();
    
    // Refresh every 5 seconds
    const interval = setInterval(fetchRuns, 5000);
    return () => clearInterval(interval);
  }, [agentId]);

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'completed':
        return <CheckCircle className="h-4 w-4 text-green-600" />;
      case 'failed':
        return <XCircle className="h-4 w-4 text-red-600" />;
      case 'running':
        return <Loader className="h-4 w-4 text-station-blue animate-spin" />;
      default:
        return <Clock className="h-4 w-4 text-muted-foreground" />;
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'border-green-200 bg-green-50';
      case 'failed':
        return 'border-red-200 bg-red-50';
      case 'running':
        return 'border-blue-200 bg-blue-50';
      default:
        return 'border-border';
    }
  };

  const formatDuration = (seconds?: number) => {
    if (!seconds) return 'N/A';
    if (seconds < 1) return `${Math.round(seconds * 1000)}ms`;
    if (seconds < 60) return `${seconds.toFixed(1)}s`;
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}m ${secs}s`;
  };

  const formatTokens = (tokens?: number) => {
    if (!tokens) return null;
    if (tokens < 1000) return `${tokens}`;
    if (tokens < 1000000) return `${(tokens / 1000).toFixed(1)}k`;
    return `${(tokens / 1000000).toFixed(2)}M`;
  };

  const estimateCost = (inputTokens?: number, outputTokens?: number): string | null => {
    if (!inputTokens || !outputTokens) return null;
    // Assuming gpt-4o-mini pricing: $0.150/1M input, $0.600/1M output
    const inputCost = (inputTokens / 1_000_000) * 0.150;
    const outputCost = (outputTokens / 1_000_000) * 0.600;
    const total = inputCost + outputCost;
    if (total < 0.0001) return '<$0.0001';
    return `$${total.toFixed(4)}`;
  };

  // Calculate trend compared to previous run
  const getTrend = (index: number): 'up' | 'down' | 'same' | null => {
    if (index >= runs.length - 1) return null;
    const current = runs[index];
    const previous = runs[index + 1];
    if (!current.duration_seconds || !previous.duration_seconds) return null;
    
    if (current.duration_seconds > previous.duration_seconds * 1.1) return 'up';
    if (current.duration_seconds < previous.duration_seconds * 0.9) return 'down';
    return 'same';
  };

  if (!agentId) {
    return (
      <div className="w-96 h-full border-l bg-muted/30 flex items-center justify-center">
        <div className="text-center text-muted-foreground text-sm">
          Select an agent to view runs
        </div>
      </div>
    );
  }

  return (
    <div className="w-96 h-full border-l bg-background overflow-hidden flex flex-col">
      {/* Header */}
      <div className="p-4 border-b">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold">Recent Runs</h2>
            {runs.some(r => r.status === 'running') && (
              <Badge variant="destructive" className="animate-pulse">
                <div className="flex items-center gap-1.5">
                  <div className="w-2 h-2 bg-white rounded-full animate-ping absolute"></div>
                  <div className="w-2 h-2 bg-white rounded-full"></div>
                  <span className="text-xs font-bold ml-1">LIVE</span>
                </div>
              </Badge>
            )}
          </div>
          {loading && <Loader className="h-4 w-4 text-muted-foreground animate-spin" />}
        </div>
        <p className="text-xs text-muted-foreground">
          {agentName} · Last {runs.length} executions
        </p>
      </div>

      {/* Runs List */}
      <div className="flex-1 overflow-y-auto p-4 space-y-3">
        {loading && runs.length === 0 ? (
          <div className="text-center py-8">
            <Loader className="h-8 w-8 text-muted-foreground animate-spin mx-auto mb-2" />
            <div className="text-muted-foreground text-sm">Loading runs...</div>
          </div>
        ) : runs.length === 0 ? (
          <div className="text-center py-8">
            <Play className="h-12 w-12 text-muted-foreground/50 mx-auto mb-3" />
            <div className="text-muted-foreground text-sm mb-1">No runs yet</div>
            <div className="text-muted-foreground/70 text-xs">
              Execute this agent to see runs here
            </div>
          </div>
        ) : (
          runs.map((run, index) => {
            const trend = getTrend(index);
            const cost = estimateCost(run.input_tokens, run.output_tokens);
            const isSelected = selectedRunId === run.id;
            
            return (
              <Card
                key={run.id}
                className={cn(
                  "transition-all duration-200",
                  isSelected && "ring-2 ring-primary",
                  run.status === 'running' && "border-blue-300",
                  run.status === 'failed' && "border-red-300",
                  run.status === 'completed' && "border-green-300"
                )}
              >
                <CardContent className="p-3">
                  {/* Header Row */}
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      {getStatusIcon(run.status)}
                      <span className="text-xs text-muted-foreground">
                        Run #{run.id}
                      </span>
                      {run.status === 'running' && (
                        <Badge variant="destructive" className="text-xs animate-pulse">
                          LIVE
                        </Badge>
                      )}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {new Date(run.started_at).toLocaleTimeString()}
                    </div>
                  </div>

                  {/* Metrics Row */}
                  <div className="flex items-center gap-3 text-xs flex-wrap mb-2">
                    {/* Duration */}
                    <div className="flex items-center gap-1">
                      <Clock className="h-3 w-3 text-muted-foreground" />
                      <span className="font-mono">
                        {formatDuration(run.duration_seconds)}
                      </span>
                      {trend && trend !== 'same' && (
                        trend === 'down' ? (
                          <TrendingDown className="h-3 w-3 text-green-600" />
                        ) : (
                          <TrendingUp className="h-3 w-3 text-orange-600" />
                        )
                      )}
                    </div>

                    {/* Tokens */}
                    {run.total_tokens && (
                      <span className="text-purple-600 font-mono">
                        {formatTokens(run.total_tokens)} tok
                      </span>
                    )}

                    {/* Cost */}
                    {cost && (
                      <div className="flex items-center gap-1">
                        <DollarSign className="h-3 w-3 text-yellow-600" />
                        <span className="text-yellow-600 font-mono">
                          {cost}
                        </span>
                      </div>
                    )}
                  </div>

                  {/* Status Badge and Action Buttons */}
                  <div className="pt-2 border-t flex items-center justify-between">
                    <Badge 
                      variant={
                        run.status === 'completed' ? 'default' :
                        run.status === 'failed' ? 'destructive' :
                        'secondary'
                      }
                      className="text-xs uppercase"
                    >
                      {run.status}
                    </Badge>
                    
                    {/* Action Buttons */}
                    <div className="flex items-center gap-1">
                      {/* Execution View Button */}
                      {onExecutionViewClick && run.status === 'completed' && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={(e) => {
                            e.stopPropagation();
                            onExecutionViewClick(run.id, run.agent_id);
                          }}
                          className="h-7 text-xs px-2"
                          title="View execution flow"
                        >
                          <Activity className="h-3 w-3 mr-1" />
                          Flow
                        </Button>
                      )}
                      
                      {/* Details Modal Button */}
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={(e) => {
                          e.stopPropagation();
                          onRunClick(run.id, run.agent_id);
                        }}
                        className="h-7 text-xs px-2"
                        title="View run details"
                      >
                        <Eye className="h-3 w-3 mr-1" />
                        Details
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>
            );
          })
        )}
      </div>
    </div>
  );
};
