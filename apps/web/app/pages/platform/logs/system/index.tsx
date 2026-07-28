import { LogListPage } from '@/components/domain/logs/log-list'

/** 展示平台范围的请求链路和运行事件日志。 */
export default function PlatformSystemLogsPage() {
  return <LogListPage kind="system" workspace="platform" />
}
