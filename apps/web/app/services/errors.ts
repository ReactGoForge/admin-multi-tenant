/** 后端业务错误码或前端保留的负数错误码。 */
export type ApiErrorCode = number

/** 无法从后端响应获得业务码时使用的客户端错误码。 */
export const CLIENT_ERROR_CODE = {
  network: -1,
  invalidResponse: -2,
} as const

interface CreateApiErrorParams {
  /** 后端业务码或客户端错误码。 */
  code: ApiErrorCode
  /** 对应的 HTTP 状态；网络错误使用请求层约定值。 */
  status: number
  /** 后端返回的权威错误文案。 */
  message?: string
  /** 请求层是否已经展示或处理过该错误。 */
  handled?: boolean
  /** 服务端响应头返回的请求追踪标识。 */
  requestId?: string
}

/** 客户端错误码对应的默认中文文案。 */
const clientErrorMessages: Partial<Record<ApiErrorCode, string>> = {
  [CLIENT_ERROR_CODE.network]: '无法连接服务，请稍后重试',
  [CLIENT_ERROR_CODE.invalidResponse]: '服务响应格式错误',
}

/** ApiError 保留数字业务码、后端权威文案、HTTP 状态以及是否已由请求层处理。 */
export class ApiError extends Error {
  /** 创建包含业务码、HTTP 状态和处理标记的统一请求异常。 */
  constructor(
    message: string,
    public readonly code: ApiErrorCode,
    public readonly status: number,
    public readonly handled = false,
    public readonly requestId?: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

/** createApiError 使用后端文案或客户端异常兜底文案创建统一错误对象。 */
export function createApiError({
  code,
  status,
  message,
  handled = false,
  requestId,
}: CreateApiErrorParams): ApiError {
  return new ApiError(
    message || clientErrorMessages[code] || '服务暂时不可用',
    code,
    status,
    handled,
    requestId,
  )
}

/** getErrorMessage 将统一请求异常转换为页面可展示的文案。 */
export function getErrorMessage(error: unknown, fallback = '操作失败'): string {
  return error instanceof ApiError ? error.message : fallback
}
