import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import tailwind from '@astrojs/tailwind';

export default defineConfig({
  site: 'https://cloudshipai.github.io',
  base: '/station',
  integrations: [
    tailwind({
      applyBaseStyles: false,
    }),
    starlight({
      title: 'Station Docs',
      description: 'Lightweight Runtime for Deployable Sub-Agents',
      defaultLocale: 'root',
      locales: {
        root: {
          label: 'English',
          lang: 'en',
        },
      },
      logo: {
        src: './src/assets/station-logo.png',
        replacesTitle: true,
      },
      social: {
        github: 'https://github.com/cloudshipai/station',
        discord: 'https://discord.gg/station-ai',
      },
      customCss: [
        './src/styles/custom.css',
      ],
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Introduction', link: '/introduction/' },
            { label: 'Why Station', link: '/why-station/' },
            { label: 'Quick Start', link: '/quickstart/' },
            { label: 'Installation', link: '/installation/' },
          ],
        },
        {
          label: 'Core Concepts',
          items: [
            { label: 'Architecture', link: '/architecture/' },
            { label: 'Intelligent Agents', link: '/intelligent-agents/' },
            { label: 'MCP Integration', link: '/mcp-integration/' },
            { label: 'Environment Management', link: '/environments/' },
          ],
        },
        {
          label: 'Templates & Bundles',
          items: [
            { label: 'Template System', link: '/templates/' },
            { label: 'Bundle Registry', link: '/registry/' },
            { label: 'Creating Bundles', link: '/creating-bundles/' },
            { label: 'Publishing Bundles', link: '/publishing-bundles/' },
          ],
        },
        {
          label: 'Use Cases',
          items: [
            { label: 'Infrastructure Monitoring', link: '/use-cases/monitoring/' },
            { label: 'Deployment Automation', link: '/use-cases/deployment/' },
            { label: 'Security Operations', link: '/use-cases/security/' },
          ],
        },
        {
          label: 'Deployment',
          items: [
            { label: 'Production Setup', link: '/deployment/production/' },
            { label: 'Docker Deployment', link: '/deployment/docker/' },
            { label: 'Database Replication', link: '/deployment/replication/' },
            { label: 'Security Configuration', link: '/deployment/security/' },
          ],
        },
        {
          label: 'API Reference',
          items: [
            { label: 'CLI Commands', link: '/api/cli/' },
            { label: 'REST API', link: '/api/rest/' },
            { label: 'WebSocket API', link: '/api/websocket/' },
            { label: 'MCP Tools', link: '/api/mcp-tools/' },
          ],
        },
      ],
    }),
  ],
});