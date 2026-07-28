export const apiSuccessCode = 0
export const unauthorizedBusinessCode = 20001

export type RequestMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

/** ApiResponse 描述后端统一响应外壳。 */
export interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

/** RequestOptions 描述统一请求支持的最小参数。 */
export interface RequestOptions<TBody = unknown> {
  url: string
  method?: RequestMethod
  data?: TBody
  header?: Record<string, string>
  accessToken?: string
  timeout?: number
}
