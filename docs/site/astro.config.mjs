import { defineConfig } from 'astro/config'

import mdx from '@astrojs/mdx'
import preact from '@astrojs/preact'
import react from '@astrojs/react'
import sitemap from '@astrojs/sitemap'

export default defineConfig({
  integrations: [mdx(), preact(), react(), sitemap()],
  base: `/station/`,
  site: `https://cloudshipai.github.io`
})
