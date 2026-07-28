import { LogListPage } from '@/components/domain/logs/log-list'

/** 展示平台范围的业务操作审计日志。 */
export default function PlatformOperationLogsPage() {
  return <LogListPage kind="audit" workspace="platform" />
}
