import fs from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { markdownConfigDefaults, setVfileFrontmatter } from "@astrojs/markdown-remark";
import astroJSXRenderer from "astro/jsx/renderer.js";
import { parse as parseESM } from "es-module-lexer";
import { VFile } from "vfile";
import { createMdxProcessor } from "./plugins.js";
import {
  ASTRO_IMAGE_ELEMENT,
  ASTRO_IMAGE_IMPORT,
  USES_ASTRO_IMAGE_FLAG
} from "./remark-images-to-component.js";
import { getFileInfo, ignoreStringPlugins, parseFrontmatter } from "./utils.js";
function mdx(partialMdxOptions = {}) {
  return {
    name: "@astrojs/mdx",
    hooks: {
      "astro:config:setup": async (params) => {
        const { updateConfig, config, addPageExtension, addContentEntryType, addRenderer } = params;
        addRenderer(astroJSXRenderer);
        addPageExtension(".mdx");
        addContentEntryType({
          extensions: [".mdx"],
          async getEntryInfo({ fileUrl, contents }) {
            const parsed = parseFrontmatter(contents, fileURLToPath(fileUrl));
            return {
              data: parsed.data,
              body: parsed.content,
              slug: parsed.data.slug,
              rawData: parsed.matter
            };
          },
          contentModuleTypes: await fs.readFile(
            new URL("../template/content-module-types.d.ts", import.meta.url),
            "utf-8"
          ),
          // MDX can import scripts and styles,
          // so wrap all MDX files with script / style propagation checks
          handlePropagation: true
        });
        const extendMarkdownConfig = partialMdxOptions.extendMarkdownConfig ?? defaultMdxOptions.extendMarkdownConfig;
        const mdxOptions = applyDefaultOptions({
          options: partialMdxOptions,
          defaults: markdownConfigToMdxOptions(
            extendMarkdownConfig ? config.markdown : markdownConfigDefaults
          )
        });
        let processor;
        updateConfig({
          vite: {
            plugins: [
              {
                name: "@mdx-js/rollup",
                enforce: "pre",
                buildEnd() {
                  processor = void 0;
                },
                configResolved(resolved) {
                  processor = createMdxProcessor(mdxOptions, {
                    sourcemap: !!resolved.build.sourcemap
                  });
                  const jsxPluginIndex = resolved.plugins.findIndex((p) => p.name === "astro:jsx");
                  if (jsxPluginIndex !== -1) {
                    const myPluginIndex = resolved.plugins.findIndex(
                      (p) => p.name === "@mdx-js/rollup"
                    );
                    if (myPluginIndex !== -1) {
                      const myPlugin = resolved.plugins[myPluginIndex];
                      resolved.plugins.splice(myPluginIndex, 1);
                      resolved.plugins.splice(jsxPluginIndex, 0, myPlugin);
                    }
                  }
                },
                async resolveId(source, importer, options) {
                  if (importer?.endsWith(".mdx") && source[0] !== "/") {
                    let resolved = await this.resolve(source, importer, options);
                    if (!resolved)
                      resolved = await this.resolve("./" + source, importer, options);
                    return resolved;
                  }
                },
                // Override transform to alter code before MDX compilation
                // ex. inject layouts
                async transform(_, id) {
                  if (!id.endsWith(".mdx"))
                    return;
                  const { fileId } = getFileInfo(id, config);
                  const code = await fs.readFile(fileId, "utf-8");
                  const { data: frontmatter, content: pageContent } = parseFrontmatter(code, id);
                  const vfile = new VFile({ value: pageContent, path: id });
                  setVfileFrontmatter(vfile, frontmatter);
                  if (!processor) {
                    return this.error(
                      "MDX processor is not initialized. This is an internal error. Please file an issue."
                    );
                  }
                  try {
                    const compiled = await processor.process(vfile);
                    return {
                      code: String(compiled.value),
                      map: compiled.map
                    };
                  } catch (e) {
                    const err = e;
                    err.name = "MDXError";
                    err.loc = { file: fileId, line: e.line, column: e.column };
                    Error.captureStackTrace(err);
                    throw err;
                  }
                }
              },
              {
                name: "@astrojs/mdx-postprocess",
                // These transforms must happen *after* JSX runtime transformations
                transform(code, id) {
                  if (!id.endsWith(".mdx"))
                    return;
                  const [moduleImports, moduleExports] = parseESM(code);
                  const importsFromJSXRuntime = moduleImports.filter(({ n }) => n === "astro/jsx-runtime").map(({ ss, se }) => code.substring(ss, se));
                  const hasFragmentImport = importsFromJSXRuntime.some(
                    (statement) => /[\s,{](?:Fragment,|Fragment\s*\})/.test(statement)
                  );
                  if (!hasFragmentImport) {
                    code = 'import { Fragment } from "astro/jsx-runtime"\n' + code;
                  }
                  const { fileUrl, fileId } = getFileInfo(id, config);
                  if (!moduleExports.find(({ n }) => n === "url")) {
                    code += `
export const url = ${JSON.stringify(fileUrl)};`;
                  }
                  if (!moduleExports.find(({ n }) => n === "file")) {
                    code += `
export const file = ${JSON.stringify(fileId)};`;
                  }
                  if (!moduleExports.find(({ n }) => n === "Content")) {
                    const hasComponents = moduleExports.find(({ n }) => n === "components");
                    const usesAstroImage = moduleExports.find(
                      ({ n }) => n === USES_ASTRO_IMAGE_FLAG
                    );
                    let componentsCode = `{ Fragment${hasComponents ? ", ...components" : ""}, ...props.components,`;
                    if (usesAstroImage) {
                      componentsCode += ` ${JSON.stringify(ASTRO_IMAGE_ELEMENT)}: ${hasComponents ? "components.img ?? " : ""} props.components?.img ?? ${ASTRO_IMAGE_IMPORT}`;
                    }
                    componentsCode += " }";
                    code = code.replace(
                      "export default function MDXContent",
                      "function MDXContent"
                    );
                    code += `
export const Content = (props = {}) => MDXContent({
											...props,
											components: ${componentsCode},
										});
										export default Content;`;
                  }
                  code += `
Content[Symbol.for('mdx-component')] = true`;
                  code += `
Content[Symbol.for('astro.needsHeadRendering')] = !Boolean(frontmatter.layout);`;
                  code += `
Content.moduleId = ${JSON.stringify(id)};`;
                  return { code, map: null };
                }
              }
            ]
          }
        });
      }
    }
  };
}
const defaultMdxOptions = {
  extendMarkdownConfig: true,
  recmaPlugins: []
};
function markdownConfigToMdxOptions(markdownConfig) {
  return {
    ...defaultMdxOptions,
    ...markdownConfig,
    remarkPlugins: ignoreStringPlugins(markdownConfig.remarkPlugins),
    rehypePlugins: ignoreStringPlugins(markdownConfig.rehypePlugins),
    remarkRehype: markdownConfig.remarkRehype ?? {},
    optimize: false
  };
}
function applyDefaultOptions({
  options,
  defaults
}) {
  return {
    syntaxHighlight: options.syntaxHighlight ?? defaults.syntaxHighlight,
    extendMarkdownConfig: options.extendMarkdownConfig ?? defaults.extendMarkdownConfig,
    recmaPlugins: options.recmaPlugins ?? defaults.recmaPlugins,
    remarkRehype: options.remarkRehype ?? defaults.remarkRehype,
    gfm: options.gfm ?? defaults.gfm,
    smartypants: options.smartypants ?? defaults.smartypants,
    remarkPlugins: options.remarkPlugins ?? defaults.remarkPlugins,
    rehypePlugins: options.rehypePlugins ?? defaults.rehypePlugins,
    shikiConfig: options.shikiConfig ?? defaults.shikiConfig,
    optimize: options.optimize ?? defaults.optimize
  };
}
export {
  mdx as default
};
