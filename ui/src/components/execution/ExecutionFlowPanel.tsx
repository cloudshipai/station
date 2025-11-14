import React, { memo, useEffect, useState, useMemo } from 'react';
import { ReactFlow, ReactFlowProvider, Background, Controls } from '@xyflow/react';
import type { Node, Edge, NodeTypes } from '@xyflow/react';
import { ExecutionFlowNode } from '../nodes/ExecutionFlowNode';
import { buildExecutionFlowGraph } from '../../utils/executionFlowBuilder';
import type { JaegerTrace, JaegerSpan } from '../../types/station';

interface ExecutionFlowPanelProps {
  traceData: JaegerTrace | null;
  activeSpanIds: string[];
  isVisible: boolean;
}

interface AgentTab {
  agentId: number;
  agentName: string;
  spanCount: number;
}

const nodeTypes: NodeTypes = {
  executionFlow: ExecutionFlowNode,
};

export const ExecutionFlowPanel = memo(({ traceData, activeSpanIds, isVisible }: ExecutionFlowPanelProps) => {
  const [nodes, setNodes] = React.useState<Node[]>([]);
  const [edges, setEdges] = React.useState<Edge[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState<number | null>(null);

  // Extract unique agents from trace data
  const agentTabs = useMemo<AgentTab[]>(() => {
    if (!traceData || !traceData.spans) return [];
    
    const agentMap = new Map<number, { name: string; count: number }>();
    
    traceData.spans.forEach((span: JaegerSpan) => {
      const agentIdTag = span.tags?.find(t => t.key === 'agent.id');
      const agentNameTag = span.tags?.find(t => t.key === 'agent.name');
      
      if (agentIdTag && typeof agentIdTag.value === 'number' && agentNameTag) {
        const agentId = agentIdTag.value;
        const agentName = String(agentNameTag.value);
        
        if (!agentMap.has(agentId)) {
          agentMap.set(agentId, { name: agentName, count: 0 });
        }
        agentMap.get(agentId)!.count++;
      }
    });
    
    return Array.from(agentMap.entries())
      .map(([agentId, info]) => ({
        agentId,
        agentName: info.name,
        spanCount: info.count,
      }))
      .sort((a, b) => b.spanCount - a.spanCount); // Sort by span count (parent first)
  }, [traceData]);

  // Set default selected agent (first one, usually the parent orchestrator)
  useEffect(() => {
    if (agentTabs.length > 0 && selectedAgentId === null) {
      setSelectedAgentId(agentTabs[0].agentId);
    }
  }, [agentTabs, selectedAgentId]);

  useEffect(() => {
    if (!traceData || !isVisible) {
      setNodes([]);
      setEdges([]);
      return;
    }

    // Filter trace data for selected agent
    let filteredTrace = traceData;
    if (selectedAgentId !== null) {
      // Build a map of which agent owns each span
      const spanToAgentMap = new Map<string, number>();
      const agentSpanCounts = new Map<number, number>();
      
      traceData.spans.forEach((span: JaegerSpan) => {
        const agentIdTag = span.tags?.find(t => t.key === 'agent.id');
        if (agentIdTag && typeof agentIdTag.value === 'number') {
          const agentId = agentIdTag.value;
          spanToAgentMap.set(span.spanID, agentId);
          agentSpanCounts.set(agentId, (agentSpanCounts.get(agentId) || 0) + 1);
        }
      });
      
      // Identify the orchestrator by name pattern (contains "orchestrator" or "coordinator")
      // Fallback: if no name match, use the LAST agent in tabs (least spans = top-level delegator)
      let orchestratorId: number | null = null;
      
      // Try to find by name pattern first
      const orchestratorTab = agentTabs.find(t => 
        t.agentName.toLowerCase().includes('orchestrator') || 
        t.agentName.toLowerCase().includes('coordinator') ||
        t.agentName.toLowerCase().includes('master')
      );
      
      if (orchestratorTab) {
        orchestratorId = orchestratorTab.agentId;
      } else {
        // Fallback: agent with LEAST spans is likely the top-level orchestrator
        // (it delegates quickly to child agents which do the actual work)
        orchestratorId = agentTabs.length > 0 ? agentTabs[agentTabs.length - 1].agentId : null;
      }
      
      // Helper: Check if span is a meaningful tool call
      const isToolCallSpan = (span: JaegerSpan): boolean => {
        const op = span.operationName;
        return op.startsWith('__') || op.startsWith('faker.') || op === 'generate';
      };
      
      // Check if selected agent is the orchestrator
      const isOrchestrator = selectedAgentId === orchestratorId;
      
      console.log(`[ExecutionFlowPanel] Agent ${selectedAgentId}, Orchestrator: ${orchestratorId}, IsOrch: ${isOrchestrator}, Tabs:`, agentTabs.map(t => `${t.agentName}(${t.agentId})`));
      
      // For the selected agent, include:
      // 1. Spans directly owned by this agent (agent.id matches)
      // 2. Tool call spans (__agent_*, __read_file, etc.) that don't belong to a DIFFERENT agent
      //    (This captures orchestrator delegations that may not have agent.id tags)
      
      const filteredSpans = traceData.spans.filter((span: JaegerSpan) => {
        const spanAgentId = spanToAgentMap.get(span.spanID);
        
        // Include if span belongs to selected agent
        if (spanAgentId === selectedAgentId) {
          return true;
        }
        
        // Include tool calls that don't belong to a different specific agent
        // (orchestrator delegations like __agent_* often don't have agent.id tags)
        if (isToolCallSpan(span)) {
          const op = span.operationName;
          
          // If span has no agent.id tag:
          if (!spanAgentId) {
            // Only include __agent_* delegations for the ORCHESTRATOR agent
            // Child agents should not see delegation calls in their timeline
            if (op.startsWith('__agent_')) {
              if (isOrchestrator) {
                console.log(`[ExecutionFlowPanel] [Orchestrator] Including agent delegation: ${op}`);
                return true;
              } else {
                console.log(`[ExecutionFlowPanel] [Child] Excluding agent delegation: ${op}`);
                return false;
              }
            }
            // Exclude other untagged tool calls (like 'generate', regular tools)
            // They belong to the agent's internal execution
            return false;
          }
          
          // If span belongs to a different agent, exclude it
          // (it will appear in that agent's tab)
          return false;
        }
        
        return false;
      });
      
      console.log(`[ExecutionFlowPanel] Filtered ${filteredSpans.length} spans for agent ${selectedAgentId}`);
      console.log(`[ExecutionFlowPanel] Tool calls found:`, filteredSpans.filter(s => isToolCallSpan(s)).map(s => s.operationName));
      
      filteredTrace = {
        ...traceData,
        spans: filteredSpans,
      };
    }

    // Build horizontal flow graph for selected agent
    const executionFlow = buildExecutionFlowGraph({
      traceData: filteredTrace,
      startX: 100,
      startY: 80,
      activeSpanIds,
      horizontal: true,
    });

    setNodes(executionFlow.nodes);
    setEdges(executionFlow.edges);
  }, [traceData, activeSpanIds, isVisible, selectedAgentId]);

  if (!isVisible) {
    return null;
  }

  return (
    <div className="h-64 border-t-2 border-cyan-500/30 bg-tokyo-bg-dark flex flex-col">
      {/* Header with Agent Tabs */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-gray-700 bg-gray-900 flex-shrink-0">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 bg-cyan-500 rounded-full animate-pulse"></div>
            <h3 className="text-sm font-mono font-semibold text-cyan-400">
              Execution Flow Timeline
            </h3>
          </div>
          
          {/* Agent Tabs */}
          {agentTabs.length > 1 && (
            <div className="flex items-center gap-2 ml-4 border-l border-gray-700 pl-4">
              {agentTabs.map((tab) => (
                <button
                  key={tab.agentId}
                  onClick={() => setSelectedAgentId(tab.agentId)}
                  className={`
                    px-3 py-1 rounded-md text-xs font-mono transition-all
                    ${selectedAgentId === tab.agentId
                      ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/50'
                      : 'bg-gray-800 text-gray-400 border border-gray-700 hover:border-cyan-500/30 hover:text-cyan-400'
                    }
                  `}
                >
                  {tab.agentName}
                  <span className="ml-1 text-gray-500">({tab.spanCount})</span>
                </button>
              ))}
            </div>
          )}
        </div>
        
        <div className="text-xs text-gray-500 font-mono flex items-center gap-2">
          <span>{nodes.length - 2} steps</span>
          <span>·</span>
          <span>Scroll horizontally to explore →</span>
        </div>
      </div>

      {/* Flow Container */}
      <div className="flex-1 relative overflow-hidden">
        {nodes.length === 0 ? (
          <div className="flex items-center justify-center h-full bg-tokyo-bg">
            <div className="text-center">
              <div className="text-gray-500 font-mono text-sm mb-2">
                No tool calls in this agent's execution
              </div>
              <div className="text-gray-600 font-mono text-xs">
                {selectedAgentId !== null && agentTabs.length > 0 
                  ? `${agentTabs.find(t => t.agentId === selectedAgentId)?.agentName || 'Agent'} executed but made no tool calls`
                  : 'Agent executed without calling tools'}
              </div>
            </div>
          </div>
        ) : (
          <ReactFlowProvider>
            <ReactFlow
              nodes={nodes}
              edges={edges}
              nodeTypes={nodeTypes}
              fitView
              fitViewOptions={{ padding: 0.2 }}
              minZoom={0.5}
              maxZoom={1.5}
              className="bg-tokyo-bg"
              panOnScroll
              panOnDrag
              zoomOnScroll={false}
              preventScrolling={false}
              nodesDraggable={false}
              nodesConnectable={false}
              elementsSelectable={true}
            >
              <Background color="#1a1b26" gap={16} />
              <Controls 
                showZoom={true}
                showFitView={true}
                showInteractive={false}
                className="!bottom-2 !left-2"
              />
            </ReactFlow>
          </ReactFlowProvider>
        )}
      </div>
    </div>
  );
});

ExecutionFlowPanel.displayName = 'ExecutionFlowPanel';
