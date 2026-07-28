import { PlatformMenuManagement } from '@/components/domain/rbac/platform-menu-management'

/** 以只读方式展示当前租户可见的菜单和权限节点。 */
export default function TenantMenusPage() {
  return <PlatformMenuManagement workspace="tenant" />
}
