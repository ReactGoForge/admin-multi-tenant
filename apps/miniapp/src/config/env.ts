type AppEnv = 'development' | 'test' | 'production'

const defaultDevelopmentApiBaseUrl = 'https://test.example.com/api/miniapp'

/** resolveAppEnv 将公开环境变量规范化为小程序内部使用的环境枚举。 */
function resolveAppEnv(): AppEnv {
  // eslint-disable-next-line node/prefer-global/process -- Taro 只会编译期替换直接访问的 process.env.TARO_APP_*。
  const configuredAppEnv = process.env.TARO_APP_ENV
  if (configuredAppEnv === 'development' || configuredAppEnv === 'test' || configuredAppEnv === 'production') {
    return configuredAppEnv
  }
  // eslint-disable-next-line node/prefer-global/process -- Taro 编译期提供 NODE_ENV。
  return process.env.NODE_ENV === 'development' ? 'development' : 'production'
}

/** resolveApiBaseUrl 读取 API 地址，开发环境默认连接测试服务器，其他环境缺失时立即报错。 */
function resolveApiBaseUrl(appEnv: AppEnv) {
  // eslint-disable-next-line node/prefer-global/process -- Taro 只会编译期替换直接访问的 process.env.TARO_APP_*。
  const configuredApiBaseUrl = process.env.TARO_APP_API_BASE_URL?.trim()
  const apiBaseUrl = configuredApiBaseUrl || (appEnv === 'development' ? defaultDevelopmentApiBaseUrl : '')
  if (!apiBaseUrl) {
    throw new Error(`当前 ${appEnv} 环境缺少 TARO_APP_API_BASE_URL`)
  }
  return apiBaseUrl.replace(/\/+$/, '')
}

const appEnv = resolveAppEnv()

export const env = {
  apiBaseUrl: resolveApiBaseUrl(appEnv),
  appEnv,
  // eslint-disable-next-line node/prefer-global/process -- Taro 编译期提供 TARO_ENV。
  taroEnv: process.env.TARO_ENV,
  isDevelopment: appEnv === 'development',
  isTest: appEnv === 'test',
  isProduction: appEnv === 'production',
} as const
