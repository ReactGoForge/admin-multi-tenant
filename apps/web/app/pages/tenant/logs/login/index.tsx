import { LoginLogListPage } from '@/components/domain/logs/login-log-list'

/** TenantLoginLogsPage 展示当前租户已识别员工的后台登录日志。 */
export default function TenantLoginLogsPage() {
  return <LoginLogListPage workspace="tenant" />
}
