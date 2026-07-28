import type { CurrentUser, LoginParams } from '@/services/auth'

import { create } from 'zustand'
import {
  clearStoredAuth,

  fetchCurrentUser,

  readStoredAuth,
  replaceStoredAuth,
  login as requestLogin,
} from '@/services/auth'
import { configureAdminRequest } from '@/services/http'
import { enterPlatformTenant } from '@/services/platform/tenants'

interface AuthState {
  /** 当前请求使用的访问令牌；未登录时为 null。 */
  accessToken: string | null
  /** 由当前用户接口返回的实时身份和权限；未登录时为 null。 */
  currentUser: CurrentUser | null
  /** 是否已经完成浏览器存储登录态的首次恢复。 */
  hydrated: boolean
  /** 是否正在执行首次登录态恢复，防止并发调用 hydrate。 */
  hydrating: boolean
  /** 是否正在提交登录请求，供登录页控制交互状态。 */
  loading: boolean
  /** Token 与当前用户均有效时为 true。 */
  isAuthenticated: boolean
  /** 从浏览器存储恢复 Token，并通过当前用户接口校验会话。 */
  hydrate: () => Promise<void>
  /** 提交账号密码并建立经过当前用户接口确认的前端会话。 */
  login: (params: LoginParams) => Promise<void>
  /** 使用平台身份进入指定租户，并保留原平台会话用于返回。 */
  enterTenant: (tenantId: string) => Promise<void>
  /** 退出代管租户并恢复进入前保存的平台会话。 */
  leaveTenant: () => Promise<void>
  /** 使用当前 Token 重新获取用户身份、品牌范围和权限。 */
  refreshCurrentUser: () => Promise<void>
  /** 主动退出并清理当前及保留的平台登录信息。 */
  logout: () => void
  /** 处理未授权响应，仅清理由指定旧 Token 建立的会话。 */
  expireSession: (accessToken?: string) => void
}

/** 管理真实后台登录态，并在刷新页面时通过 /me 恢复最新用户与权限。 */
export const useAuthStore = create<AuthState>((set, get) => ({
  accessToken: null,
  currentUser: null,
  hydrated: false,
  hydrating: false,
  loading: false,
  isAuthenticated: false,
  /** 从浏览器存储读取有效 Token，再调用 /me 恢复当前登录身份。 */
  hydrate: async () => {
    if (get().hydrated || get().hydrating) {
      return
    }
    set({ hydrating: true })
    const storedAuth = readStoredAuth()
    if (!storedAuth) {
      set({ hydrated: true, hydrating: false })
      return
    }

    try {
      const currentUser = await fetchCurrentUser(storedAuth.accessToken)
      set({
        accessToken: storedAuth.accessToken,
        currentUser,
        hydrated: true,
        hydrating: false,
        isAuthenticated: true,
      })
    }
    catch {
      clearStoredAuth(storedAuth.accessToken)
      set({
        accessToken: null,
        currentUser: null,
        hydrated: true,
        hydrating: false,
        isAuthenticated: false,
      })
    }
  },
  /** 提交真实登录请求，并只在 Token 与 /me 均成功后建立前端会话。 */
  login: async (params) => {
    set({ loading: true })
    try {
      const auth = await requestLogin(params)
      set({
        accessToken: auth.accessToken,
        currentUser: auth.user,
        isAuthenticated: true,
        hydrated: true,
        loading: false,
      })
    }
    catch (error) {
      set({ loading: false })
      throw error
    }
  },
  /** 进入租户并保留原平台 Token，刷新后仍可返回平台端。 */
  enterTenant: async (tenantId) => {
    const original = readStoredAuth()
    if (!original || original.platformAuth)
      throw new Error('当前平台登录状态无效')
    const managed = await enterPlatformTenant(tenantId)
    const currentUser = await fetchCurrentUser(managed.accessToken)
    replaceStoredAuth({
      ...managed,
      platformAuth: {
        accessToken: original.accessToken,
        expiresAt: original.expiresAt,
      },
    })
    set({
      accessToken: managed.accessToken,
      currentUser,
      isAuthenticated: true,
    })
  },
  /** 返回平台端并使用保留的原 Token 重新获取实时权限。 */
  leaveTenant: async () => {
    const stored = readStoredAuth()
    if (!stored?.platformAuth)
      throw new Error('未找到原平台登录状态')
    const platformAuth = stored.platformAuth
    const currentUser = await fetchCurrentUser(platformAuth.accessToken)
    replaceStoredAuth(platformAuth)
    set({
      accessToken: platformAuth.accessToken,
      currentUser,
      isAuthenticated: true,
    })
  },
  /** 重新读取当前用户、租户名称和最新权限。 */
  refreshCurrentUser: async () => {
    const token = get().accessToken
    if (!token)
      throw new Error('当前登录状态无效')
    const currentUser = await fetchCurrentUser(token)
    set({ currentUser })
  },
  /** 退出登录并清理访问 Token 与当前员工的全部工作空间标签记录。 */
  logout: () => {
    const employeeId = get().currentUser?.employeeId
    if (typeof window !== 'undefined' && employeeId) {
      const workspaceTabsKeyPrefix = `admin-multi-tenant:workspace-tabs:${employeeId}:`
      for (let index = window.sessionStorage.length - 1; index >= 0; index--) {
        const storageKey = window.sessionStorage.key(index)
        if (storageKey?.startsWith(workspaceTabsKeyPrefix)) {
          window.sessionStorage.removeItem(storageKey)
        }
      }
    }
    clearStoredAuth()
    set({
      accessToken: null,
      currentUser: null,
      isAuthenticated: false,
      hydrated: true,
      hydrating: false,
    })
  },
  /** 仅清理触发 401 的旧 Token，避免旧标签页覆盖新标签页登录态。 */
  expireSession: (accessToken) => {
    clearStoredAuth(accessToken)
    set({
      accessToken: null,
      currentUser: null,
      isAuthenticated: false,
      hydrated: true,
      hydrating: false,
    })
  },
}))

// 将认证 Store 接入统一请求层，使请求自动携带最新 Token，并在 401 时失效对应会话。
configureAdminRequest({
  getAccessToken: () => useAuthStore.getState().accessToken,
  onUnauthorized: accessToken =>
    useAuthStore.getState().expireSession(accessToken),
})
