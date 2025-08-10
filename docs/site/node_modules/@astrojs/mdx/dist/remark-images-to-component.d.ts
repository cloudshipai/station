import type { MarkdownVFile } from '@astrojs/markdown-remark';
export declare const ASTRO_IMAGE_ELEMENT = "astro-image";
export declare const ASTRO_IMAGE_IMPORT = "__AstroImage__";
export declare const USES_ASTRO_IMAGE_FLAG = "__usesAstroImage";
export declare function remarkImageToComponent(): (tree: any, file: MarkdownVFile) => void;
