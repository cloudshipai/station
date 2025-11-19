import React from 'react';
import { 
  ReactFlow, 
  ReactFlowProvider, 
  Background, 
  Controls, 
  type Node, 
  type Edge, 
  type NodeTypes,
  type OnNodesChange,
  type OnEdgesChange
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { Bot } from 'lucide-react';

interface SimplifiedGraphCanvasProps {
  nodes: Node[];
  edges: Edge[];
  onNodesChange: OnNodesChange;
  onEdgesChange: OnEdgesChange;
  nodeTypes: NodeTypes;
  selectedAgent: any | null;
}

export const SimplifiedGraphCanvas: React.FC<SimplifiedGraphCanvasProps> = ({ 
  nodes, 
  edges, 
  onNodesChange, 
  onEdgesChange, 
  nodeTypes,
  selectedAgent 
}) => {
  
  if (!selectedAgent) {
    return (
      <div className="h-full w-full bg-gray-50 flex flex-col items-center justify-center relative overflow-hidden">
        <div className="absolute inset-0 opacity-30" 
             style={{
               backgroundImage: 'radial-gradient(#cbd5e1 1px, transparent 1px)',
               backgroundSize: '20px 20px'
             }} 
        />
        <div className="bg-white p-4 rounded-full shadow-sm mb-4 z-10">
          <Bot className="h-8 w-8 text-gray-300" />
        </div>
        <p className="text-gray-400 font-medium z-10">Select an agent to view details</p>
      </div>
    );
  }

  return (
    <div className="h-full w-full bg-gray-50 relative">
      <ReactFlowProvider>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          nodeTypes={nodeTypes}
          fitView
          className="bg-gray-50"
          defaultEdgeOptions={{
            animated: true,
            style: {
              stroke: '#a855f7', // Purple style to match light theme
              strokeWidth: 2,
            }
          }}
        >
          <Background color="#cbd5e1" gap={20} size={1} />
          <Controls className="bg-white border-gray-200 shadow-sm" />
        </ReactFlow>
      </ReactFlowProvider>
    </div>
  );
};
