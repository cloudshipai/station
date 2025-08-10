export const SITE = {
  title: 'Station Documentation',
  description: 'Lightweight Runtime for Deployable Sub-Agents - Secure, self-hosted platform for building and deploying intelligent sub-agents.',
  defaultLanguage: 'en-us'
} as const

export const OPEN_GRAPH = {
  image: {
    src: 'default-og-image.png',
    alt: 'Station logo - Lightweight Runtime for Deployable Sub-Agents'
  },
  twitter: 'cloudshipai'
}

export const KNOWN_LANGUAGES = {
  English: 'en'
} as const
export const KNOWN_LANGUAGE_CODES = Object.values(KNOWN_LANGUAGES)

export const EDIT_URL = `https://github.com/cloudshipai/station/tree/main/docs/site`

export const COMMUNITY_INVITE_URL = `https://discord.gg/station-ai`

// See "Algolia" section of the README for more information.
export const ALGOLIA = {
  indexName: 'XXXXXXXXXX',
  appId: 'XXXXXXXXXX',
  apiKey: 'XXXXXXXXXX'
}

export type Sidebar = Record<
  (typeof KNOWN_LANGUAGE_CODES)[number],
  Record<string, { text: string; link: string }[]>
>
export const SIDEBAR: Sidebar = {
  en: {
    'Getting Started': [
      { text: 'Introduction', link: 'en/introduction' },
      { text: 'Why Station', link: 'en/why-station' },
      { text: 'Quick Start', link: 'en/quickstart' },
      { text: 'Installation', link: 'en/installation' }
    ],
    'Core Concepts': [
      { text: 'Architecture', link: 'en/architecture' },
      { text: 'Intelligent Agents', link: 'en/intelligent-agents' },
      { text: 'MCP Integration', link: 'en/mcp-integration' },
      { text: 'Environment Management', link: 'en/environments' }
    ],
    'Templates & Bundles': [
      { text: 'Template System', link: 'en/templates' },
      { text: 'Bundle Registry', link: 'en/registry' },
      { text: 'Creating Bundles', link: 'en/creating-bundles' },
      { text: 'Publishing Bundles', link: 'en/publishing-bundles' }
    ],
    'Use Cases': [
      { text: 'Infrastructure Monitoring', link: 'en/use-cases/monitoring' },
      { text: 'Deployment Automation', link: 'en/use-cases/deployment' },
      { text: 'Security Operations', link: 'en/use-cases/security' }
    ],
    'Deployment': [
      { text: 'Production Setup', link: 'en/deployment/production' },
      { text: 'Docker Deployment', link: 'en/deployment/docker' },
      { text: 'Database Replication', link: 'en/deployment/replication' },
      { text: 'Security Configuration', link: 'en/deployment/security' }
    ],
    'API Reference': [
      { text: 'CLI Commands', link: 'en/api/cli' },
      { text: 'REST API', link: 'en/api/rest' },
      { text: 'WebSocket API', link: 'en/api/websocket' },
      { text: 'MCP Tools', link: 'en/api/mcp-tools' }
    ]
  }
}
