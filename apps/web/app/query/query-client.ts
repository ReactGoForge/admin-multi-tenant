import { QueryClient } from '@tanstack/react-query'

/** createAppQueryClient 创建统一的服务端数据缓存实例，测试可使用独立实例避免状态串扰。 */
export function createAppQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        retry: 1,
        refetchOnWindowFocus: false,
      },
      mutations: {
        retry: 0,
      },
    },
  })
}

/** appQueryClient 是浏览器应用生命周期内唯一的 QueryClient。 */
export const appQueryClient = createAppQueryClient()
