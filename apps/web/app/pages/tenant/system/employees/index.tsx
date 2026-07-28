import { PlatformEmployeeManagement } from '@/components/domain/rbac/platform-employee-management'

/** 承载当前租户员工管理能力，并固定租户工作空间范围。 */
export default function TenantEmployeesPage() {
  return <PlatformEmployeeManagement workspace="tenant" />
}
