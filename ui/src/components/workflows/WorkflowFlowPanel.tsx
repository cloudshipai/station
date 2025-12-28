import React, { memo, useEffect, useState, useMemo, useCallback } from 'react';
import { ReactFlow, ReactFlowProvider, Background, Controls, useNodesState, useEdgesState, MarkerType, Handle, Position } from '@xyflow/react';
import type { Node, Edge, NodeTypes } from '@xyflow/react';
import ELK from 'elkjs/lib/elk.bundled.js';
import { Play, CheckCircle, GitBranch, ArrowRight, Layout, AlertOctagon, MoreHorizontal, Bot, Repeat } from 'lucide-react';
import type { WorkflowDefinition } from '../../types/station';

const elk = new ELK();

interface WorkflowFlowPanelProps {
  workflow: WorkflowDefinition;
  isVisible: boolean;
}

interface WorkflowState {
  id: string;
  type: 'operation' | 'switch' | 'inject' | 'parallel' | 'foreach' | 'agent' | 'transform' | 'cron';
  name?: string;
  transition?: string;
  next?: string;
  end?: boolean;
  conditions?: Array<{ if: string; next: string }>;
  defaultNext?: string;
  branches?: Array<{ name: string; states?: WorkflowState[]; agent?: string }>;
  agent?: string;
  input?: { agent?: string };
}

const getAgentName = (state: WorkflowState): string | null => {
  if ((state.type === 'agent' || state.type === 'foreach') && state.agent) {
    return state.agent;
  }
  if (state.type === 'operation' && state.input?.agent) {
    return state.input.agent;
  }
  return null;
};

const getParallelAgents = (state: WorkflowState): string[] => {
  if (state.type !== 'parallel' || !state.branches) return [];
  
  const agents: string[] = [];
  for (const branch of state.branches) {
    if (branch.agent) {
      agents.push(branch.agent);
    }
    if (branch.states) {
      for (const branchState of branch.states) {
        const agent = getAgentName(branchState as WorkflowState);
        if (agent) agents.push(agent);
      }
    }
  }
  return agents;
};

const WorkflowNode = memo(({ data }: { data: any }) => {
  const { type, label, end, agentName, parallelAgents } = data;

  const getStyles = () => {
    switch (type) {
      case 'start':
        return {
          bg: 'bg-green-100',
          border: 'border-green-500',
          text: 'text-green-800',
          icon: <Play className="h-4 w-4" />,
          shape: 'rounded-full w-16 h-16 flex items-center justify-center'
        };
      case 'operation':
      case 'inject':
        return {
          bg: 'bg-blue-50',
          border: 'border-blue-400',
          text: 'text-blue-900',
          icon: <Layout className="h-4 w-4" />,
          shape: 'rounded-md min-w-[280px] max-w-[320px] px-4 py-3'
        };
      case 'foreach':
        return {
          bg: 'bg-indigo-50',
          border: 'border-indigo-400',
          text: 'text-indigo-900',
          icon: <Repeat className="h-4 w-4" />,
          shape: 'rounded-md min-w-[280px] max-w-[320px] px-4 py-3'
        };
      case 'switch':
        return {
          bg: 'bg-yellow-50',
          border: 'border-yellow-400',
          text: 'text-yellow-900',
          icon: <GitBranch className="h-4 w-4" />,
          shape: 'rounded-md min-w-[280px] max-w-[320px] px-4 py-3' 
        };
      case 'parallel':
        return {
          bg: 'bg-purple-50',
          border: 'border-purple-400',
          text: 'text-purple-900',
          icon: <MoreHorizontal className="h-4 w-4" />,
          shape: 'rounded-md min-w-[280px] max-w-[320px] px-4 py-3'
        };
      case 'end': 
        return {
          bg: 'bg-red-50',
          border: 'border-red-500',
          text: 'text-red-900',
          icon: <CheckCircle className="h-4 w-4" />,
          shape: 'rounded-full w-12 h-12 flex items-center justify-center'
        };
      default:
        return {
          bg: 'bg-gray-50',
          border: 'border-gray-400',
          text: 'text-gray-900',
          icon: <ArrowRight className="h-4 w-4" />,
          shape: 'rounded-md min-w-[280px] max-w-[320px] px-4 py-3'
        };
    }
  };

  const style = getStyles();
  const isEnd = end || type === 'end';

  const renderAgentBadge = (agent: string, key?: string) => (
    <div 
      key={key}
      className="inline-flex items-center gap-1.5 px-2 py-1 bg-emerald-100 text-emerald-800 rounded-md text-xs font-semibold"
    >
      <Bot className="h-3 w-3" />
      <span>{agent}</span>
    </div>
  );

  return (
    <div className={`
      relative border-2 shadow-sm transition-all duration-200
      ${style.bg} ${style.text} ${isEnd && type !== 'end' ? 'border-red-500 ring-2 ring-red-100' : style.border}
      ${style.shape}
    `}>
      {type === 'start' || type === 'end' ? (
        <div className="flex flex-col items-center justify-center">
          {style.icon}
          {label && <span className="text-xs font-bold mt-1">{label}</span>}
        </div>
      ) : (
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-3">
            <div className="p-1.5 rounded-full bg-white/50 border border-current flex-shrink-0">
              {style.icon}
            </div>
            <div className="flex-1 min-w-0">
              <div className="font-bold text-xs uppercase tracking-wider opacity-75">{type}</div>
              <div className="font-semibold text-sm break-words">{label}</div>
            </div>
          </div>
          
          {agentName && (
            <div className="pl-9">
              {renderAgentBadge(agentName)}
            </div>
          )}
          
          {parallelAgents && parallelAgents.length > 0 && (
            <div className="pl-9 flex flex-wrap gap-1.5">
              {parallelAgents.map((agent: string, idx: number) => renderAgentBadge(agent, `${agent}-${idx}`))}
            </div>
          )}
        </div>
      )}
      
      <Handle type="target" position={Position.Top} className="w-2 h-2 !bg-transparent !border-0" />
      <Handle type="source" position={Position.Bottom} className="w-2 h-2 !bg-transparent !border-0" />
    </div>
  );
});

const nodeTypes: NodeTypes = {
  workflowNode: WorkflowNode,
};

export const WorkflowFlowPanel = memo(({ workflow, isVisible }: WorkflowFlowPanelProps) => {
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);

  useEffect(() => {
    if (!isVisible || !workflow?.definition) return;

    const buildGraph = async () => {
      const flowNodes: Node[] = [];
      const flowEdges: Edge[] = [];
      const definition = workflow.definition;
      
      flowNodes.push({
        id: 'start',
        type: 'workflowNode',
        position: { x: 0, y: 0 },
        data: { type: 'start', label: 'START' }
      });

      const states = definition.states || [];
      const stateMap = new Map(states.map((s: WorkflowState) => [s.id, s]));

      states.forEach((state: WorkflowState) => {
        const parallelAgents = state.type === 'parallel' ? getParallelAgents(state) : undefined;
        flowNodes.push({
          id: state.id,
          type: 'workflowNode',
          position: { x: 0, y: 0 },
          data: { 
            type: state.type, 
            label: state.name || state.id,
            end: state.end,
            agentName: getAgentName(state),
            parallelAgents: parallelAgents && parallelAgents.length > 0 ? parallelAgents : undefined
          }
        });
      });

      if (definition.start) {
        flowEdges.push({
          id: `start-${definition.start}`,
          source: 'start',
          target: definition.start,
          type: 'smoothstep',
          markerEnd: { type: MarkerType.ArrowClosed },
          style: { stroke: '#94a3b8', strokeWidth: 2 }
        });
      }

      states.forEach((state: WorkflowState) => {
        if (state.transition) {
          flowEdges.push({
            id: `${state.id}-${state.transition}`,
            source: state.id,
            target: state.transition,
            type: 'smoothstep',
            markerEnd: { type: MarkerType.ArrowClosed },
            style: { stroke: '#94a3b8', strokeWidth: 2 }
          });
        }
        
        // 'next' field (legacy/simple)
        if (state.next) {
          flowEdges.push({
            id: `${state.id}-${state.next}`,
            source: state.id,
            target: state.next,
            type: 'smoothstep',
            markerEnd: { type: MarkerType.ArrowClosed },
            style: { stroke: '#94a3b8', strokeWidth: 2 }
          });
        }

        if (state.type === 'switch' && state.conditions) {
          state.conditions.forEach((cond, idx) => {
            flowEdges.push({
              id: `${state.id}-${cond.next}-${idx}`,
              source: state.id,
              target: cond.next,
              type: 'smoothstep',
              label: cond.if ? (cond.if.length > 15 ? cond.if.slice(0, 15) + '...' : cond.if) : `Cond ${idx + 1}`,
              labelStyle: { fill: '#64748b', fontSize: 10 },
              labelBgStyle: { fill: '#f1f5f9' },
              markerEnd: { type: MarkerType.ArrowClosed },
              style: { stroke: '#fbbf24', strokeWidth: 2 }
            });
          });

          if (state.defaultNext) {
            flowEdges.push({
              id: `${state.id}-${state.defaultNext}-default`,
              source: state.id,
              target: state.defaultNext,
              type: 'smoothstep',
              label: 'default',
              labelStyle: { fill: '#64748b', fontSize: 10 },
              labelBgStyle: { fill: '#f1f5f9' },
              markerEnd: { type: MarkerType.ArrowClosed },
              style: { stroke: '#94a3b8', strokeWidth: 2, strokeDasharray: '5,5' }
            });
          }
        }
      });

      const elkGraph = {
        id: 'root',
        layoutOptions: {
          'elk.algorithm': 'layered',
          'elk.direction': 'DOWN',
          'elk.spacing.nodeNode': '60',
          'elk.layered.spacing.nodeNodeBetweenLayers': '80',
          'elk.padding': '[top=20,left=20,bottom=20,right=20]'
        },
        children: flowNodes.map(node => {
          const isCircular = node.data.type === 'start' || node.data.type === 'end';
          const hasAgents = node.data.agentName || (node.data.parallelAgents && node.data.parallelAgents.length > 0);
          const agentCount = node.data.parallelAgents?.length || (node.data.agentName ? 1 : 0);
          return {
            id: node.id,
            width: isCircular ? 64 : 280,
            height: isCircular ? 64 : (hasAgents ? 80 + Math.ceil(agentCount / 2) * 28 : 60)
          };
        }),
        edges: flowEdges.map(edge => ({
          id: edge.id,
          sources: [edge.source],
          targets: [edge.target]
        }))
      };

      try {
        const layoutedGraph = await elk.layout(elkGraph);
        
        const layoutedNodes = flowNodes.map(node => {
          const layoutNode = layoutedGraph.children?.find(n => n.id === node.id);
          return {
            ...node,
            position: {
              x: layoutNode?.x || 0,
              y: layoutNode?.y || 0
            }
          };
        });

        setNodes(layoutedNodes);
        setEdges(flowEdges);
      } catch (err) {
        console.error('Elk layout error:', err);
        // Fallback to basic positioning if layout fails
        setNodes(flowNodes.map((n, i) => ({ ...n, position: { x: 0, y: i * 100 } })));
        setEdges(flowEdges);
      }
    };

    buildGraph();
  }, [workflow, isVisible, setNodes, setEdges]);

  if (!isVisible) return null;

  return (
    <div className="h-96 border border-gray-200 rounded-lg bg-gray-50 overflow-hidden">
      <ReactFlowProvider>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          fitView
          fitViewOptions={{ padding: 0.2 }}
          attributionPosition="bottom-right"
          nodesDraggable={false}
          nodesConnectable={false}
        >
          <Background color="#e2e8f0" gap={16} />
          <Controls className="bg-white border border-gray-200 shadow-sm" />
        </ReactFlow>
      </ReactFlowProvider>
    </div>
  );
});

WorkflowFlowPanel.displayName = 'WorkflowFlowPanel';
