import starlightPlugin from '@astrojs/starlight-tailwind';

/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{astro,html,js,jsx,md,mdx,svelte,ts,tsx,vue}'],
  theme: {
    extend: {
      colors: {
        // Station brand colors
        'station-primary': '#667eea',
        'station-secondary': '#764ba2',
        'station-accent': '#f093fb',
      },
      fontFamily: {
        'mono': ['SF Mono', 'Monaco', 'Cascadia Code', 'monospace'],
      },
    },
  },
  plugins: [starlightPlugin()],
};