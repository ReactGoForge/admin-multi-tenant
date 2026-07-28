import { describe, expect, it } from 'vitest'
import {
  currentSessionSchema,
  parseTenantScene,
  storedAccessTokenSchema,
} from './session'

describe('小程序会话边界校验', () => {
  it('只接受正整数租户 scene', () => {
    expect(parseTenantScene('12')).toBe('12')
    expect(parseTenantScene('0')).toBe('')
    expect(parseTenantScene('tenant-1')).toBe('')
  })

  it('拒绝损坏的 Storage 与当前用户响应', () => {
    expect(storedAccessTokenSchema.safeParse('').success).toBe(false)
    expect(currentSessionSchema.safeParse({
      user: { id: '1', status: 'unknown' },
      tenant: { id: '1', name: '租户' },
    }).success).toBe(false)
  })
})
