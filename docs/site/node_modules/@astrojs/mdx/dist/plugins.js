import {
  rehypeHeadingIds,
  rehypePrism,
  rehypeShiki,
  remarkCollectImages
} from "@astrojs/markdown-remark";
import { createProcessor, nodeTypes } from "@mdx-js/mdx";
import rehypeRaw from "rehype-raw";
import remarkGfm from "remark-gfm";
import remarkSmartypants from "remark-smartypants";
import { SourceMapGenerator } from "source-map";
import { rehypeApplyFrontmatterExport } from "./rehype-apply-frontmatter-export.js";
import { rehypeInjectHeadingsExport } from "./rehype-collect-headings.js";
import rehypeMetaString from "./rehype-meta-string.js";
import { rehypeOptimizeStatic } from "./rehype-optimize-static.js";
import { remarkImageToComponent } from "./remark-images-to-component.js";
const isPerformanceBenchmark = Boolean(process.env.ASTRO_PERFORMANCE_BENCHMARK);
function createMdxProcessor(mdxOptions, extraOptions) {
  return createProcessor({
    remarkPlugins: getRemarkPlugins(mdxOptions),
    rehypePlugins: getRehypePlugins(mdxOptions),
    recmaPlugins: mdxOptions.recmaPlugins,
    remarkRehypeOptions: mdxOptions.remarkRehype,
    jsx: true,
    jsxImportSource: "astro",
    // Note: disable `.md` (and other alternative extensions for markdown files like `.markdown`) support
    format: "mdx",
    mdExtensions: [],
    elementAttributeNameCase: "html",
    SourceMapGenerator: extraOptions.sourcemap ? SourceMapGenerator : void 0
  });
}
function getRemarkPlugins(mdxOptions) {
  let remarkPlugins = [];
  if (!isPerformanceBenchmark) {
    if (mdxOptions.gfm) {
      remarkPlugins.push(remarkGfm);
    }
    if (mdxOptions.smartypants) {
      remarkPlugins.push(remarkSmartypants);
    }
  }
  remarkPlugins.push(...mdxOptions.remarkPlugins, remarkCollectImages, remarkImageToComponent);
  return remarkPlugins;
}
function getRehypePlugins(mdxOptions) {
  let rehypePlugins = [
    // ensure `data.meta` is preserved in `properties.metastring` for rehype syntax highlighters
    rehypeMetaString,
    // rehypeRaw allows custom syntax highlighters to work without added config
    [rehypeRaw, { passThrough: nodeTypes }]
  ];
  if (!isPerformanceBenchmark) {
    if (mdxOptions.syntaxHighlight === "shiki") {
      rehypePlugins.push([rehypeShiki, mdxOptions.shikiConfig]);
    } else if (mdxOptions.syntaxHighlight === "prism") {
      rehypePlugins.push(rehypePrism);
    }
  }
  rehypePlugins.push(...mdxOptions.rehypePlugins);
  if (!isPerformanceBenchmark) {
    rehypePlugins.push(rehypeHeadingIds, rehypeInjectHeadingsExport);
  }
  rehypePlugins.push(rehypeApplyFrontmatterExport);
  if (mdxOptions.optimize) {
    const options = mdxOptions.optimize === true ? void 0 : mdxOptions.optimize;
    rehypePlugins.push([rehypeOptimizeStatic, options]);
  }
  return rehypePlugins;
}
export {
  createMdxProcessor
};
