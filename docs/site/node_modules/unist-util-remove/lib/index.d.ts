export function remove(node: Node, test?: Test): undefined
export function remove(
  node: Node,
  options: Options | null | undefined,
  test?: Test
): undefined
export type Node = import('unist').Node
export type Parent = import('unist').Parent
export type Test = import('unist-util-is').Test
/**
 * Configuration.
 */
export type Options = {
  /**
   * Whether to drop parent nodes if they had children, but all their children
   * were filtered out (default: `true`).
   */
  cascade?: boolean | null | undefined
}
