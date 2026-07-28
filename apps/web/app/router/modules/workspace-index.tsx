import { Navigate } from 'react-router'

import { getFirstAuthorizedPath } from '@/auth/permission'
import { useAuthContext } from '@/auth/use-auth-context'
import { useAuthStore } from '@/stores/auth'

/** 工作空间索引路由，根据当前权限菜单重定向到首个可访问页面。 */
export default function WorkspaceIndex() {
  const authContext = useAuthContext()
  const menus = useAuthStore(state => state.currentUser?.menus ?? [])
  return <Navigate replace to={getFirstAuthorizedPath(authContext, menus)} />
}
