import Taro from '@tarojs/taro'

import { parseTenantScene, storedAccessTokenSchema } from '../schemas/session'

export const storageKeys = {
  accessToken: 'admin-multi-tenant:miniapp:access-token',
  tenantScene: 'admin-multi-tenant:miniapp:tenant-scene',
} as const

/** getStoredSession 读取当前 Token 及其所属租户入口。 */
export function getStoredSession() {
  const accessTokenResult = storedAccessTokenSchema.safeParse(
    Taro.getStorageSync(storageKeys.accessToken),
  )
  const accessToken = accessTokenResult.success ? accessTokenResult.data : null
  const tenantScene = parseTenantScene(
    Taro.getStorageSync(storageKeys.tenantScene),
  ) || null
  return { accessToken, tenantScene }
}

/** setStoredSession 持久化登录 Token 及其所属租户入口。 */
export function setStoredSession(accessToken: string, tenantScene: string) {
  Taro.setStorageSync(storageKeys.accessToken, accessToken)
  Taro.setStorageSync(storageKeys.tenantScene, tenantScene)
}

/** setStoredTenantScene 单独保存后续登录或会话恢复使用的租户入口。 */
export function setStoredTenantScene(tenantScene: string) {
  Taro.setStorageSync(storageKeys.tenantScene, tenantScene)
}

/** clearStoredAccessToken 只清理已确认失效的访问 Token，保留租户入口。 */
export function clearStoredAccessToken() {
  Taro.removeStorageSync(storageKeys.accessToken)
}
