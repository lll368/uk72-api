import path from 'path'
import { fileURLToPath } from 'url'
import { createRequire } from 'module'
import { defineConfig, loadEnv } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'
import { tanstackRouter } from '@tanstack/router-plugin/rspack'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const require = createRequire(import.meta.url)

// Lazily load Rsdoctor only when explicitly enabled to avoid pulling
// the heavy analyzer toolchain into normal builds / installs.
const enableRsdoctor = process.env.RSDOCTOR === 'true'

export default defineConfig(({ envMode }) => {
  const env = loadEnv({ mode: envMode, prefixes: ['VITE_'] })
  const serverUrl =
    process.env.VITE_REACT_APP_SERVER_URL ||
    env.rawPublicVars.VITE_REACT_APP_SERVER_URL ||
    'http://localhost:3000'

  const isProd = envMode === 'production'
  const prodAssetPrefix = (() => {
    if (!isProd) return ''
    const rawPrefix = (
      process.env.VITE_CDN_URL || env.rawPublicVars.VITE_CDN_URL || ''
    ).trim()
    if (rawPrefix === '') {
      return '/'
    }
    return rawPrefix.endsWith('/') ? rawPrefix : `${rawPrefix}/`
  })()
  const devProxy = Object.fromEntries(
    (['/api', '/mj', '/pg'] as const).map((key) => [
      key,
      { target: serverUrl, changeOrigin: true },
    ]),
  ) as Record<string, { target: string; changeOrigin: boolean }>

  return {
    plugins: [pluginReact()],
    // Rsbuild 2: replaces deprecated `performance.chunkSplit` (RSPack 2 aligned)
    splitChunks: {
      preset: 'default',
      cacheGroups: {
        'vendor-react': {
          test: /node_modules[\\/](react|react-dom)[\\/]/,
          name: 'vendor-react',
          chunks: 'all',
          priority: 0,
          enforce: true,
        },
        'vendor-ui-primitives': {
          test: /node_modules[\\/](@base-ui|@radix-ui)[\\/]/,
          name: 'vendor-ui-primitives',
          chunks: 'all',
          priority: 0,
          enforce: true,
        },
        'vendor-tanstack': {
          test: /node_modules[\\/]@tanstack[\\/]/,
          name: 'vendor-tanstack',
          chunks: 'all',
          priority: 0,
          enforce: true,
        },
        // ── Heavy feature-specific libraries: isolate into their own chunks
        // so they are only downloaded by the pages that actually use them.
        'vendor-charts': {
          test: /node_modules[\\/](@visactor[\\/](react-vchart|vchart)|recharts|d3-[^\\/]+)[\\/]/,
          name: 'vendor-charts',
          chunks: 'all',
          priority: 10,
          enforce: true,
        },
        'vendor-shiki': {
          test: /node_modules[\\/](shiki|@shikijs)[\\/]/,
          name: 'vendor-shiki',
          chunks: 'all',
          priority: 10,
          enforce: true,
        },
        'vendor-markdown': {
          test: /node_modules[\\/](react-markdown|streamdown|remark-[^\\/]+|rehype-[^\\/]+|mdast-util-[^\\/]+|micromark[^\\/]*|hast-util-[^\\/]+|unist-util-[^\\/]+|unified|vfile[^\\/]*)[\\/]/,
          name: 'vendor-markdown',
          chunks: 'all',
          priority: 10,
          enforce: true,
        },
        'vendor-icons': {
          // ⚠️ Excludes @lobehub/icons on purpose: it's imported via `import * as` in
          // lib/lobe-icon.tsx and is ~4.6MB. enforce:true + chunks:'all' would force it
          // into the initial bundle, ballooning first-paint cost. Let webpack default
          // splitChunks decide for @lobehub/icons (it ends up async + size-capped).
          test: /node_modules[\\/](@hugeicons[\\/][^\\/]+|react-icons|lucide-react)[\\/]/,
          name: 'vendor-icons',
          chunks: 'all',
          priority: 10,
          enforce: true,
        },
        'vendor-motion': {
          test: /node_modules[\\/](motion|framer-motion)[\\/]/,
          name: 'vendor-motion',
          chunks: 'all',
          priority: 10,
          enforce: true,
        },
        'vendor-flow': {
          test: /node_modules[\\/]@xyflow[\\/]/,
          name: 'vendor-flow',
          chunks: 'all',
          priority: 10,
          enforce: true,
        },
        'vendor-ai': {
          test: /node_modules[\\/](ai|tokenlens|sse\.js|@ai-sdk[\\/][^\\/]+)[\\/]/,
          name: 'vendor-ai',
          chunks: 'all',
          priority: 10,
          enforce: true,
        },
      },
    },
    source: {
      entry: {
        index: './src/main.tsx',
      },
    },
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    html: {
      template: './index.html',
    },
    server: {
      host: '0.0.0.0',
      port: 3001,
      strictPort: false,
      proxy: devProxy,
    },
    output: {
      minify: isProd,
      target: 'web',
      distPath: {
        root: 'dist',
      },
      compression: isProd
        ? {
            algorithm: 'gzip',
            threshold: 1024,
          }
        : false,
      assetPrefix: prodAssetPrefix,
      hash: isProd,
    },
    performance: {
      removeConsole: isProd ? ['log'] : false,
      buildCache: {
        cacheDigest: [process.env.VITE_REACT_APP_VERSION],
      },
    },
    tools: {
      rspack: {
        plugins: [
          tanstackRouter({
            target: 'react',
            // Dev: avoid per-route async chunks (reduces white flash on navigation + faster HMR feedback).
            // Prod: keep route-based code splitting.
            autoCodeSplitting: isProd,
          }),
          // Bundle analyzer: enabled only when RSDOCTOR=true (`bun run analyze`).
          // Generates an interactive report under .rsdoctor/ and opens the browser.
          ...(enableRsdoctor
            ? [
                // eslint-disable-next-line @typescript-eslint/no-require-imports
                new (require('@rsdoctor/rspack-plugin').RsdoctorRspackPlugin)({
                  supports: {
                    generateTileGraph: true,
                  },
                  linter: {
                    level: 'warn',
                  },
                }),
              ]
            : []),
        ],
      },
    },
  }
})
