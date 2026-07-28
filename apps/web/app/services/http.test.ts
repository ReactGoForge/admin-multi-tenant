import type { AxiosAdapter, AxiosResponse } from 'axios'
import { AxiosError, AxiosHeaders } from 'axios'
import { describe, expect, it, vi } from 'vitest'
import {
  configureAdminRequest,
  readRequestID,
  requestAdminJSON,
} from './http'

describe('统一 Axios 请求层', () => {
  it('从大小写无关的 Axios 响应头读取 Request ID', () => {
    const headers = new AxiosHeaders({ 'X-Request-ID': ' request-1 ' })
    expect(readRequestID(headers)).toBe('request-1')
  })

  it('401 只失效实际使用的 Token，并保留 Request ID', async () => {
    const onUnauthorized = vi.fn()
    configureAdminRequest({
      getAccessToken: () => 'access-token',
      onUnauthorized,
    })
    const adapter: AxiosAdapter = async (config) => {
      const response: AxiosResponse = {
        config,
        data: { code: 20001, message: '登录状态已失效，请重新登录', data: null },
        headers: new AxiosHeaders({ 'X-Request-ID': 'request-401' }),
        status: 401,
        statusText: 'Unauthorized',
      }
      throw new AxiosError(
        'Unauthorized',
        'ERR_BAD_REQUEST',
        config,
        undefined,
        response,
      )
    }

    await expect(
      requestAdminJSON('/api/admin/me', { adapter }),
    ).rejects.toMatchObject({
      code: 20001,
      handled: true,
      requestId: 'request-401',
      status: 401,
    })
    expect(onUnauthorized).toHaveBeenCalledOnce()
    expect(onUnauthorized).toHaveBeenCalledWith('access-token')
  })
})
