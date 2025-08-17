import ELK, { type ElkNode } from "elkjs/lib/elk.bundled.js";
import type { Node, Edge } from '@xyflow/react';

// Node dimensions (using larger sizes like Langflow for better visibility)
export const NODE_WIDTH = 300;
export const NODE_HEIGHT = 150;

// ELK layout configuration based on Langflow's proven approach
const layoutOptions = {
  "elk.algorithm": "layered",
  "elk.direction": "RIGHT", // Left-to-right like Langflow
  "elk.components.direction": "DOWN",
  "elk.layered.spacing.edgeNodeBetweenLayers": "80",
  "elk.spacing.nodeNode": "60", 
  "elk.layered.nodePlacement.strategy": "NETWORK_SIMPLEX",
  "elk.separateConnectedComponents": "true",
  "elk.layered.crossingMinimization.strategy": "LAYER_SWEEP",
  "elk.spacing.componentComponent": `${NODE_WIDTH}`,
  "elk.layered.considerModelOrder.strategy": "NODES_AND_EDGES",
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
      // Create ports for better edge connections (like Langflow)
      const targetPorts = edges
        .filter((e) => e.source === node.id)
        .map((e) => ({
          id: e.sourceHandle || `${e.id}-source`,
          properties: {
            side: "EAST", // Right side for outgoing connections
          },
        }));

      const sourcePorts = edges
        .filter((e) => e.target === node.id)
        .map((e) => ({
          id: e.targetHandle || `${e.id}-target`,
          properties: {
            side: "WEST", // Left side for incoming connections
          },
        }));

      return {
        id: node.id,
        width: NODE_WIDTH,
        height: NODE_HEIGHT,
        properties: {
          "org.eclipse.elk.portConstraints": "FIXED_ORDER",
        },
        // Include node ID and all ports for better edge routing
        ports: [{ id: node.id }, ...targetPorts, ...sourcePorts],
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