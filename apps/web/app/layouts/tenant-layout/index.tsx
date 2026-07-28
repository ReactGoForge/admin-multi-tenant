import { Outlet } from 'react-router'

import { ProtectedRoute } from '@/components/domain/auth/protected-route'
import { WorkspaceGuard } from '@/components/domain/auth/workspace-guard'
import { AppShell } from '@/layouts/app-shell'

/** 租户端路由布局，依次校验登录态和租户工作空间后渲染主框架。 */
export function TenantLayout() {
  return (
    <ProtectedRoute>
      <WorkspaceGuard workspace="tenant">
        <AppShell workspace="tenant">
          <Outlet />
        </AppShell>
      </WorkspaceGuard>
    </ProtectedRoute>
  )
}
