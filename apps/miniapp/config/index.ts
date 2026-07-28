import type { Plugin } from 'vite'

import path from 'node:path'
import { defineConfig } from '@tarojs/cli'
import { createStyleImportPlugin } from 'vite-plugin-style-import'
import { WeappTailwindcss } from 'weapp-tailwindcss/vite'

const projectRoot = path.resolve(__dirname, '..')

export default defineConfig<'vite'>({
  projectName: 'miniapp',
  date: '2026-7-27',
  designWidth: 750,
  deviceRatio: {
    640: 2.34 / 2,
    750: 1,
    375: 2,
    828: 1.81 / 2,
  },
  sourceRoot: 'src',
  outputRoot: 'dist',
  plugins: ['@tarojs/plugin-generator'],
  framework: 'react',
  compiler: {
    type: 'vite',
    vitePlugins: [
      ...(WeappTailwindcss({
        tailwindcssBasedir: projectRoot,
        cssEntries: [path.resolve(projectRoot, 'src/app.css')],
        cssOptions: {
          rem2rpx: true,
          injectAdditionalCssVarScope: true,
        },
      }) ?? []),
      createStyleImportPlugin({
        libs: [{
          libraryName: '@taroify/core',
          esModule: true,
          resolveStyle: name => `@taroify/core/${name}/style/index.js`,
          ensureStyleFile: true,
        }],
      }),
    ] as unknown as Plugin[],
  },
})
