import type { ReactNode } from 'react'

import { hasPermission } from '@/auth/permission'
import { useAuthContext } from '@/auth/use-auth-context'

/** 操作级权限展示组件配置。 */
export interface PermissionProps {
  /** 当前操作或内容要求的权限编码。 */
  code: string
  /** 拥有权限时展示的内容。 */
  children: ReactNode
  /** 缺少权限时展示的替代内容，默认不渲染。 */
  fallback?: ReactNode
}

/** Permission 根据当前登录上下文控制操作级内容展示。 */
export function Permission({
  code,
  children,
  fallback = null,
}: PermissionProps) {
  const authContext = useAuthContext()
  return hasPermission(authContext, code) ? children : fallback
}
