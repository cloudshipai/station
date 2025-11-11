import React, { memo, useEffect } from 'react';
import { ReactFlow, ReactFlowProvider, Background, Controls } from '@xyflow/react';
import type { Node, Edge, NodeTypes } from '@xyflow/react';
import { ExecutionFlowNode } from '../nodes/ExecutionFlowNode';
import { buildExecutionFlowGraph } from '../../utils/executionFlowBuilder';
import type { JaegerTrace } from '../../types/station';

interface ExecutionFlowPanelProps {
  traceData: JaegerTrace | null;
  activeSpanIds: string[];
  isVisible: boolean;
}

const nodeTypes: NodeTypes = {
  executionFlow: ExecutionFlowNode,
};

export const ExecutionFlowPanel = memo(({ traceData, activeSpanIds, isVisible }: ExecutionFlowPanelProps) => {
  const [nodes, setNodes] = React.useState<Node[]>([]);
  const [edges, setEdges] = React.useState<Edge[]>([]);

  useEffect(() => {
    if (!traceData || !isVisible) {
      setNodes([]);
      setEdges([]);
      return;
    }

    // Build horizontal flow graph
    const executionFlow = buildExecutionFlowGraph({
      traceData,
      startX: 100,
      startY: 80, // Centered vertically in panel
      activeSpanIds,
      horizontal: true, // NEW: horizontal layout
    });

    setNodes(executionFlow.nodes);
    setEdges(executionFlow.edges);
  }, [traceData, activeSpanIds, isVisible]);

  if (!isVisible) {
    return null;
  }

  return (
    <div className="h-56 border-t-2 border-cyan-500/30 bg-tokyo-bg-dark flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-gray-700 bg-gray-900 flex-shrink-0">
        <div className="flex items-center gap-2">
          <div className="w-2 h-2 bg-cyan-500 rounded-full animate-pulse"></div>
          <h3 className="text-sm font-mono font-semibold text-cyan-400">
            Execution Flow Timeline
          </h3>
          <span className="text-xs text-gray-500 font-mono">
            {nodes.length - 2} steps {/* Exclude START and END */}
          </span>
        </div>
        <div className="text-xs text-gray-500 font-mono">
          Scroll horizontally to explore →
        </div>
      </div>

      {/* Flow Container */}
      <div className="flex-1 relative overflow-hidden">
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
      </div>
    </div>
  );
});

ExecutionFlowPanel.displayName = 'ExecutionFlowPanel';
