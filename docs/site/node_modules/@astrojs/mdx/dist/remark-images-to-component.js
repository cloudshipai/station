import { visit } from "unist-util-visit";
import { jsToTreeNode } from "./utils.js";
const ASTRO_IMAGE_ELEMENT = "astro-image";
const ASTRO_IMAGE_IMPORT = "__AstroImage__";
const USES_ASTRO_IMAGE_FLAG = "__usesAstroImage";
function remarkImageToComponent() {
  return function(tree, file) {
    if (!file.data.imagePaths)
      return;
    const importsStatements = [];
    const importedImages = /* @__PURE__ */ new Map();
    visit(tree, "image", (node, index, parent) => {
      if (file.data.imagePaths?.has(node.url)) {
        let importName = importedImages.get(node.url);
        if (!importName) {
          importName = `__${importedImages.size}_${node.url.replace(/\W/g, "_")}__`;
          importsStatements.push({
            type: "mdxjsEsm",
            value: "",
            data: {
              estree: {
                type: "Program",
                sourceType: "module",
                body: [
                  {
                    type: "ImportDeclaration",
                    source: {
                      type: "Literal",
                      value: node.url,
                      raw: JSON.stringify(node.url)
                    },
                    specifiers: [
                      {
                        type: "ImportDefaultSpecifier",
                        local: { type: "Identifier", name: importName }
                      }
                    ]
                  }
                ]
              }
            }
          });
          importedImages.set(node.url, importName);
        }
        const componentElement = {
          name: ASTRO_IMAGE_ELEMENT,
          type: "mdxJsxFlowElement",
          attributes: [
            {
              name: "src",
              type: "mdxJsxAttribute",
              value: {
                type: "mdxJsxAttributeValueExpression",
                value: importName,
                data: {
                  estree: {
                    type: "Program",
                    sourceType: "module",
                    comments: [],
                    body: [
                      {
                        type: "ExpressionStatement",
                        expression: { type: "Identifier", name: importName }
                      }
                    ]
                  }
                }
              }
            },
            { name: "alt", type: "mdxJsxAttribute", value: node.alt || "" }
          ],
          children: []
        };
        if (node.title) {
          componentElement.attributes.push({
            type: "mdxJsxAttribute",
            name: "title",
            value: node.title
          });
        }
        if (node.data && node.data.hProperties) {
          const createArrayAttribute = (name, values) => {
            return {
              type: "mdxJsxAttribute",
              name,
              value: {
                type: "mdxJsxAttributeValueExpression",
                value: name,
                data: {
                  estree: {
                    type: "Program",
                    body: [
                      {
                        type: "ExpressionStatement",
                        expression: {
                          type: "ArrayExpression",
                          elements: values.map((value) => ({
                            type: "Literal",
                            value,
                            raw: String(value)
                          }))
                        }
                      }
                    ],
                    sourceType: "module",
                    comments: []
                  }
                }
              }
            };
          };
          Object.entries(node.data.hProperties).forEach(
            ([key, value]) => {
              if (Array.isArray(value)) {
                componentElement.attributes.push(createArrayAttribute(key, value));
              } else {
                componentElement.attributes.push({
                  name: key,
                  type: "mdxJsxAttribute",
                  value: String(value)
                });
              }
            }
          );
        }
        parent.children.splice(index, 1, componentElement);
      }
    });
    tree.children.unshift(...importsStatements);
    tree.children.unshift(
      jsToTreeNode(`import { Image as ${ASTRO_IMAGE_IMPORT} } from "astro:assets";`)
    );
    tree.children.push(jsToTreeNode(`export const ${USES_ASTRO_IMAGE_FLAG} = true`));
  };
}
export {
  ASTRO_IMAGE_ELEMENT,
  ASTRO_IMAGE_IMPORT,
  USES_ASTRO_IMAGE_FLAG,
  remarkImageToComponent
};
