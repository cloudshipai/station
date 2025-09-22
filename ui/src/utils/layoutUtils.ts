import ELK, { type ElkNode } from "elkjs/lib/elk.bundled.js";
import type { Node, Edge } from '@xyflow/react';

// Node dimensions optimized for concise cards
export const NODE_WIDTH = 280;
export const NODE_HEIGHT = 130;

// ELK layout configuration optimized for proper spacing
const layoutOptions = {
  "elk.algorithm": "layered",
  "elk.direction": "RIGHT",
  "elk.layered.spacing.edgeNodeBetweenLayers": "200",
  "elk.spacing.nodeNode": "150",
  "elk.layered.spacing.nodeNodeBetweenLayers": "200",
  "elk.layered.nodePlacement.strategy": "NETWORK_SIMPLEX",
  "elk.separateConnectedComponents": "true",
  "elk.layered.crossingMinimization.strategy": "LAYER_SWEEP",
  "elk.spacing.componentComponent": "250",
  "elk.layered.considerModelOrder.strategy": "NODES_AND_EDGES",
  "elk.padding": "[top=80,left=80,bottom=80,right=80]",
  "elk.partitioning.activate": "true",
  "elk.aspectRatio": "1.5",
};

const elk = new ELK();

// Apply automatic layout to nodes using ELK.js (based on Langflow's implementation)
export const getLayoutedNodes = async (
  nodes: Node[],
  edges: Edge[]
): Promise<Node[]> => {
  if (nodes.length === 0) return nodes;

  console.log('Starting layout with nodes:', nodes.length, 'edges:', edges.length);

  const graph = {
    id: "root",
    layoutOptions,
    children: nodes.map((node) => {
      return {
        id: node.id,
        width: NODE_WIDTH,
        height: NODE_HEIGHT,
        properties: {
          "org.eclipse.elk.portConstraints": "FREE",
          "org.eclipse.elk.nodeLabels.placement": "INSIDE V_CENTER H_CENTER",
        },
        ports: [
          {
            id: `${node.id}-west`,
            properties: {
              side: "WEST",
              "org.eclipse.elk.port.anchor": "(0,0.5)",
            },
          },
          {
            id: `${node.id}-east`,
            properties: {
              side: "EAST",
              "org.eclipse.elk.port.anchor": "(1,0.5)",
            },
          },
        ],
      };
    }) as ElkNode[],
    edges: edges.map((edge) => ({
      id: edge.id,
      sources: [edge.sourceHandle || edge.source],
      targets: [edge.targetHandle || edge.target],
    })),
  };

  try {
    console.log('ELK graph:', JSON.stringify(graph, null, 2));
    const layoutedGraph = await elk.layout(graph);
    console.log('Layout completed successfully');

    const layoutedNodes = nodes.map((node) => {
      const layoutedNode = layoutedGraph.children?.find(
        (lgNode) => lgNode.id === node.id
      );

      const newPosition = {
        x: layoutedNode?.x ?? 0,
        y: layoutedNode?.y ?? 0,
      };

      console.log(`Node ${node.id}: positioned at (${newPosition.x}, ${newPosition.y})`);

      return {
        ...node,
        position: newPosition,
      };
    });

    return layoutedNodes;
  } catch (error) {
    console.error('Layout failed:', error);
    return nodes; // Return original nodes if layout fails
  }
};

// Layout function for environments page that returns both nodes and edges
export const layoutElements = async (
  nodes: Node[],
  edges: Edge[],
  direction: string = 'RIGHT'
): Promise<{ nodes: Node[]; edges: Edge[] }> => {
  const layoutedNodes = await getLayoutedNodes(nodes, edges);
  return {
    nodes: layoutedNodes,
    edges: edges
  };
};