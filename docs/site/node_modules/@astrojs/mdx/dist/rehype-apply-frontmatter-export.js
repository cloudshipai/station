import { InvalidAstroDataError } from "@astrojs/markdown-remark";
import { safelyGetAstroData } from "@astrojs/markdown-remark/dist/internal.js";
import { jsToTreeNode } from "./utils.js";
function rehypeApplyFrontmatterExport() {
  return function(tree, vfile) {
    const astroData = safelyGetAstroData(vfile.data);
    if (astroData instanceof InvalidAstroDataError)
      throw new Error(
        // Copied from Astro core `errors-data`
        // TODO: find way to import error data from core
        '[MDX] A remark or rehype plugin attempted to inject invalid frontmatter. Ensure "astro.frontmatter" is set to a valid JSON object that is not `null` or `undefined`.'
      );
    const { frontmatter } = astroData;
    const exportNodes = [
      jsToTreeNode(`export const frontmatter = ${JSON.stringify(frontmatter)};`)
    ];
    if (frontmatter.layout) {
      exportNodes.unshift(
        jsToTreeNode(
          /** @see 'vite-plugin-markdown' for layout props reference */
          `import { jsx as layoutJsx } from 'astro/jsx-runtime';

				export default async function ({ children }) {
					const Layout = (await import(${JSON.stringify(frontmatter.layout)})).default;
					const { layout, ...content } = frontmatter;
					content.file = file;
					content.url = url;
					return layoutJsx(Layout, {
						file,
						url,
						content,
						frontmatter: content,
						headings: getHeadings(),
						'server:root': true,
						children,
					});
				};`
        )
      );
    }
    tree.children = exportNodes.concat(tree.children);
  };
}
export {
  rehypeApplyFrontmatterExport
};
