import { LogListPage } from '@/components/domain/logs/log-list'

/** 展示当前租户范围内的业务操作审计日志。 */
export default function TenantOperationLogsPage() {
  return <LogListPage kind="audit" workspace="tenant" />
}
