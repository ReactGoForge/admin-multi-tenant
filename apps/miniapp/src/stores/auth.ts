import type { StoreApi } from 'zustand/vanilla'
import type { CurrentSession, MiniappSession, MiniappUser } from '../api/modules/auth'

import Taro from '@tarojs/taro'
import { useCallback, useDebugValue, useSyncExternalStore } from 'react'
import { createStore } from 'zustand/vanilla'

import { ApiError } from '../api/errors'
import { getCurrentSession, loginMiniapp } from '../api/modules/auth'
import {
  clearStoredAccessToken,
  getStoredSession,
  setStoredSession,
  setStoredTenantScene,
} from '../storage'

export type SessionPhase = 'initializing' | 'idle' | 'switching'

interface SessionSnapshot {
  accessToken: string | null
  tenantScene: string | null
  currentUser: MiniappUser | null
  currentTenant: MiniappSession['tenant'] | null
}

export interface AuthState extends SessionSnapshot {
  phase: SessionPhase
  syncSession: (entryTenantScene: string) => Promise<void>
  loginTenant: (tenantScene: string, phoneCode?: string) => Promise<boolean>
  invalidateToken: (expectedAccessToken: string | null) => void
  updateUser: (user: MiniappUser) => void
}

type GetAuthState = StoreApi<AuthState>['getState']
type SetAuthState = StoreApi<AuthState>['setState']

const emptySession: SessionSnapshot = {
  accessToken: null,
  tenantScene: null,
  currentUser: null,
  currentTenant: null,
}

let sessionOperationSeq = 0

/** isUnauthorizedError 判断接口错误是否明确表示 Token 已失效。 */
function isUnauthorizedError(error: unknown) {
  return error instanceof ApiError && error.isUnauthorized
}

/** isForbiddenError 判断用户、租户或租户归属是否被服务端禁止使用。 */
function isForbiddenError(error: unknown) {
  return error instanceof ApiError && error.httpStatus === 403
}

/** showSessionError 展示认证流程中的稳定错误提示。 */
async function showSessionError(error: unknown, fallback: string) {
  const title = error instanceof Error ? error.message : fallback
  await Taro.showToast({ title, icon: 'none' })
}

/** invalidateCurrentToken 只清理仍匹配的旧 Token，保留租户入口。 */
function invalidateCurrentToken(
  expectedAccessToken: string | null,
  getState: GetAuthState,
  setState: SetAuthState,
) {
  const current = getState()
  if (current.accessToken !== expectedAccessToken) {
    return
  }
  clearStoredAccessToken()
  setState({
    accessToken: null,
    currentUser: null,
    currentTenant: null,
  })
}

/** commitCurrentSession 保存 /me 确认后的会话资料并纠正租户入口。 */
function commitCurrentSession(
  accessToken: string,
  session: CurrentSession,
  setState: SetAuthState,
) {
  setStoredSession(accessToken, session.tenant.id)
  setState({
    accessToken,
    tenantScene: session.tenant.id,
    currentUser: session.user,
    currentTenant: session.tenant,
  })
}

/**
 * performTenantLogin 登录目标租户。
 * 只有当前操作仍有效且返回租户一致时才覆盖原会话。
 */
async function performTenantLogin(
  tenantScene: string,
  operationSeq: number,
  setState: SetAuthState,
  phoneCode?: string,
) {
  const loginResult = await Taro.login()
  if (!loginResult.code) {
    throw new Error('未获取到微信登录凭证')
  }
  const result = await loginMiniapp({
    code: loginResult.code,
    scene: tenantScene,
    ...(phoneCode ? { phoneCode } : {}),
  })
  if (result.tenant.id !== tenantScene) {
    throw new Error('登录返回的租户与当前入口不一致')
  }
  if (operationSeq !== sessionOperationSeq) {
    return false
  }
  setStoredSession(result.accessToken, tenantScene)
  setState({
    accessToken: result.accessToken,
    tenantScene,
    currentUser: result.user,
    currentTenant: result.tenant,
  })
  return true
}

/** recoverSnapshot 恢复跨租户切换前的冷启动会话。 */
async function recoverSnapshot(
  snapshot: SessionSnapshot,
  operationSeq: number,
  getState: GetAuthState,
  setState: SetAuthState,
) {
  if (snapshot.currentUser && snapshot.currentTenant) {
    return
  }
  if (!snapshot.accessToken || !snapshot.tenantScene) {
    return
  }
  try {
    const current = await getCurrentSession(snapshot.accessToken)
    if (operationSeq !== sessionOperationSeq) {
      return
    }
    if (current.tenant.id === snapshot.tenantScene) {
      commitCurrentSession(snapshot.accessToken, current, setState)
      return
    }
    await performTenantLogin(snapshot.tenantScene, operationSeq, setState)
  }
  catch (error) {
    if (operationSeq !== sessionOperationSeq) {
      return
    }
    if (isUnauthorizedError(error)) {
      invalidateCurrentToken(snapshot.accessToken, getState, setState)
      try {
        await performTenantLogin(snapshot.tenantScene, operationSeq, setState)
      }
      catch {
        // 原租户登录失败时继续保留 scene，等待用户手动重试。
      }
      return
    }
    if (isForbiddenError(error)) {
      invalidateCurrentToken(snapshot.accessToken, getState, setState)
    }
    // 网络或服务异常保留原 Token，下次进入前台时再次校验。
  }
}

export const authStore = createStore<AuthState>((set, get) => ({
  ...emptySession,
  phase: 'initializing',

  /** syncSession 统一处理刷新恢复、Token 自愈和跨租户切换。 */
  syncSession: async (entryTenantScene) => {
    const operationSeq = ++sessionOperationSeq
    const memory = get()
    const stored = memory.accessToken || memory.tenantScene
      ? {
          accessToken: memory.accessToken,
          tenantScene: memory.tenantScene,
        }
      : getStoredSession()
    const snapshot: SessionSnapshot = {
      accessToken: stored.accessToken,
      tenantScene: stored.tenantScene,
      currentUser: memory.currentUser,
      currentTenant: memory.currentTenant,
    }
    const targetTenantScene = entryTenantScene || stored.tenantScene || ''
    const isSwitchingTenant = Boolean(
      stored.accessToken
      && entryTenantScene
      && stored.tenantScene
      && stored.tenantScene !== entryTenantScene,
    )

    set({
      accessToken: stored.accessToken,
      tenantScene: targetTenantScene || stored.tenantScene,
      phase: isSwitchingTenant ? 'switching' : 'initializing',
    })

    if (!targetTenantScene) {
      set({ phase: 'idle' })
      return
    }
    if (entryTenantScene && (!stored.accessToken || !stored.tenantScene)) {
      setStoredTenantScene(entryTenantScene)
    }

    if (isSwitchingTenant) {
      try {
        await performTenantLogin(entryTenantScene, operationSeq, set)
      }
      catch (error) {
        if (operationSeq !== sessionOperationSeq) {
          return
        }
        set(snapshot)
        await recoverSnapshot(snapshot, operationSeq, get, set)
        await showSessionError(error, '租户切换失败，已保留原租户')
      }
      finally {
        if (operationSeq === sessionOperationSeq) {
          set({ phase: 'idle' })
        }
      }
      return
    }

    try {
      if (!stored.accessToken) {
        return
      }

      const current = await getCurrentSession(stored.accessToken)
      if (operationSeq !== sessionOperationSeq) {
        return
      }
      if (entryTenantScene && current.tenant.id !== entryTenantScene) {
        await performTenantLogin(entryTenantScene, operationSeq, set)
        return
      }
      commitCurrentSession(stored.accessToken, current, set)
    }
    catch (error) {
      if (operationSeq !== sessionOperationSeq) {
        return
      }
      if (isUnauthorizedError(error)) {
        invalidateCurrentToken(stored.accessToken, get, set)
        try {
          await performTenantLogin(targetTenantScene, operationSeq, set)
        }
        catch (loginError) {
          await showSessionError(loginError, '自动登录失败，请稍后重试')
        }
        return
      }
      if (isForbiddenError(error)) {
        invalidateCurrentToken(stored.accessToken, get, set)
      }
      await showSessionError(error, '登录状态恢复失败，请稍后重试')
    }
    finally {
      if (operationSeq === sessionOperationSeq) {
        set({ phase: 'idle' })
      }
    }
  },

  /** loginTenant 执行用户主动发起的微信或手机号登录。 */
  loginTenant: async (tenantScene, phoneCode) => {
    const operationSeq = ++sessionOperationSeq
    set({ phase: 'initializing', tenantScene })
    try {
      return await performTenantLogin(tenantScene, operationSeq, set, phoneCode)
    }
    finally {
      if (operationSeq === sessionOperationSeq) {
        set({ phase: 'idle' })
      }
    }
  },

  /** invalidateToken 只清理调用方确认失效且仍匹配的 Token。 */
  invalidateToken: expectedAccessToken => invalidateCurrentToken(expectedAccessToken, get, set),

  /** updateUser 更新头像等当前用户资料。 */
  updateUser: currentUser => set({ currentUser }),
}))

/** useAuthStore 使用 React 外部 Store 能力订阅 Zustand vanilla Store。 */
export function useAuthStore<T>(selector: (state: AuthState) => T) {
  const selectedState = useSyncExternalStore(
    authStore.subscribe,
    useCallback(() => selector(authStore.getState()), [selector]),
    useCallback(() => selector(authStore.getInitialState()), [selector]),
  )
  useDebugValue(selectedState)
  return selectedState
}
