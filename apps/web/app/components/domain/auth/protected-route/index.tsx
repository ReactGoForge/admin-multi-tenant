import type { ReactNode } from 'react'
import { Spin } from 'antd'
import { useEffect } from 'react'
import { useLocation, useNavigate } from 'react-router'

import { useAuthStore } from '@/stores/auth'

/** 登录保护路由配置。 */
interface ProtectedRouteProps {
  /** 需要登录后才能访问的页面内容。 */
  children: ReactNode
}

/** 保护需要登录的页面，未登录时自动跳转到登录页。 */
export function ProtectedRoute({ children }: ProtectedRouteProps) {
  // hydrated 表示浏览器会话恢复已经完成，恢复期间保持全屏加载状态。
  const navigate = useNavigate()
  const location = useLocation()
  const { hydrate, hydrated, isAuthenticated } = useAuthStore()

  // 首次进入受保护区域时恢复本地有效 Token 和最新用户信息。
  useEffect(() => {
    if (!hydrated) {
      void hydrate()
    }
  }, [hydrate, hydrated])

  // 恢复完成仍未登录时跳转登录页，并保存当前路径用于登录后返回。
  useEffect(() => {
    if (hydrated && !isAuthenticated) {
      navigate('/login', {
        replace: true,
        state: { from: location.pathname },
      })
    }
  }, [hydrated, isAuthenticated, location.pathname, navigate])

  if (!hydrated || !isAuthenticated) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-50">
        <Spin size="large" />
      </div>
    )
  }

  return children
}
