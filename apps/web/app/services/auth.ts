import type { CurrentSessionUser } from '@/types/auth'
import { requestAdminJSON, requestJSON } from '@/services/http'

/** 当前用户接口返回的会话身份。 */
export type CurrentUser = CurrentSessionUser

/** 登录验证码接口的关闭状态或一次性验证码信息。 */
export type CaptchaResult
  = | {
    /** 当前登录不需要验证码。 */
    enabled: false
  }
  | {
    /** 当前登录需要提交验证码。 */
    enabled: true
    /** 本次验证码挑战的唯一标识。 */
    captchaId: string
    /** 可直接展示的验证码图片内容。 */
    image: string
    /** 验证码剩余有效秒数。 */
    expiresIn: number
  }

/** 后台账号登录请求参数。 */
export interface LoginParams {
  /** 登录账号。 */
  username: string
  /** 登录密码明文，仅用于本次 HTTPS 请求。 */
  password: string
  /** 是否将登录态写入持久存储，默认保持登录。 */
  remember?: boolean
  /** 验证码开启时对应的挑战标识。 */
  captchaId?: string
  /** 用户输入的验证码内容。 */
  captchaCode?: string
}

/** 浏览器存储中的访问令牌及可选的平台返回凭据。 */
export interface StoredAuth {
  /** 当前会话访问令牌。 */
  accessToken: string
  /** 当前令牌的服务端过期时间。 */
  expiresAt: string
  /** 代管租户时保留的原平台会话。 */
  platformAuth?: {
    /** 原平台会话访问令牌。 */
    accessToken: string
    /** 原平台令牌的服务端过期时间。 */
    expiresAt: string
  }
}

/** 登录完成后返回给认证 Store 的令牌和当前用户组合。 */
export type LoginResult = StoredAuth & {
  /** 使用新令牌即时获取并校验过的当前用户。 */
  user: CurrentUser
}

/** 登录信息在 localStorage 或 sessionStorage 中共用的键。 */
const AUTH_STORAGE_KEY = 'admin-multi-tenant:auth'
/** 正在进行的验证码请求，用于合并开发环境并发触发的相同请求。 */
let pendingCaptchaRequest: Promise<CaptchaResult> | null = null

/** fetchCaptcha 获取后台登录验证码当前状态及一次性图片。 */
export function fetchCaptcha(): Promise<CaptchaResult> {
  if (!pendingCaptchaRequest) {
    pendingCaptchaRequest = requestJSON<CaptchaResult>(
      '/api/admin/auth/captcha',
      {
        headers: { 'Cache-Control': 'no-store' },
      },
    ).finally(() => {
      pendingCaptchaRequest = null
    })
  }

  return pendingCaptchaRequest
}

/** fetchCurrentUser 携带访问 Token 获取数据库中的最新员工、角色与权限。 */
export function fetchCurrentUser(accessToken: string): Promise<CurrentUser> {
  return requestAdminJSON<CurrentUser>('/api/admin/me', {
    accessToken,
  })
}

/** updateCurrentBasicProfile 修改当前员工本人的可空手机号。 */
export function updateCurrentBasicProfile(phone: string | null): Promise<null> {
  return requestAdminJSON<null>('/api/admin/profile/basic', {
    method: 'PUT',
    data: { phone },
  })
}

/** changeCurrentPassword 校验原密码并修改当前员工密码。 */
export function changeCurrentPassword(
  currentPassword: string,
  newPassword: string,
): Promise<null> {
  return requestAdminJSON<null>('/api/admin/profile/password', {
    method: 'PUT',
    data: { currentPassword, newPassword },
  })
}

/** uploadCurrentAvatar 上传当前员工裁剪后的头像并返回临时访问地址。 */
export function uploadCurrentAvatar(
  file: File,
): Promise<{ avatarUrl: string }> {
  const formData = new FormData()
  formData.append('file', file)
  return requestAdminJSON<{ avatarUrl: string }>('/api/admin/profile/avatar', {
    method: 'POST',
    data: formData,
  })
}

/** login 使用真实凭证换取 Token，写入选定存储后立即校验当前用户。 */
export async function login(params: LoginParams): Promise<LoginResult> {
  const auth = await requestJSON<StoredAuth>('/api/admin/auth/login', {
    method: 'POST',
    data: {
      username: params.username,
      password: params.password,
      captchaId: params.captchaId,
      captchaCode: params.captchaCode,
    },
  })

  writeStoredAuth(auth, params.remember ?? true)
  try {
    const user = await fetchCurrentUser(auth.accessToken)
    return { ...auth, user }
  }
  catch (error) {
    clearStoredAuth(auth.accessToken)
    throw error
  }
}

/** readStoredAuth 从两种浏览器存储读取未过期 Token，不恢复用户或权限快照。 */
export function readStoredAuth(): StoredAuth | null {
  if (typeof window === 'undefined') {
    return null
  }

  for (const storage of [window.localStorage, window.sessionStorage]) {
    const rawValue = storage.getItem(AUTH_STORAGE_KEY)
    if (!rawValue) {
      continue
    }
    try {
      const parsedValue = JSON.parse(rawValue) as Partial<StoredAuth>
      const now = Date.now()
      const expiresAt = Date.parse(parsedValue.expiresAt ?? '')
      if (
        !parsedValue.accessToken
        || !Number.isFinite(expiresAt)
        || expiresAt <= now
      ) {
        storage.removeItem(AUTH_STORAGE_KEY)
        continue
      }

      const platformExpiresAt = Date.parse(
        parsedValue.platformAuth?.expiresAt ?? '',
      )
      const platformAuth
        = parsedValue.platformAuth?.accessToken
          && Number.isFinite(platformExpiresAt)
          && platformExpiresAt > now
          ? {
              accessToken: parsedValue.platformAuth.accessToken,
              expiresAt: parsedValue.platformAuth.expiresAt as string,
            }
          : undefined

      return {
        accessToken: parsedValue.accessToken,
        expiresAt: parsedValue.expiresAt as string,
        platformAuth,
      }
    }
    catch {
      storage.removeItem(AUTH_STORAGE_KEY)
    }
  }
  return null
}

/** writeStoredAuth 根据“保持登录状态”选择持久或会话存储，并先清理旧位置。 */
export function writeStoredAuth(auth: StoredAuth, remember: boolean) {
  if (typeof window === 'undefined') {
    return
  }

  clearStoredAuth()
  const storage = remember ? window.localStorage : window.sessionStorage
  storage.setItem(AUTH_STORAGE_KEY, JSON.stringify(auth))
}

/** replaceStoredAuth 在当前登录使用的存储位置替换 Token。 */
export function replaceStoredAuth(auth: StoredAuth) {
  if (typeof window === 'undefined')
    return
  const remember = window.localStorage.getItem(AUTH_STORAGE_KEY) !== null
  writeStoredAuth(auth, remember)
}

/** clearStoredAuth 清理全部 Token，或仅清理与失效请求 Token 匹配的存储记录。 */
export function clearStoredAuth(accessToken?: string) {
  if (typeof window === 'undefined') {
    return
  }

  for (const storage of [window.localStorage, window.sessionStorage]) {
    if (!accessToken) {
      storage.removeItem(AUTH_STORAGE_KEY)
      continue
    }

    const rawValue = storage.getItem(AUTH_STORAGE_KEY)
    if (!rawValue)
      continue
    try {
      const stored = JSON.parse(rawValue) as Partial<StoredAuth>
      if (
        stored.accessToken === accessToken
        || stored.platformAuth?.accessToken === accessToken
      ) {
        storage.removeItem(AUTH_STORAGE_KEY)
      }
    }
    catch {
      storage.removeItem(AUTH_STORAGE_KEY)
    }
  }
}
