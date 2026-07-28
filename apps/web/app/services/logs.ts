import { requestAdminJSON } from '@/services/http'

/** 日志查询接口的通用分页结果。 */
export interface LogList<T> {
  /** 当前页日志记录。 */
  items: T[]
  /** 当前页码。 */
  page: number
  /** 每页记录数。 */
  pageSize: number
  /** 符合条件的记录总数。 */
  total: number
}

/** 系统日志和操作日志共用的查询条件。 */
export interface LogQuery {
  /** 请求页码。 */
  page: number
  /** 每页记录数。 */
  pageSize: number
  /** 查询开始时间。 */
  startAt?: string
  /** 查询结束时间。 */
  endAt?: string
  /** 系统日志类型：请求日志或运行事件。 */
  logType?: 'request' | 'event'
  /** 日志级别。 */
  level?: string
  /** HTTP 请求方法。 */
  method?: string
  /** 后端登记的接口路由。 */
  route?: string
  /** HTTP 响应状态码。 */
  statusCode?: number
  /** 接口返回的业务状态码。 */
  businessCode?: number
  /** 用于串联一次请求的请求标识。 */
  requestId?: string
  /** 平台查询时限定的租户标识。 */
  tenantId?: string
  /** 操作者选项的复合键。 */
  operator?: string
  /** 操作者来源类型。 */
  actorType?: 'employee' | 'miniapp_user'
  /** 操作者在对应来源中的唯一标识。 */
  actorId?: string
  /** 日志发生的平台端或租户端范围。 */
  workspace?: string
  /** 审计操作所属模块编码。 */
  module?: string
  /** 审计操作动作编码。 */
  action?: string
  /** 审计目标业务类型。 */
  targetType?: string
  /** 审计目标名称或标识关键字。 */
  target?: string
  /** 登录日志中的标准化账号关键字。 */
  account?: string
  /** 登录结果。 */
  result?: 'success' | 'failed' | 'limited'
  /** 登录请求的客户端 IP。 */
  clientIp?: string
}

/** 后台账号的一次登录结果记录。 */
export interface LoginLog {
  /** 登录日志唯一标识。 */
  id: string
  /** 关联请求标识。 */
  requestId: string | null
  /** 登录发生时间。 */
  occurredAt: string
  /** 登录结果：成功、失败或限流。 */
  result: 'success' | 'failed' | 'limited'
  /** 稳定失败原因编码。 */
  reason: string
  /** 已识别员工标识；未知账号为 null。 */
  employeeId: string | null
  /** 已识别员工姓名；未知账号为 null。 */
  employeeName: string | null
  /** 用户提交的标准化登录账号。 */
  account: string | null
  /** 已识别员工原始工作空间。 */
  workspace: 'platform' | 'tenant' | null
  /** 已识别租户员工所属租户标识。 */
  tenantId: string | null
  /** 已识别租户员工所属租户名称。 */
  tenantName: string | null
  /** 客户端 IP。 */
  clientIp: string | null
  /** 已截断的客户端 User-Agent。 */
  userAgent: string | null
}

/** 后端请求链路或运行事件产生的系统日志。 */
export interface SystemLog {
  /** 系统日志唯一标识。 */
  id: string
  /** 记录来自 HTTP 请求还是应用运行事件。 */
  logType: 'request' | 'event'
  /** 日志严重级别。 */
  level: 'info' | 'warn' | 'error'
  /** 请求链路标识；无法关联请求时为 null。 */
  requestId: string | null
  /** 日志实际发生时间。 */
  occurredAt: string
  /** HTTP 请求方法；事件日志为 null。 */
  method: string | null
  /** 后端匹配到的路由模板；事件日志为 null。 */
  route: string | null
  /** 客户端请求的实际路径；事件日志为 null。 */
  path: string | null
  /** HTTP 响应状态码；事件日志为 null。 */
  statusCode: number | null
  /** 业务响应码；无法取得时为 null。 */
  businessCode: number | null
  /** 请求处理耗时毫秒数；事件日志为 null。 */
  durationMs: number | null
  /** 客户端 IP；无法取得时为 null。 */
  clientIp: string | null
  /** 客户端 User-Agent；无法取得时为 null。 */
  userAgent: string | null
  /** 操作者来源类型；匿名请求为 null。 */
  actorType: string | null
  /** 操作者唯一标识；匿名请求为 null。 */
  actorId: string | null
  /** 操作者显示名称；匿名或历史数据缺失时为 null。 */
  actorName: string | null
  /** 操作者登录账号；无法取得时为 null。 */
  actorAccount: string | null
  /** 请求发生的工作空间；无法识别时为 null。 */
  workspace: string | null
  /** 请求所属租户标识；平台请求为 null。 */
  tenantId: string | null
  /** 请求所属租户名称；平台请求或历史名称缺失时为 null。 */
  tenantName: string | null
  /** 正常登录或代管访问模式；无法识别时为 null。 */
  authMode: string | null
  /** 运行事件或失败响应的说明文案。 */
  message: string | null
  /** 后端附加的结构化诊断信息。 */
  metadata: unknown
}

/** 对业务数据产生修改的操作审计日志。 */
export interface AuditLog {
  /** 审计日志唯一标识。 */
  id: string
  /** 关联请求链路的唯一标识。 */
  requestId: string
  /** 业务操作发生时间。 */
  occurredAt: string
  /** 执行操作的员工标识。 */
  actorEmployeeId: string
  /** 操作者显示名称快照。 */
  actorName: string
  /** 操作者登录账号快照。 */
  actorAccount: string
  /** 操作者身份来源于平台员工还是租户员工。 */
  actorScope: 'platform' | 'tenant'
  /** 操作实际影响的平台端或租户端范围。 */
  workspace: 'platform' | 'tenant'
  /** 操作时处于正常登录还是平台代管模式。 */
  authMode: 'normal' | 'managed'
  /** 操作所属租户标识；平台操作为 null。 */
  tenantId: string | null
  /** 操作所属租户名称快照；平台操作为 null。 */
  tenantName: string | null
  /** 业务模块稳定编码。 */
  moduleCode: string
  /** 业务动作稳定编码。 */
  actionCode: string
  /** 业务动作中文名称。 */
  actionName: string
  /** 被操作对象的业务类型。 */
  targetType: string
  /** 被操作对象唯一标识；无具体对象时为 null。 */
  targetId: string | null
  /** 被操作对象名称快照；无具体名称时为 null。 */
  targetName: string | null
  /** 可直接展示的操作摘要。 */
  summary: string
  /** 操作前后字段变化的结构化数据。 */
  changes: unknown
  /** 操作者客户端 IP；无法取得时为 null。 */
  clientIp: string | null
  /** 操作者客户端 User-Agent；无法取得时为 null。 */
  userAgent: string | null
}

/** 日志筛选器可选择的租户快照。 */
export interface LogTenantOption {
  /** 租户唯一标识。 */
  id: string
  /** 租户显示名称。 */
  name: string
  /** 租户当前启停状态。 */
  status: 'enabled' | 'disabled'
}

/** 日志筛选器可选择的历史操作者。 */
export interface LogOperatorOption {
  /** 组合操作者类型和标识的下拉选项键。 */
  key: string
  /** 操作者为后台员工或小程序用户。 */
  actorType: 'employee' | 'miniapp_user'
  /** 操作者在对应来源中的唯一标识。 */
  actorId: string
  /** 操作者显示名称快照。 */
  name: string
  /** 操作者账号快照；小程序用户或缺失数据为 null。 */
  account: string | null
}

/** 模块和动作筛选器使用的编码选项。 */
export interface LogCodeOption {
  /** 请求查询时提交的稳定编码。 */
  value: string
  /** 面向用户展示的名称。 */
  label: string
}

/** 日志页面一次性加载的全部动态筛选选项。 */
export interface LogFilterOptions {
  /** 平台日志中出现过的租户。 */
  tenants: LogTenantOption[]
  /** 当前日志范围中出现过的操作者。 */
  operators: LogOperatorOption[]
  /** 当前日志范围中出现过的业务模块。 */
  modules: LogCodeOption[]
  /** 当前日志范围中出现过的业务动作。 */
  actions: LogCodeOption[]
}

/** 将有实际值的日志查询条件编码为 URL 查询字符串。 */
function queryString(query: LogQuery) {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== null && value !== '') {
      params.set(key, String(value))
    }
  }
  return params.toString()
}

/** fetchPlatformSystemLogs 查询平台系统请求与运行事件日志。 */
export function fetchPlatformSystemLogs(query: LogQuery, signal?: AbortSignal) {
  return requestAdminJSON<LogList<SystemLog>>(
    `/api/admin/platform/logs/system?${queryString(query)}`,
    { signal },
  )
}

/** fetchPlatformSystemLogFilterOptions 查询平台系统日志的租户和历史操作者选项。 */
export function fetchPlatformSystemLogFilterOptions(signal?: AbortSignal) {
  return requestAdminJSON<LogFilterOptions>(
    '/api/admin/platform/logs/system/filter-options',
    { signal },
  )
}

/** fetchPlatformAuditLogs 查询平台范围操作审计日志。 */
export function fetchPlatformAuditLogs(query: LogQuery, signal?: AbortSignal) {
  return requestAdminJSON<LogList<AuditLog>>(
    `/api/admin/platform/logs/operations?${queryString(query)}`,
    { signal },
  )
}

/** fetchPlatformAuditLogFilterOptions 查询平台操作日志的租户和历史操作者选项。 */
export function fetchPlatformAuditLogFilterOptions(signal?: AbortSignal) {
  return requestAdminJSON<LogFilterOptions>(
    '/api/admin/platform/logs/operations/filter-options',
    { signal },
  )
}

/** fetchTenantAuditLogs 查询当前认证租户的操作审计日志。 */
export function fetchTenantAuditLogs(query: LogQuery, signal?: AbortSignal) {
  return requestAdminJSON<LogList<AuditLog>>(
    `/api/admin/tenant/logs/operations?${queryString(query)}`,
    { signal },
  )
}

/** fetchTenantAuditLogFilterOptions 查询当前租户操作日志的历史操作者选项。 */
export function fetchTenantAuditLogFilterOptions(signal?: AbortSignal) {
  return requestAdminJSON<LogFilterOptions>(
    '/api/admin/tenant/logs/operations/filter-options',
    { signal },
  )
}

/** fetchPlatformLoginLogs 查询平台可见的后台登录日志。 */
export function fetchPlatformLoginLogs(query: LogQuery, signal?: AbortSignal) {
  return requestAdminJSON<LogList<LoginLog>>(
    `/api/admin/platform/logs/login?${queryString(query)}`,
    { signal },
  )
}

/** fetchPlatformLoginLogFilterOptions 查询平台登录日志租户选项。 */
export function fetchPlatformLoginLogFilterOptions(signal?: AbortSignal) {
  return requestAdminJSON<LogFilterOptions>(
    '/api/admin/platform/logs/login/filter-options',
    { signal },
  )
}

/** fetchTenantLoginLogs 查询当前认证租户内已识别员工的登录日志。 */
export function fetchTenantLoginLogs(query: LogQuery, signal?: AbortSignal) {
  return requestAdminJSON<LogList<LoginLog>>(
    `/api/admin/tenant/logs/login?${queryString(query)}`,
    { signal },
  )
}

/** fetchTenantLoginLogFilterOptions 查询当前租户登录日志筛选选项。 */
export function fetchTenantLoginLogFilterOptions(signal?: AbortSignal) {
  return requestAdminJSON<LogFilterOptions>(
    '/api/admin/tenant/logs/login/filter-options',
    { signal },
  )
}
