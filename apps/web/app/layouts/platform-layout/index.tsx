import { Outlet } from 'react-router'

import { ProtectedRoute } from '@/components/domain/auth/protected-route'
import { WorkspaceGuard } from '@/components/domain/auth/workspace-guard'
import { AppShell } from '@/layouts/app-shell'

/** 平台端路由布局，依次校验登录态和平台工作空间后渲染主框架。 */
export function PlatformLayout() {
  return (
    <ProtectedRoute>
      <WorkspaceGuard workspace="platform">
        <AppShell workspace="platform">
          <Outlet />
        </AppShell>
      </WorkspaceGuard>
    </ProtectedRoute>
  )
}
