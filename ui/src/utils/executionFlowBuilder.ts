import type { Node, Edge } from '@xyflow/react';
import type { JaegerTrace, JaegerSpan } from '../types/station';
import type { ExecutionFlowNodeData } from '../components/nodes/ExecutionFlowNode';

interface BuildExecutionFlowGraphParams {
  traceData: JaegerTrace;
  startX?: number;
  startY?: number;
  activeSpanIds?: string[];
  horizontal?: boolean; // NEW: horizontal flow layout
}

export const buildExecutionFlowGraph = ({
  traceData,
  startX = 100,
  startY = 100,
  activeSpanIds = [],
  horizontal = false,
}: BuildExecutionFlowGraphParams): { nodes: Node[]; edges: Edge[] } => {
  if (!traceData || !traceData.spans || traceData.spans.length === 0) {
    return { nodes: [], edges: [] };
  }

  const nodes: Node[] = [];
  const edges: Edge[] = [];

  // Filter spans to only meaningful tool calls (matching ToolCallsView logic)
  // This captures: __agent_*, __read_file, faker.*, generate, etc.
  // EXCLUDES: agent.execute_with_run_id and other low-level wrappers
  const relevantSpans = traceData.spans
    .filter((span: JaegerSpan) => {
      const op = span.operationName;
      
      // INCLUDE: Actual tool calls with __ prefix (including __agent_*)
      if (op.startsWith('__')) {
        return true;
      }
      
      // INCLUDE: Faker tool calls
      if (op.startsWith('faker.')) {
        return true;
      }
      
      // INCLUDE: LLM generate calls
      if (op === 'generate') {
        return true;
      }
      
      // INCLUDE: Tool operations (but not agent.execute_with_run_id wrappers)
      if (op.includes('tool.') && !op.includes('agent.')) {
        return true;
      }
      
      // EXCLUDE: Low-level agent execution wrappers
      return false;
    })
    .sort((a, b) => a.startTime - b.startTime);

  if (relevantSpans.length === 0) {
    return { nodes: [], edges: [] };
  }

  const runStartTime = traceData.spans[0]?.startTime || 0;

  // Create START node
  const startNodeId = 'exec-start';
  nodes.push({
    id: startNodeId,
    type: 'executionFlow',
    position: { x: startX, y: startY },
    data: {
      label: 'START',
      type: 'start',
      duration: 0,
      status: 'success',
      startTime: 0,
      spanID: 'start',
      isActive: false,
    } as ExecutionFlowNodeData,
  });

  let previousNodeId = startNodeId;
  let xOffset = horizontal ? 280 : 0; // Horizontal spacing (280px between nodes)
  let yOffset = horizontal ? 0 : 120; // Vertical spacing

  // Create nodes for each span
  relevantSpans.forEach((span: JaegerSpan, index: number) => {
    const nodeId = `exec-${span.spanID}`;
    const hasError = span.tags?.some(t => t.key === 'error' || t.key === 'error.message');
    
    // Determine node type
    let nodeType: 'tool' | 'llm' = 'tool';
    if (span.operationName === 'generate') {
      nodeType = 'llm';
    }

    // Extract tool parameters and LLM data from tags
    const toolParams: Record<string, any> = {};
    let toolResult: any = null;
    let llmPrompt: string | undefined;
    let llmTokens: { input: number; output: number } | undefined;
    let llmModel: string | undefined;
    
    if (span.tags) {
      span.tags.forEach(tag => {
        // Tool input parameters
        if (tag.key.startsWith('input.')) {
          const paramName = tag.key.replace('input.', '');
          toolParams[paramName] = tag.value;
        }
        if (tag.key === 'tool.input' || tag.key === 'genkit:input') {
          try {
            const parsed = typeof tag.value === 'string' ? JSON.parse(tag.value) : tag.value;
            Object.assign(toolParams, parsed);
          } catch (e) {
            toolParams['input'] = tag.value;
          }
        }
        
        // Tool output
        if (tag.key === 'tool.output' || tag.key === 'genkit:output') {
          toolResult = tag.value;
        }
        
        // LLM-specific tags
        if (tag.key === 'llm.prompt' || tag.key === 'genkit:input') {
          llmPrompt = String(tag.value);
        }
        if (tag.key === 'llm.tokens.input' || tag.key === 'gen_ai.usage.input_tokens') {
          if (!llmTokens) llmTokens = { input: 0, output: 0 };
          llmTokens.input = Number(tag.value) || 0;
        }
        if (tag.key === 'llm.tokens.output' || tag.key === 'gen_ai.usage.output_tokens') {
          if (!llmTokens) llmTokens = { input: 0, output: 0 };
          llmTokens.output = Number(tag.value) || 0;
        }
        if (tag.key === 'llm.model' || tag.key === 'gen_ai.request.model') {
          llmModel = String(tag.value);
        }
      });
    }

    // Create a friendly label - prefer tool.name tag over operation name
    let label = span.operationName;
    
    // Check for tool.name tag (captures agent delegations like __agent_math_calculator)
    const toolNameTag = span.tags?.find(t => t.key === 'tool.name' || t.key === 'tool');
    if (toolNameTag && String(toolNameTag.value).startsWith('__agent_')) {
      label = String(toolNameTag.value);
    }
    
    // Clean up label
    if (label.startsWith('__agent_')) {
      label = label.replace('__agent_', ''); // Show "math_calculator" instead of "__agent_math_calculator"
    } else if (label.startsWith('__')) {
      label = label.substring(2); // Remove __ prefix for regular tools
    }
    if (label.startsWith('faker.')) {
      label = label.replace('faker.', ''); // Remove faker. prefix
    }
    
    // Truncate long labels
    if (label.length > 30) {
      label = label.substring(0, 27) + '...';
    }

    const nodeData: ExecutionFlowNodeData = {
      label,
      type: nodeType,
      duration: span.duration,
      status: hasError ? 'error' : 'success',
      toolName: span.operationName,
      toolParams: Object.keys(toolParams).length > 0 ? toolParams : undefined,
      toolResult,
      llmPrompt,
      llmTokens,
      llmModel,
      startTime: span.startTime - runStartTime,
      spanID: span.spanID,
      isActive: activeSpanIds.includes(span.spanID),
    };

    // Calculate position based on layout direction
    const position = horizontal
      ? { x: startX + xOffset, y: startY }
      : { x: startX + ((index % 3) * 50 - 50), y: startY + yOffset };
    
    nodes.push({
      id: nodeId,
      type: 'executionFlow',
      position,
      data: nodeData,
    });

    // Create edge from previous node
    edges.push({
      id: `edge-${previousNodeId}-${nodeId}`,
      source: previousNodeId,
      target: nodeId,
      type: 'default',
      animated: nodeData.isActive,
      style: {
        stroke: nodeData.isActive ? '#06b6d4' : nodeType === 'llm' ? '#06b6d4' : '#22c55e',
        strokeWidth: nodeData.isActive ? 3 : 2,
        filter: nodeData.isActive 
          ? 'drop-shadow(0 0 8px rgba(6, 182, 212, 0.8))'
          : 'drop-shadow(0 0 4px rgba(34, 197, 94, 0.3))',
      },
    });

    previousNodeId = nodeId;
    if (horizontal) {
      xOffset += 280; // Move right for next node (280px spacing)
    } else {
      yOffset += 120; // Move down for next node
    }
  });

  // Create END node
  const endNodeId = 'exec-end';
  const endPosition = horizontal
    ? { x: startX + xOffset, y: startY }
    : { x: startX, y: startY + yOffset };
    
  nodes.push({
    id: endNodeId,
    type: 'executionFlow',
    position: endPosition,
    data: {
      label: 'END',
      type: 'end',
      duration: 0,
      status: 'success',
      startTime: 0,
      spanID: 'end',
      isActive: false,
    } as ExecutionFlowNodeData,
  });

  // Edge to END node
  edges.push({
    id: `edge-${previousNodeId}-${endNodeId}`,
    source: previousNodeId,
    target: endNodeId,
    type: 'default',
    animated: false,
    style: {
      stroke: '#a855f7',
      strokeWidth: 2,
      filter: 'drop-shadow(0 0 4px rgba(168, 85, 247, 0.3))',
    },
  });

  return { nodes, edges };
};
