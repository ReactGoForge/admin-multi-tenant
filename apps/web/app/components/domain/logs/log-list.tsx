import type { TableColumnsType, TablePaginationConfig } from 'antd'
import type { FormFieldConfig } from '@/components/composite/schema-form'
import type { AuditLog, LogFilterOptions, LogQuery, SystemLog } from '@/services/logs'
import { App, Descriptions, Drawer, Form, Tag, Typography } from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { PageContainer } from '@/components/base/page-container'
import { LogOperatorSelect, TenantSelect } from '@/components/base/selects'
import { SearchTable } from '@/components/composite/search-table'
import { getErrorMessage } from '@/services/errors'
import { isSilentRequestError } from '@/services/http'
import {

  fetchPlatformAuditLogFilterOptions,
  fetchPlatformAuditLogs,
  fetchPlatformSystemLogFilterOptions,
  fetchPlatformSystemLogs,
  fetchTenantAuditLogFilterOptions,
  fetchTenantAuditLogs,

} from '@/services/logs'

/** 日志列表页面配置。 */
interface LogListProps {
  /** 日志类型：系统请求日志或业务操作日志。 */
  kind: 'system' | 'audit'
  /** 日志所属的平台或租户工作空间。 */
  workspace: 'platform' | 'tenant'
}

/** 日志时间范围控件返回的最小日期能力。 */
interface DateValue {
  /** 将日期转换为接口接受的 ISO 时间字符串。 */
  toISOString: () => string
}

/** 日志搜索表单值，在接口查询参数基础上增加时间范围和操作者复合键。 */
type SearchValues = Omit<
  LogQuery,
  'page' | 'pageSize' | 'startAt' | 'endAt' | 'actorType' | 'actorId'
> & {
  /** 用户选择的开始和结束时间，由提交逻辑转换为 startAt 和 endAt。 */
  timeRange?: [DateValue, DateValue]
  /** 操作者类型和 ID 组成的稳定复合键。 */
  operatorKey?: string
}

/** 日志筛选选项的空值，防止异步选项加载前出现未定义数据。 */
const emptyFilterOptions: LogFilterOptions = {
  tenants: [],
  operators: [],
  modules: [],
  actions: [],
}

/** LogListPage 复用平台系统日志、平台操作日志和租户操作日志的查询展示。 */
export function LogListPage({ kind, workspace }: LogListProps) {
  // 页面反馈和日志搜索、列表状态：
  // - logSearchForm/logQuery：控制搜索表单，并保存已经提交的查询条件。
  // - logPage/logPageSize：当前服务端分页位置和每页数量。
  // - logs/logTotal：当前页日志记录及满足条件的总数量。
  // - logListLoading：日志列表请求期间的表格加载状态。
  // - selectedLog：当前在详情抽屉中查看的日志，null 表示抽屉关闭。
  const { message } = App.useApp()
  const [logSearchForm] = Form.useForm<SearchValues>()
  const [logQuery, setLogQuery] = useState<SearchValues>({})
  const [logPage, setLogPage] = useState(1)
  const [logPageSize, setLogPageSize] = useState(10)
  const [logs, setLogs] = useState<Array<SystemLog | AuditLog>>([])
  const [logTotal, setLogTotal] = useState(0)
  const [logListLoading, setLogListLoading] = useState(false)
  const [selectedLog, setSelectedLog] = useState<SystemLog | AuditLog | null>(
    null,
  )

  // 筛选选项状态：接口按日志类型和工作空间返回租户、操作者、模块及动作选项。
  // filterOptionsLoading 只控制选择器加载提示，不影响日志主列表。
  const [filterOptions, setFilterOptions]
    = useState<LogFilterOptions>(emptyFilterOptions)
  const [filterOptionsLoading, setFilterOptionsLoading] = useState(false)
  // 将审计日志模块编码映射为服务端返回的中文模块名称。
  const moduleLabelMap = useMemo(
    () =>
      new Map(
        filterOptions.modules.map(option => [option.value, option.label]),
      ),
    [filterOptions.modules],
  )

  /** handleLogRequestError 统一展示日志请求错误，并忽略主动取消的请求。 */
  const handleLogRequestError = useCallback(
    (error: unknown) => {
      if (!isSilentRequestError(error))
        void message.error(getErrorMessage(error, '日志加载失败'))
    },
    [message],
  )

  // 日志类型或工作空间变化时重新加载对应筛选选项；卸载或切换时取消旧请求。
  useEffect(() => {
    const controller = new AbortController()
    setFilterOptionsLoading(true)
    const request
      = kind === 'system'
        ? fetchPlatformSystemLogFilterOptions(controller.signal)
        : workspace === 'platform'
          ? fetchPlatformAuditLogFilterOptions(controller.signal)
          : fetchTenantAuditLogFilterOptions(controller.signal)
    void request
      .then(options =>
        setFilterOptions({ ...emptyFilterOptions, ...options }),
      )
      .catch(error => handleLogRequestError(error))
      .finally(() => {
        if (!controller.signal.aborted)
          setFilterOptionsLoading(false)
      })
    return () => controller.abort()
  }, [handleLogRequestError, kind, workspace])

  // 查询条件或分页变化时加载日志，并将表单时间范围和操作者复合键转换为接口参数。
  useEffect(() => {
    const controller = new AbortController()
    setLogListLoading(true)
    const { timeRange, operatorKey, ...filters } = logQuery
    const selectedOperator = parseOperatorKey(operatorKey)
    const requestQuery: LogQuery = {
      page: logPage,
      pageSize: logPageSize,
      ...filters,
      actorType: kind === 'system' ? selectedOperator?.actorType : undefined,
      actorId: selectedOperator?.actorId,
      startAt: timeRange?.[0].toISOString(),
      endAt: timeRange?.[1].toISOString(),
    }
    const request
      = kind === 'system'
        ? fetchPlatformSystemLogs(requestQuery, controller.signal)
        : workspace === 'platform'
          ? fetchPlatformAuditLogs(requestQuery, controller.signal)
          : fetchTenantAuditLogs(requestQuery, controller.signal)
    void request
      .then((result) => {
        setLogs(result.items)
        setLogTotal(result.total)
      })
      .catch(handleLogRequestError)
      .finally(() => {
        if (!controller.signal.aborted)
          setLogListLoading(false)
      })
    return () => controller.abort()
  }, [handleLogRequestError, kind, logPage, logPageSize, logQuery, workspace])

  // 系统日志和审计日志使用不同搜索字段，平台审计日志额外支持租户筛选。
  const logSearchFields = useMemo<Array<FormFieldConfig<SearchValues>>>(
    () =>
      kind === 'system'
        ? [
            {
              name: 'timeRange',
              label: '发生时间',
              type: 'rangePicker',
              componentProps: { showTime: true },
            },
            {
              name: 'logType',
              label: '日志类型',
              type: 'select',
              options: [
                { label: '请求', value: 'request' },
                { label: '运行事件', value: 'event' },
              ],
            },
            {
              name: 'level',
              label: '级别',
              type: 'select',
              options: [
                { label: '信息', value: 'info' },
                { label: '警告', value: 'warn' },
                { label: '错误', value: 'error' },
              ],
            },
            {
              name: 'method',
              label: '请求方法',
              type: 'select',
              options: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map(
                value => ({ label: value, value }),
              ),
            },
            {
              name: 'route',
              label: '路由',
              componentProps: { maxLength: 255 },
            },
            {
              name: 'statusCode',
              label: 'HTTP 状态码',
              type: 'number',
              componentProps: { min: 100, max: 599 },
            },
            {
              name: 'businessCode',
              label: '业务错误码',
              type: 'number',
              componentProps: { min: 0 },
            },
            {
              name: 'tenantId',
              label: '所属租户',
              render: () => (
                <TenantSelect
                  loading={filterOptionsLoading}
                  options={filterOptions.tenants}
                />
              ),
            },
            {
              name: 'operatorKey',
              label: '操作者',
              render: () => (
                <LogOperatorSelect
                  loading={filterOptionsLoading}
                  options={filterOptions.operators}
                />
              ),
            },
            {
              name: 'requestId',
              label: '请求 ID',
              componentProps: { maxLength: 64 },
            },
          ]
        : [
            {
              name: 'timeRange',
              label: '操作时间',
              type: 'rangePicker',
              componentProps: { showTime: true },
            },
            ...(workspace === 'platform'
              ? [
                  {
                    name: 'tenantId' as const,
                    label: '所属租户',
                    render: () => (
                      <TenantSelect
                        loading={filterOptionsLoading}
                        options={filterOptions.tenants}
                      />
                    ),
                  },
                ]
              : []),
            {
              name: 'operatorKey',
              label: '操作者',
              render: () => (
                <LogOperatorSelect
                  loading={filterOptionsLoading}
                  options={filterOptions.operators}
                />
              ),
            },
            {
              name: 'module',
              label: '模块',
              type: 'select',
              options: filterOptions.modules,
            },
            {
              name: 'action',
              label: '动作',
              type: 'select',
              options: filterOptions.actions,
            },
            {
              name: 'target',
              label: '目标',
              componentProps: { maxLength: 200 },
            },
          ],
    [filterOptions, filterOptionsLoading, kind, workspace],
  )

  // 根据日志类型生成对应表格列，并统一提供详情入口。
  const logTableColumns = useMemo<TableColumnsType<SystemLog | AuditLog>>(
    () =>
      kind === 'system'
        ? [
            {
              title: '时间',
              dataIndex: 'occurredAt',
              width: 190,
              render: formatTime,
            },
            {
              title: '类型',
              dataIndex: 'logType',
              width: 105,
              render: (value: SystemLog['logType']) => (
                <Tag color={value === 'event' ? 'purple' : 'blue'}>
                  {value === 'event' ? '运行事件' : '请求'}
                </Tag>
              ),
            },
            {
              title: '级别',
              dataIndex: 'level',
              width: 80,
              render: (value: SystemLog['level']) => (
                <Tag
                  color={
                    value === 'error'
                      ? 'error'
                      : value === 'warn'
                        ? 'warning'
                        : 'processing'
                  }
                >
                  {value}
                </Tag>
              ),
            },
            {
              title: '请求/事件内容',
              width: 360,
              ellipsis: true,
              render: (_, row) => {
                const item = row as SystemLog
                if (item.logType === 'event')
                  return nullable(item.message)
                const route = item.route || item.path || '-'
                return `${item.method || '-'} ${route}`
              },
            },
            {
              title: '结果',
              width: 150,
              render: (_, row) => formatSystemResult(row as SystemLog),
            },
            {
              title: '耗时',
              dataIndex: 'durationMs',
              width: 90,
              render: (value: number | null) =>
                value === null ? '-' : `${value} ms`,
            },
            {
              title: '操作者',
              dataIndex: 'actorName',
              width: 180,
              render: (_, row) => formatSystemOperator(row as SystemLog),
            },
            {
              title: '所属租户',
              dataIndex: 'tenantName',
              width: 150,
              render: (_, row) => {
                const item = row as SystemLog
                return nullable(item.tenantName || item.tenantId)
              },
            },
            {
              title: '操作',
              width: 80,
              fixed: 'right',
              render: (_, row) => (
                <Typography.Link onClick={() => setSelectedLog(row)}>
                  详情
                </Typography.Link>
              ),
            },
          ]
        : [
            {
              title: '时间',
              dataIndex: 'occurredAt',
              width: 190,
              render: formatTime,
            },
            {
              title: '来源',
              dataIndex: 'authMode',
              width: 105,
              render: (value: AuditLog['authMode'], row) =>
                value === 'managed'
                  ? (
                      <Tag color="purple">平台代管</Tag>
                    )
                  : (
                      <Tag>
                        {(row as AuditLog).actorScope === 'platform'
                          ? '平台'
                          : '租户'}
                      </Tag>
                    ),
            },
            {
              title: '操作者',
              width: 180,
              render: (_, row) => {
                const item = row as AuditLog
                return `${item.actorName}（${item.actorAccount}）`
              },
            },
            ...(workspace === 'platform'
              ? [
                  {
                    title: '所属租户',
                    dataIndex: 'tenantName',
                    width: 150,
                    render: (_: unknown, row: SystemLog | AuditLog) => {
                      const item = row as AuditLog
                      return nullable(item.tenantName || item.tenantId)
                    },
                  },
                ]
              : []),
            {
              title: '模块',
              dataIndex: 'moduleCode',
              width: 130,
              render: (value: string) => moduleLabelMap.get(value) || value,
            },
            { title: '动作', dataIndex: 'actionName', width: 100 },
            {
              title: '目标',
              width: 170,
              render: (_, row) => {
                const item = row as AuditLog
                return item.targetName || item.targetId || '-'
              },
            },
            { title: '摘要', dataIndex: 'summary', ellipsis: true },
            {
              title: '操作',
              width: 80,
              fixed: 'right',
              render: (_, row) => (
                <Typography.Link onClick={() => setSelectedLog(row)}>
                  详情
                </Typography.Link>
              ),
            },
          ],
    [kind, moduleLabelMap, workspace],
  )

  return (
    <PageContainer>
      <SearchTable<SystemLog | AuditLog, SearchValues>
        search={{
          fields: logSearchFields,
          form: logSearchForm,
          columns: 4,
          onReset: () => {
            setLogPage(1)
            setLogQuery({})
          },
          onSearch: (values) => {
            setLogPage(1)
            setLogQuery(values)
          },
        }}
        columns={logTableColumns}
        dataSource={logs}
        loading={logListLoading}
        rowKey="id"
        scroll={{ x: 1250 }}
        onChange={(pagination: TablePaginationConfig) => {
          const size = pagination.pageSize ?? 10
          setLogPageSize(size)
          setLogPage(size === logPageSize ? (pagination.current ?? 1) : 1)
        }}
        pagination={{
          current: logPage,
          pageSize: logPageSize,
          total: logTotal,
          showSizeChanger: true,
          showTotal: count => `共 ${count} 条`,
        }}
      />
      <LogDetail item={selectedLog} onClose={() => setSelectedLog(null)} />
    </PageContainer>
  )
}

/** LogDetail 使用通用键值描述列表展示所选系统日志或审计日志的完整原始字段。 */
function LogDetail({
  item,
  onClose,
}: {
  /** 当前查看的日志；null 时关闭详情抽屉。 */
  item: SystemLog | AuditLog | null
  /** 用户关闭详情抽屉时触发。 */
  onClose: () => void
}) {
  return (
    <Drawer open={Boolean(item)} onClose={onClose} size={680} title="日志详情">
      {item
        ? (
            <Descriptions
              bordered
              column={1}
              size="small"
              items={Object.entries(item).map(([key, value]) => ({
                key,
                label: key,
                children:
              typeof value === 'object' && value !== null
                ? (
                    <pre className="m-0 whitespace-pre-wrap break-all">
                      {JSON.stringify(value, null, 2)}
                    </pre>
                  )
                : (
                    nullable(value)
                  ),
              }))}
            />
          )
        : null}
    </Drawer>
  )
}

/** formatTime 将接口时间转换为不使用十二小时制的中文本地时间。 */
function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

/** parseOperatorKey 将选择器复合键拆分为接口需要的操作者类型和 ID。 */
function parseOperatorKey(value?: string) {
  if (!value)
    return null
  const [actorType, actorId] = value.split(':')
  if ((actorType !== 'employee' && actorType !== 'miniapp_user') || !actorId) {
    return null
  }
  return { actorType, actorId } as const
}

/** formatSystemOperator 组合系统日志操作者姓名和账号，并处理匿名日志。 */
function formatSystemOperator(item: SystemLog) {
  if (!item.actorName && !item.actorAccount)
    return '-'
  if (item.actorName && item.actorAccount) {
    return `${item.actorName}（${item.actorAccount}）`
  }
  return item.actorName || item.actorAccount || '-'
}

/** formatSystemResult 组合系统请求日志的 HTTP 状态码与业务码。 */
function formatSystemResult(item: SystemLog) {
  if (item.logType === 'event')
    return '-'
  const status
    = item.statusCode === null ? 'HTTP -' : `HTTP ${item.statusCode}`
  const business
    = item.businessCode === null ? '业务码 -' : `业务码 ${item.businessCode}`
  return `${status} / ${business}`
}

/** nullable 将空值统一展示为短横线，其他值转换为字符串。 */
function nullable(value: unknown) {
  return value === null || value === undefined || value === ''
    ? '-'
    : String(value)
}
