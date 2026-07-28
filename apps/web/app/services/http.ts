import type { AxiosError, AxiosRequestConfig, InternalAxiosRequestConfig } from 'axios'
import type { ApiErrorCode } from '@/services/errors'
import axios from 'axios'
import {
  ApiError,

  CLIENT_ERROR_CODE,
  createApiError,
} from '@/services/errors'

/** 后端所有 JSON 接口遵循的统一响应外壳。 */
export interface ApiResponse<T> {
  /** 业务状态码，0 表示成功。 */
  code: number
  /** 成功提示或失败原因。 */
  message: string
  /** 接口实际返回的业务数据。 */
  data: T
}

/** 请求层从认证 Store 获取和失效会话所需的回调。 */
interface AdminRequestAuth {
  /** 读取发起请求时最新的访问令牌。 */
  getAccessToken: () => string | null
  /** 收到 401 时失效实际参与该请求的令牌。 */
  onUnauthorized: (accessToken?: string) => void
}

/** Axios 配置附加的后台认证控制字段。 */
type RequestConfig = AxiosRequestConfig & {
  /** 是否要求请求拦截器注入后台访问令牌。 */
  requireAdminAuth?: boolean
  /** 特定请求显式指定的访问令牌。 */
  accessToken?: string
  /** 请求实际使用的令牌，供 401 响应精确失效旧会话。 */
  adminAccessTokenUsed?: string
}

/** 后台请求支持的统一 Axios 配置。 */
export type AdminRequestInit = RequestConfig

/** 由认证 Store 注册的令牌读取和未授权处理能力。 */
let adminRequestAuth: AdminRequestAuth | null = null

/** 统一处理 JSON 协议、认证和错误转换的 Axios 实例。 */
const jsonClient = axios.create({
  headers: {
    Accept: 'application/json',
  },
})

export { ApiError }

/** configureAdminRequest 注册受保护请求读取 Token 与退出登录的统一回调。 */
export function configureAdminRequest(auth: AdminRequestAuth) {
  adminRequestAuth = auth
}

/** isSilentRequestError 判断请求错误是否已由请求层处理或由主动取消产生。 */
export function isSilentRequestError(error: unknown): boolean {
  return axios.isCancel(error) || (error instanceof ApiError && error.handled)
}

jsonClient.interceptors.request.use((config) => {
  const requestConfig = config as InternalAxiosRequestConfig & RequestConfig
  if (!requestConfig.requireAdminAuth) {
    return requestConfig
  }

  const accessToken
    = requestConfig.accessToken ?? adminRequestAuth?.getAccessToken() ?? null
  if (!accessToken) {
    adminRequestAuth?.onUnauthorized()
    throw createApiError({
      code: 20001,
      status: 401,
      message: '登录状态已失效，请重新登录',
      handled: true,
    })
  }

  requestConfig.adminAccessTokenUsed = accessToken
  requestConfig.headers.set('Authorization', `Bearer ${accessToken}`)
  return requestConfig
})

jsonClient.interceptors.response.use(
  (response) => {
    validateSuccessResponse(
      response.data,
      response.status,
      readRequestID(response.headers),
    )
    return response
  },
  (error: AxiosError<unknown>) => {
    if (axios.isCancel(error)) {
      throw error
    }

    const status = error.response?.status ?? 0
    const requestId = readRequestID(error.response?.headers)
    if (!error.response) {
      throw createApiError({
        code: CLIENT_ERROR_CODE.network,
        status,
        requestId,
      })
    }

    const response = error.response.data
    if (!isApiResponse(response) || response.code === 0) {
      throw createApiError({
        code: CLIENT_ERROR_CODE.invalidResponse,
        status,
        requestId,
      })
    }

    const apiError = createApiError({
      code: response.code as ApiErrorCode,
      status,
      message: response.message,
      requestId,
    })
    if (status === 401) {
      const requestConfig = error.config as RequestConfig | undefined
      adminRequestAuth?.onUnauthorized(requestConfig?.adminAccessTokenUsed)
      throw createApiError({
        code: apiError.code,
        status: apiError.status,
        message: apiError.message,
        handled: true,
        requestId: apiError.requestId,
      })
    }

    throw apiError
  },
)

/** requestJSON 发送 JSON 请求，并统一校验响应外壳后返回业务 data。 */
export async function requestJSON<T>(
  path: string,
  config?: RequestConfig,
): Promise<T> {
  const response = await jsonClient.request<ApiResponse<T>>({
    ...config,
    url: path,
  })
  return response.data.data
}

/** requestAdminJSON 统一为受保护后台请求注入 Token，并集中处理登录失效。 */
export function requestAdminJSON<T>(
  path: string,
  config: AdminRequestInit = {},
): Promise<T> {
  return requestJSON<T>(path, { ...config, requireAdminAuth: true })
}

/** validateSuccessResponse 校验成功响应是否符合统一协议。 */
function validateSuccessResponse(
  value: unknown,
  status: number,
  requestId?: string,
): void {
  if (!isApiResponse(value) || value.code !== 0) {
    if (isApiResponse(value) && value.code !== 0) {
      throw createApiError({
        code: value.code,
        status,
        message: value.message,
        requestId,
      })
    }
    throw createApiError({
      code: CLIENT_ERROR_CODE.invalidResponse,
      status,
      requestId,
    })
  }
}

/** readRequestID 从 Axios 响应头中读取服务端请求追踪标识。 */
export function readRequestID(headers: unknown): string | undefined {
  if (!headers || typeof headers !== 'object') {
    return undefined
  }
  const candidate = 'get' in headers && typeof headers.get === 'function'
    ? headers.get('x-request-id')
    : (headers as Record<string, unknown>)['x-request-id']
  return typeof candidate === 'string' && candidate.trim()
    ? candidate.trim()
    : undefined
}

/** isApiResponse 判断未知内容是否满足统一响应外壳的必要字段。 */
function isApiResponse(value: unknown): value is ApiResponse<unknown> {
  return (
    typeof value === 'object'
    && value !== null
    && typeof (value as Partial<ApiResponse<unknown>>).code === 'number'
    && typeof (value as Partial<ApiResponse<unknown>>).message === 'string'
    && 'data' in value
  )
}
