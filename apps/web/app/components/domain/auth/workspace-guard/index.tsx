import type { ReactNode } from 'react'
import type { WorkspaceType } from '@/types/auth'

import { Navigate, useLocation } from 'react-router'
import { hasPermission } from '@/auth/permission'
import { useAuthContext } from '@/auth/use-auth-context'
import { useAuthStore } from '@/stores/auth'

/** 工作空间、页面菜单和查看权限守卫配置。 */
interface WorkspaceGuardProps {
  /** 路由要求的平台或租户工作空间。 */
  workspace: WorkspaceType
  /** 通过工作空间、菜单和查看权限校验后展示的页面。 */
  children: ReactNode
}

/** WorkspaceGuard 校验工作空间、菜单状态和页面查看权限。 */
export function WorkspaceGuard({ workspace, children }: WorkspaceGuardProps) {
  // 当前路径用于查找数据库菜单节点，认证上下文和菜单共同决定访问结果。
  const location = useLocation()
  const authContext = useAuthContext()
  const menus = useAuthStore(state => state.currentUser?.menus ?? [])

  if (
    !authContext
    || authContext.workspace !== workspace
    || (workspace === 'tenant' && !authContext.tenantId)
  ) {
    return <Navigate replace to="/403" />
  }

  const pageNode = menus.find(
    node =>
      node.scope === workspace
      && node.type === 'menu'
      && node.path === location.pathname,
  )
  if (!pageNode) {
    // 首页和个人信息是不依赖数据库菜单的静态页面，工作空间校验通过后直接放行。
    if (
      location.pathname === `/${workspace}`
      || location.pathname === `/${workspace}/profile`
    ) {
      return children
    }
    return <Navigate replace to="/404" />
  }
  if (
    pageNode.status !== 'enabled'
    || !pageNode.permissionCode
    || !hasPermission(authContext, pageNode.permissionCode)
  ) {
    return <Navigate replace to="/403" />
  }

  return children
}
