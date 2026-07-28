import { useMemo } from 'react'

import { resolveAuthContext } from '@/auth/permission'
import { useAuthStore } from '@/stores/auth'

/**
 * 订阅当前登录用户并返回精简的权限上下文。
 * 仅在会话身份变化时重新计算，供组件执行权限和工作空间判断。
 */
export function useAuthContext() {
  const currentUser = useAuthStore(state => state.currentUser)

  return useMemo(() => resolveAuthContext(currentUser), [currentUser])
}
