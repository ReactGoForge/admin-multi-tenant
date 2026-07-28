import { PlatformDepartmentManagement } from '@/components/domain/rbac/platform-department-management'

/** 承载当前租户组织部门管理能力，并固定租户工作空间范围。 */
export default function TenantDepartmentsPage() {
  return <PlatformDepartmentManagement workspace="tenant" />
}
