import type { Plugin } from 'vite'
import { reactRouter } from '@react-router/dev/vite'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig, loadEnv } from 'vite'

const chromeDevToolsProbePath
  = '/.well-known/appspecific/com.chrome.devtools.json'
const defaultApiProxyTarget = 'http://127.0.0.1:8080'

/** 阻止 Chrome DevTools 的工作区探测请求进入 React Router。 */
function ignoreChromeDevToolsProbe(): Plugin {
  return {
    name: 'ignore-chrome-devtools-probe',
    apply: 'serve',
    enforce: 'pre',
    configureServer(server) {
      server.middlewares.use((request, response, next) => {
        if (request.url?.split('?', 1)[0] !== chromeDevToolsProbePath) {
          next()
          return
        }

        response.statusCode = 204
        response.end()
      })
    },
  }
}

export default defineConfig(({ mode }) => {
  const environment = loadEnv(mode, process.cwd(), 'VITE_')
  const apiProxyTarget
    = environment.VITE_API_PROXY_TARGET?.trim() || defaultApiProxyTarget

  return {
    server: {
      host: '0.0.0.0',
      port: 10001,
      proxy: {
        '/api': {
          target: apiProxyTarget,
          changeOrigin: true,
        },
      },
    },
    plugins: [ignoreChromeDevToolsProbe(), tailwindcss(), reactRouter()],
    resolve: {
      tsconfigPaths: true,
    },
  }
})
