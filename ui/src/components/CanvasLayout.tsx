import React, { useCallback } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  addEdge,
  type Connection,
  type Edge,
  type Node,
  BackgroundVariant,
} from '@xyflow/react';

interface CanvasLayoutProps {
  title: string;
  initialNodes?: Node[];
  initialEdges?: Edge[];
  nodeTypes?: any;
  onNodeClick?: (event: React.MouseEvent, node: Node) => void;
  children?: React.ReactNode;
}

export const CanvasLayout: React.FC<CanvasLayoutProps> = ({
  title,
  initialNodes = [],
  initialEdges = [],
  nodeTypes,
  onNodeClick,
  children,
}) => {
  const [nodes, , onNodesChange] = useNodesState(initialNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges);

  const onConnect = useCallback(
    (params: Connection) => setEdges((eds) => addEdge(params, eds)),
    [setEdges]
  );

  return (
    <div className="h-full flex flex-col">
      {/* Canvas Header */}
      <div className="flex items-center justify-between p-4 border-b border-border bg-card">
        <h1 className="text-xl font-semibold">{title}</h1>
        <div className="flex items-center space-x-2">
          {children}
        </div>
      </div>

      {/* Canvas Area */}
      <div className="flex-1 relative">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={onConnect}
          onNodeClick={onNodeClick}
          nodeTypes={nodeTypes}
          fitView
          className="bg-background"
        >
          <Background 
            color="#aaa" 
            gap={16} 
            size={1}
            variant={BackgroundVariant.Dots} 
          />
          <Controls 
            className="bg-card border border-border"
          />
          <MiniMap 
            className="bg-card border border-border"
            nodeColor="#3b82f6"
            maskColor="rgba(0, 0, 0, 0.1)"
          />
        </ReactFlow>
      </div>
    </div>
  );
};