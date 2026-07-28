import type { PlatformUserQuery } from '@/services/users'
import { queryOptions } from '@tanstack/react-query'
import { fetchPlatformUsers } from '@/services/users'

export const platformUserQueryKeys = {
  all: ['platform-users'] as const,
  lists: () => [...platformUserQueryKeys.all, 'list'] as const,
  list: (query: PlatformUserQuery) =>
    [...platformUserQueryKeys.lists(), query] as const,
}

/** platformUserListQueryOptions 创建平台用户分页列表的稳定查询键和请求函数。 */
export function platformUserListQueryOptions(query: PlatformUserQuery) {
  return queryOptions({
    queryKey: platformUserQueryKeys.list(query),
    queryFn: ({ signal }) => fetchPlatformUsers(query, signal),
  })
}
