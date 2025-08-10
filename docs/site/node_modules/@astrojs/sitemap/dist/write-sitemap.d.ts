import type { AstroConfig } from 'astro';
import type { SitemapItem } from './index.js';
type WriteSitemapConfig = {
    filenameBase: string;
    hostname: string;
    sitemapHostname?: string;
    sourceData: SitemapItem[];
    destinationDir: string;
    publicBasePath?: string;
    limit?: number;
    xslURL?: string;
};
export declare function writeSitemap({ filenameBase, hostname, sitemapHostname, sourceData, destinationDir, limit, publicBasePath, xslURL: xslUrl, }: WriteSitemapConfig, astroConfig: AstroConfig): Promise<void>;
export {};
