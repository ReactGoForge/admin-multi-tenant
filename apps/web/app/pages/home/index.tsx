import { Navigate } from 'react-router'
import { getFirstAuthorizedPath } from '@/auth/permission'
import { useAuthContext } from '@/auth/use-auth-context'
import { ProtectedRoute } from '@/components/domain/auth/protected-route'
import { useAuthStore } from '@/stores/auth'

/** 根据当前会话菜单跳转到排序最靠前的已授权页面。 */
function HomeRedirect() {
  const authContext = useAuthContext()
  const menus = useAuthStore(state => state.currentUser?.menus ?? [])
  return <Navigate replace to={getFirstAuthorizedPath(authContext, menus)} />
}

/** 渲染空白工作台骨架，后续业务目录从这里开始扩展。 */
export default function HomePage() {
  return (
    <ProtectedRoute>
      <HomeRedirect />
    </ProtectedRoute>
  )
}
