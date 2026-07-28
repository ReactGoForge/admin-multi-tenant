interface ApiErrorOptions {
  httpStatus?: number
  businessCode?: number
  isUnauthorized?: boolean
  requestId?: string
}

/** ApiError 统一描述 HTTP、业务、网络、超时和取消错误。 */
export class ApiError extends Error {
  readonly httpStatus?: number
  readonly businessCode?: number
  readonly isUnauthorized: boolean
  readonly requestId?: string

  constructor(message: string, options: ApiErrorOptions = {}) {
    super(message)
    this.name = 'ApiError'
    this.httpStatus = options.httpStatus
    this.businessCode = options.businessCode
    this.isUnauthorized = options.isUnauthorized ?? false
    this.requestId = options.requestId
  }
}

/** createTransportError 将 Taro 底层失败结果转换为稳定错误类型。 */
export function createTransportError(error: unknown, fallbackMessage = '网络异常，请稍后重试') {
  const rawMessage = error instanceof Error
    ? error.message
    : typeof error === 'object' && error !== null && 'errMsg' in error
      ? String(error.errMsg)
      : ''
  const isTimeout = /timeout/i.test(rawMessage)
  const isCanceled = /abort|cancel/i.test(rawMessage)
  return new ApiError(
    isTimeout ? '请求超时，请稍后重试' : isCanceled ? '请求已取消' : fallbackMessage,
  )
}
