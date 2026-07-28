import { PlatformRoleManagement } from '@/components/domain/rbac/platform-role-management'

/** 承载当前租户角色管理能力，并固定租户工作空间范围。 */
export default function TenantRolesPage() {
  return <PlatformRoleManagement workspace="tenant" />
}
