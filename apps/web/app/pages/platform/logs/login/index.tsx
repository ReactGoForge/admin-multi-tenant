import { LoginLogListPage } from '@/components/domain/logs/login-log-list'

/** PlatformLoginLogsPage 展示平台权限范围内的后台登录日志。 */
export default function PlatformLoginLogsPage() {
  return <LoginLogListPage workspace="platform" />
}
