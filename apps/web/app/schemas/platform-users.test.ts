import { describe, expect, it } from 'vitest'
import { parsePlatformUserPage } from './platform-users'

describe('parsePlatformUserPage', () => {
  it('拒绝不合法的分页和用户标识', () => {
    expect(() =>
      parsePlatformUserPage({
        items: [{
          id: '0',
          nickname: null,
          avatarUrl: null,
          phone: null,
          status: 'enabled',
          tenantCount: 0,
          createdAt: '2026-01-01 00:00:00',
        }],
        page: 1,
        pageSize: 10,
        total: 1,
      }),
    ).toThrow()
  })
})
