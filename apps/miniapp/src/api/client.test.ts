import Taro from '@tarojs/taro'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from './client'

vi.mock('@tarojs/taro', () => ({
  default: {
    request: vi.fn(),
  },
}))

describe('小程序统一请求层', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('返回包含 Request ID 的结构化 401 错误', async () => {
    vi.mocked(Taro.request).mockResolvedValue({
      cookies: [],
      data: {
        code: 20001,
        message: '登录状态已失效，请重新登录',
        data: null,
      },
      errMsg: 'request:ok',
      header: { 'X-Request-ID': 'request-miniapp-401' },
      statusCode: 401,
    })

    await expect(request({
      url: '/me',
      accessToken: 'access-token',
    })).rejects.toMatchObject({
      businessCode: 20001,
      httpStatus: 401,
      isUnauthorized: true,
      requestId: 'request-miniapp-401',
    })
  })

  it('只为显式 Token 注入 Authorization 请求头', async () => {
    vi.mocked(Taro.request).mockResolvedValue({
      cookies: [],
      data: {
        code: 0,
        message: '成功',
        data: { tenantId: '1' },
      },
      errMsg: 'request:ok',
      header: {},
      statusCode: 200,
    })

    await expect(request({
      url: '/me',
      accessToken: 'access-token',
    })).resolves.toEqual({ tenantId: '1' })
    expect(Taro.request).toHaveBeenCalledWith(expect.objectContaining({
      header: expect.objectContaining({
        Authorization: 'Bearer access-token',
      }),
    }))
  })
})
