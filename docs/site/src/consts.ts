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
  Record<string, SidebarSection>
>
export type SidebarItem = {
  text: string;
  link: string;
  badge?: string;
  children?: SidebarItem[];
}

export type SidebarSection = {
  text: string;
  collapsed?: boolean;
  items: SidebarItem[];
}

export const SIDEBAR: Sidebar = {
  en: {
    'Getting Started': {
      collapsed: false,
      items: [
        { text: 'Introduction', link: 'en/introduction' },
        { text: 'Quick Start', link: 'en/quickstart', badge: 'Start Here' },
        { text: 'Installation', link: 'en/installation' },
        { text: 'Authentication', link: 'en/authentication' },
        { text: 'Why Station', link: 'en/why-station' }
      ]
    },
    'MCP Integration': {
      collapsed: false,
      items: [
        { text: 'Overview', link: 'en/mcp/overview' },
        { text: 'Claude Desktop Setup', link: 'en/mcp/claude-desktop' },
        { text: 'MCP Tools Reference', link: 'en/mcp/tools' },
        { text: 'Other MCP Clients', link: 'en/mcp/other-clients' }
      ]
    },
    'Core Features': {
      collapsed: false,
      items: [
        { text: 'Sandbox Execution', link: 'en/sandbox' },
        { text: 'Scheduling', link: 'en/scheduling' },
        { text: 'Database', link: 'en/database' },
        { text: 'Webhooks', link: 'en/webhooks' }
      ]
    },
    'Agent Management': {
      collapsed: false,
      items: [
        { text: 'Overview', link: 'en/agents/overview' },
        { text: 'Creating Agents', link: 'en/agents/creating' },
        { text: 'Agent Configuration', link: 'en/agents/config' },
        { text: 'Environment Isolation', link: 'en/agents/environments' },
        { text: 'Monitoring & Logs', link: 'en/agents/monitoring' }
      ]
    },
    'Advanced Features': {
      collapsed: true,
      items: [
        { text: 'Coding Backend', link: 'en/coding-backend', badge: 'New' },
        { text: 'Fakers (Mock Data)', link: 'en/fakers' },
        { text: 'OpenAPI to MCP', link: 'en/openapi-mcp' },
        { text: 'GitOps Workflow', link: 'en/gitops' },
        { text: 'CloudShip Integration', link: 'en/cloudship' }
      ]
    },
    'CLI Management': {
      collapsed: true,
      items: [
        { text: 'Setup & Configuration', link: 'en/cli/setup' },
        { 
          text: 'Tool Management', 
          link: 'en/cli/tools',
          children: [
            { text: 'Installing Tools', link: 'en/cli/tools/installing' },
            { text: 'Custom Tools', link: 'en/cli/tools/custom' }
          ]
        },
        { text: 'Template System', link: 'en/cli/templates' },
        { text: 'Advanced Commands', link: 'en/cli/advanced' }
      ]
    },
    'Templates & Bundles': {
      collapsed: true,
      items: [
        { text: 'Bundle Registry', link: 'en/bundles/registry', badge: 'Browse' },
        { text: 'Creating Bundles', link: 'en/bundles/creating' },
        { text: 'Publishing Bundles', link: 'en/bundles/publishing' }
      ]
    },
    'Deployment': {
      collapsed: true,
      items: [
        { text: 'Docker Deployment', link: 'en/deployment/docker' },
        { text: 'Production Setup', link: 'en/deployment/production' },
        { text: 'Security Configuration', link: 'en/deployment/security' },
        { text: 'Observability', link: 'en/observability' },
        { text: 'Monitoring & Metrics', link: 'en/deployment/monitoring' }
      ]
    },
    'CI/CD & Operations': {
      collapsed: true,
      items: [
        { text: 'CI/CD Integration', link: 'en/ci-cd-integration', badge: 'Production Ready' }
      ]
    },
    'Examples': {
      collapsed: true,
      items: [
        { text: 'SRE Team Tutorial', link: 'en/examples/sre-team' }
      ]
    },
    'Reference': {
      collapsed: true,
      items: [
        { text: 'Architecture', link: 'en/architecture' }
      ]
    }
  }
}
