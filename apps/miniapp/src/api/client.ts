import type { ApiResponse, RequestOptions } from './types'

import Taro from '@tarojs/taro'

import { env } from '../config/env'
import { ApiError, createTransportError } from './errors'
import { apiSuccessCode, unauthorizedBusinessCode } from './types'

const defaultRequestTimeout = 10_000

/** readRequestID 从 Taro 响应头中读取服务端请求追踪标识。 */
export function readRequestID(headers?: Record<string, unknown>) {
  if (!headers) {
    return undefined
  }
  const entry = Object.entries(headers).find(
    ([name]) => name.toLowerCase() === 'x-request-id',
  )
  return typeof entry?.[1] === 'string' && entry[1].trim()
    ? entry[1].trim()
    : undefined
}

/** readApiResponse 对不可信响应做最小外壳校验，不校验具体业务 data。 */
export function readApiResponse<T>(value: unknown): ApiResponse<T> | null {
  if (typeof value !== 'object' || value === null || !('code' in value) || !('message' in value) || !('data' in value)) {
    return null
  }
  if (typeof value.code !== 'number' || typeof value.message !== 'string') {
    return null
  }
  return value as ApiResponse<T>
}

/** resolveRequestUrl 将相对接口路径拼接到小程序 API Base URL。 */
function resolveRequestUrl(url: string) {
  if (/^https?:\/\//i.test(url)) {
    return url
  }
  return `${env.apiBaseUrl}/${url.replace(/^\/+/, '')}`
}

/** request 统一处理小程序接口地址、Token、响应外壳和错误分类。 */
export async function request<TResponse, TBody = unknown>(
  options: RequestOptions<TBody>,
): Promise<TResponse> {
  let response: Taro.request.SuccessCallbackResult<Record<string, unknown>>
  try {
    response = await Taro.request<Record<string, unknown>>({
      url: resolveRequestUrl(options.url),
      method: options.method ?? 'GET',
      data: options.data,
      timeout: options.timeout ?? defaultRequestTimeout,
      header: {
        'Content-Type': 'application/json',
        ...(options.accessToken ? { Authorization: `Bearer ${options.accessToken}` } : {}),
        ...options.header,
      },
    })
  }
  catch (error) {
    throw createTransportError(error)
  }

  const apiResponse = readApiResponse<TResponse>(response.data)
  const requestId = readRequestID(response.header)
  const isUnauthorized = apiResponse?.code === unauthorizedBusinessCode
    || response.statusCode === 401

  if (response.statusCode < 200 || response.statusCode >= 300) {
    throw new ApiError(apiResponse?.message || '请求失败，请稍后重试', {
      httpStatus: response.statusCode,
      businessCode: apiResponse?.code,
      isUnauthorized,
      requestId,
    })
  }
  if (!apiResponse) {
    throw new ApiError('服务响应格式异常', {
      httpStatus: response.statusCode,
      requestId,
    })
  }
  if (apiResponse.code !== apiSuccessCode) {
    throw new ApiError(apiResponse.message || '请求失败，请稍后重试', {
      httpStatus: response.statusCode,
      businessCode: apiResponse.code,
      isUnauthorized,
      requestId,
    })
  }
  return apiResponse.data
}
