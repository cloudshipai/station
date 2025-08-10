import { visit } from "estree-util-visit";
import { toHtml } from "hast-util-to-html";
const exportConstComponentsRe = /export\s+const\s+components\s*=/;
function rehypeOptimizeStatic(options) {
  return (tree) => {
    const customComponentNames = new Set(options?.customComponentNames);
    for (const child of tree.children) {
      if (child.type === "mdxjsEsm" && exportConstComponentsRe.test(child.value)) {
        const objectPropertyNodes = child.data.estree.body[0]?.declarations?.[0]?.init?.properties;
        if (objectPropertyNodes) {
          for (const objectPropertyNode of objectPropertyNodes) {
            const componentName = objectPropertyNode.key?.name ?? objectPropertyNode.key?.value;
            if (componentName) {
              customComponentNames.add(componentName);
            }
          }
        }
      }
    }
    const allPossibleElements = /* @__PURE__ */ new Set();
    const elementStack = [];
    visit(tree, {
      enter(node) {
        const isCustomComponent = node.tagName && customComponentNames.has(node.tagName);
        if (node.type.startsWith("mdx") || isCustomComponent) {
          for (const el of elementStack) {
            allPossibleElements.delete(el);
          }
          elementStack.length = 0;
        }
        if (node.type === "element" || node.type === "mdxJsxFlowElement") {
          elementStack.push(node);
          allPossibleElements.add(node);
        }
      },
      leave(node, _, __, parents) {
        if (node.type === "element" || node.type === "mdxJsxFlowElement") {
          elementStack.pop();
          const parent = parents[parents.length - 1];
          if (allPossibleElements.has(parent)) {
            allPossibleElements.delete(node);
          }
        }
      }
    });
    for (const el of allPossibleElements) {
      if (el.type === "mdxJsxFlowElement") {
        el.attributes.push({
          type: "mdxJsxAttribute",
          name: "set:html",
          value: toHtml(el.children)
        });
      } else {
        el.properties["set:html"] = toHtml(el.children);
      }
      el.children = [];
    }
  };
}
export {
  rehypeOptimizeStatic
};
