import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  base: '/',
  build: {
    outDir: '../internal/ui/static',
    emptyOutDir: false,  // Don't delete logos, favicons, images
    rollupOptions: {
      output: {
        entryFileNames: `assets/[name]-[hash]-${Date.now()}.js`,
        chunkFileNames: `assets/[name]-[hash]-${Date.now()}.js`,
        assetFileNames: `assets/[name]-[hash]-${Date.now()}.[ext]`,
        // Manual chunks to prevent TDZ errors from circular dependencies
        // See: https://github.com/cloudshipai/station/issues/82
        manualChunks: (id) => {
          // React core must be in its own chunk to initialize first
          if (id.includes('node_modules/react/') ||
              id.includes('node_modules/react-dom/') ||
              id.includes('node_modules/scheduler/')) {
            return 'react-vendor';
          }

          // Radix UI primitives - used across many components
          if (id.includes('node_modules/@radix-ui/')) {
            return 'radix-ui';
          }

          // React Flow / xyflow - complex graph visualization
          if (id.includes('node_modules/@xyflow/') ||
              id.includes('node_modules/reactflow/')) {
            return 'react-flow';
          }

          // Monaco Editor - large, can be lazy loaded
          if (id.includes('node_modules/monaco-') ||
              id.includes('node_modules/@monaco-editor/')) {
            return 'monaco';
          }

          // Recharts and charting dependencies
          if (id.includes('node_modules/recharts/') ||
              id.includes('node_modules/d3-') ||
              id.includes('node_modules/victory-')) {
            return 'charts';
          }

          // TanStack Query - state management
          if (id.includes('node_modules/@tanstack/')) {
            return 'tanstack';
          }

          // Zustand state management
          if (id.includes('node_modules/zustand/')) {
            return 'zustand';
          }

          // All other node_modules go to vendor chunk
          if (id.includes('node_modules/')) {
            return 'vendor';
          }
        }
      }
    }
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8585',
        changeOrigin: true,
      },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
})
