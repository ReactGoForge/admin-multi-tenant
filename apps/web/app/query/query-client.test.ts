import { describe, expect, it } from 'vitest'
import { createAppQueryClient } from './query-client'

describe('createAppQueryClient', () => {
  it('合并相同查询并复用未过期缓存', async () => {
    const queryClient = createAppQueryClient()
    let requestCount = 0
    const options = {
      queryKey: ['dedupe-test'] as const,
      queryFn: async () => {
        requestCount += 1
        await Promise.resolve()
        return 'ok'
      },
    }

    const [first, second] = await Promise.all([
      queryClient.fetchQuery(options),
      queryClient.fetchQuery(options),
    ])
    const cached = await queryClient.fetchQuery(options)

    expect([first, second, cached]).toEqual(['ok', 'ok', 'ok'])
    expect(requestCount).toBe(1)
  })
})
