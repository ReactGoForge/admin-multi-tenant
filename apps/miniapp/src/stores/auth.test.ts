import Taro from '@tarojs/taro'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError } from '../api/errors'
import { getCurrentSession, loginMiniapp } from '../api/modules/auth'
import { storageKeys } from '../storage'
import { authStore } from './auth'

const authMocks = vi.hoisted(() => ({
  storage: new Map<string, unknown>(),
}))

vi.mock('@tarojs/taro', () => ({
  default: {
    getStorageSync: vi.fn((key: string) => authMocks.storage.get(key)),
    login: vi.fn(),
    removeStorageSync: vi.fn((key: string) => authMocks.storage.delete(key)),
    setStorageSync: vi.fn((key: string, value: unknown) => authMocks.storage.set(key, value)),
    showToast: vi.fn(),
  },
}))

vi.mock('../api/modules/auth', () => ({
  getCurrentSession: vi.fn(),
  loginMiniapp: vi.fn(),
}))

const testUser = {
  id: '10',
  phone: null,
  nickname: null,
  avatarUrl: null,
  status: 'enabled' as const,
}

/** createCurrentSession 创建 Store 恢复测试使用的会话。 */
function createCurrentSession(tenantId: string) {
  return {
    user: testUser,
    tenant: { id: tenantId, name: `租户${tenantId}` },
  }
}

/** createLoginSession 创建 Store 登录测试使用的完整会话。 */
function createLoginSession(tenantId: string) {
  return {
    accessToken: `token-${tenantId}`,
    expiresAt: '2026-08-03T00:00:00Z',
    ...createCurrentSession(tenantId),
  }
}

describe('小程序认证 Store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authMocks.storage.clear()
    authStore.setState({
      accessToken: null,
      currentTenant: null,
      currentUser: null,
      phase: 'initializing',
      tenantScene: null,
    })
    vi.mocked(Taro.login).mockResolvedValue({
      code: 'wechat-code',
      errMsg: 'login:ok',
    })
    vi.mocked(Taro.showToast).mockResolvedValue({
      errMsg: 'showToast:ok',
    })
    vi.mocked(loginMiniapp).mockImplementation(async body => createLoginSession(body.scene))
  })

  it('首次只有 scene 时等待用户主动登录', async () => {
    await authStore.getState().syncSession('1')

    expect(getCurrentSession).not.toHaveBeenCalled()
    expect(Taro.login).not.toHaveBeenCalled()
    expect(loginMiniapp).not.toHaveBeenCalled()
    expect(authMocks.storage.get(storageKeys.tenantScene)).toBe('1')
    expect(authStore.getState()).toMatchObject({
      accessToken: null,
      currentTenant: null,
      currentUser: null,
      phase: 'idle',
      tenantScene: '1',
    })
  })

  it('有效 Token 刷新时调用 /me 恢复会话', async () => {
    authMocks.storage.set(storageKeys.accessToken, 'stored-token')
    authMocks.storage.set(storageKeys.tenantScene, '1')
    vi.mocked(getCurrentSession).mockResolvedValue(createCurrentSession('1'))

    await authStore.getState().syncSession('')

    expect(getCurrentSession).toHaveBeenCalledWith('stored-token')
    expect(loginMiniapp).not.toHaveBeenCalled()
    expect(authStore.getState()).toMatchObject({
      accessToken: 'stored-token',
      currentTenant: { id: '1', name: '租户1' },
      phase: 'idle',
      tenantScene: '1',
    })
  })

  it('/me 返回 401 时保留 scene 并自动登录', async () => {
    authMocks.storage.set(storageKeys.accessToken, 'expired-token')
    authMocks.storage.set(storageKeys.tenantScene, '1')
    vi.mocked(getCurrentSession).mockRejectedValue(new ApiError('登录状态已失效', {
      httpStatus: 401,
      isUnauthorized: true,
    }))

    await authStore.getState().syncSession('')

    expect(Taro.removeStorageSync).toHaveBeenCalledWith(storageKeys.accessToken)
    expect(authMocks.storage.get(storageKeys.tenantScene)).toBe('1')
    expect(loginMiniapp).toHaveBeenCalledWith({
      code: 'wechat-code',
      scene: '1',
    })
    expect(authStore.getState().accessToken).toBe('token-1')
  })

  it('网络异常时保留原 Token 和 scene', async () => {
    authMocks.storage.set(storageKeys.accessToken, 'stored-token')
    authMocks.storage.set(storageKeys.tenantScene, '1')
    vi.mocked(getCurrentSession).mockRejectedValue(new ApiError('网络异常，请稍后重试'))

    await authStore.getState().syncSession('')

    expect(Taro.removeStorageSync).not.toHaveBeenCalled()
    expect(loginMiniapp).not.toHaveBeenCalled()
    expect(authStore.getState()).toMatchObject({
      accessToken: 'stored-token',
      phase: 'idle',
      tenantScene: '1',
    })
  })

  it('从 A 扫码进入 B 时成功后才覆盖原会话', async () => {
    authMocks.storage.set(storageKeys.accessToken, 'token-1')
    authMocks.storage.set(storageKeys.tenantScene, '1')

    await authStore.getState().syncSession('2')

    expect(getCurrentSession).not.toHaveBeenCalled()
    expect(authStore.getState()).toMatchObject({
      accessToken: 'token-2',
      currentTenant: { id: '2', name: '租户2' },
      phase: 'idle',
      tenantScene: '2',
    })
  })

  it('在 B 登录失败时恢复原 A 会话', async () => {
    authMocks.storage.set(storageKeys.accessToken, 'token-1')
    authMocks.storage.set(storageKeys.tenantScene, '1')
    vi.mocked(loginMiniapp).mockRejectedValueOnce(new Error('租户 B 登录失败'))
    vi.mocked(getCurrentSession).mockResolvedValue(createCurrentSession('1'))

    await authStore.getState().syncSession('2')

    expect(getCurrentSession).toHaveBeenCalledWith('token-1')
    expect(authStore.getState()).toMatchObject({
      accessToken: 'token-1',
      currentTenant: { id: '1', name: '租户1' },
      phase: 'idle',
      tenantScene: '1',
    })
  })

  it('旧 A 响应迟到时不能覆盖已经登录的 B', async () => {
    authMocks.storage.set(storageKeys.accessToken, 'token-1')
    authMocks.storage.set(storageKeys.tenantScene, '1')
    let resolveTenantA: ((session: ReturnType<typeof createCurrentSession>) => void) | undefined
    vi.mocked(getCurrentSession).mockReturnValue(new Promise((resolve) => {
      resolveTenantA = resolve
    }))

    const restoreTenantA = authStore.getState().syncSession('')
    const loginTenantB = authStore.getState().syncSession('2')
    await loginTenantB
    resolveTenantA?.(createCurrentSession('1'))
    await restoreTenantA

    expect(authStore.getState()).toMatchObject({
      accessToken: 'token-2',
      currentTenant: { id: '2', name: '租户2' },
      phase: 'idle',
      tenantScene: '2',
    })
  })
})
