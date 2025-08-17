import ELK, { type ElkNode } from "elkjs/lib/elk.bundled.js";
import type { Node, Edge } from '@xyflow/react';

// Node dimensions
export const NODE_WIDTH = 200;
export const NODE_HEIGHT = 120;

// ELK layout configuration optimized for our agent -> MCP -> tools hierarchy
const layoutOptions = {
  "elk.algorithm": "layered",
  "elk.direction": "DOWN",
  "elk.layered.spacing.edgeNodeBetweenLayers": "80",
  "elk.spacing.nodeNode": "50", 
  "elk.layered.nodePlacement.strategy": "NETWORK_SIMPLEX",
  "elk.separateConnectedComponents": "true",
  "elk.layered.crossingMinimization.strategy": "LAYER_SWEEP",
  "elk.spacing.componentComponent": `${NODE_WIDTH + 50}`,
  "elk.layered.considerModelOrder.strategy": "NODES_AND_EDGES",
};

const elk = new ELK();

// Apply automatic layout to nodes using ELK.js
export const getLayoutedNodes = async (
  nodes: Node[],
  edges: Edge[]
): Promise<Node[]> => {
  if (nodes.length === 0) return nodes;

  const graph = {
    id: "root",
    layoutOptions,
    children: nodes.map((node) => ({
      id: node.id,
      width: NODE_WIDTH,
      height: NODE_HEIGHT,
      properties: {
        "org.eclipse.elk.portConstraints": "FIXED_ORDER",
      },
    })) as ElkNode[],
    edges: edges.map((edge) => ({
      id: edge.id,
      sources: [edge.source],
      targets: [edge.target],
    })),
  };

  try {
    const layoutedGraph = await elk.layout(graph);

    const layoutedNodes = nodes.map((node) => {
      const layoutedNode = layoutedGraph.children?.find(
        (lgNode) => lgNode.id === node.id
      );

      return {
        ...node,
        position: {
          x: layoutedNode?.x ?? node.position.x,
          y: layoutedNode?.y ?? node.position.y,
        },
      };
    });

    return layoutedNodes;
  } catch (error) {
    console.error('Layout failed:', error);
    return nodes; // Return original nodes if layout fails
  }
};