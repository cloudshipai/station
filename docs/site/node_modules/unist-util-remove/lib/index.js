/**
 * @typedef {import('unist').Node} Node
 * @typedef {import('unist').Parent} Parent
 * @typedef {import('unist-util-is').Test} Test
 */

/**
 * @typedef Options
 *   Configuration.
 * @property {boolean | null | undefined} [cascade=true]
 *   Whether to drop parent nodes if they had children, but all their children
 *   were filtered out (default: `true`).
 */

import {convert} from 'unist-util-is'

/**
 * Change the given `tree` by removing all nodes that pass `test`.
 *
 * `tree` itself is never tested.
 * The tree is walked in preorder (NLR), visiting the node itself, then its
 * head, etc.
 *
 * @overload
 * @param {Node} node
 * @param {Test} [test]
 * @returns {undefined}
 *
 * @overload
 * @param {Node} node
 * @param {Options | null | undefined} options
 * @param {Test} [test]
 * @returns {undefined}
 *
 * @param {Node} tree
 *   Tree to change.
 * @param {Options | Test} options
 *   Configuration (optional).
 * @param {Test} [test]
 *   `unist-util-is` compatible test.
 * @returns {undefined}
 *   Nothing.
 */
export function remove(tree, options, test) {
  const is = convert(test || options)
  let cascade = true

  if (
    options &&
    typeof options === 'object' &&
    'cascade' in options &&
    typeof options.cascade === 'boolean'
  ) {
    cascade = options.cascade
  }

  preorder(tree)

  /**
   * Check and remove nodes recursively in preorder.
   * For each composite node, modify its children array in-place.
   *
   * @param {Node} node
   * @param {number | undefined} [index]
   * @param {Parent | undefined} [parent]
   * @returns {boolean}
   */
  function preorder(node, index, parent) {
    if (node !== tree && is(node, index, parent)) {
      return false
    }

    if ('children' in node && Array.isArray(node.children)) {
      const nodeAsParent = /** @type {Parent} */ (node)
      const children = nodeAsParent.children
      let oldChildIndex = -1
      let newChildIndex = 0

      if (children.length > 0) {
        // Move all living children to the beginning of the children array.
        while (++oldChildIndex < children.length) {
          if (preorder(children[oldChildIndex], oldChildIndex, nodeAsParent)) {
            children[newChildIndex++] = children[oldChildIndex]
          }
        }

        // Cascade delete.
        if (node !== tree && cascade && !newChildIndex) {
          return false
        }

        // Drop other nodes.
        children.length = newChildIndex
      }
    }

    return true
  }
}
