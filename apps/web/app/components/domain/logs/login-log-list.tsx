import type { TableColumnsType, TablePaginationConfig } from 'antd'
import type { FormFieldConfig } from '@/components/composite/schema-form'
import type { LoginLog, LogQuery, LogTenantOption } from '@/services/logs'
import { App, Descriptions, Drawer, Form, Tag, Typography } from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { PageContainer } from '@/components/base/page-container'
import { TenantSelect } from '@/components/base/selects'
import { SearchTable } from '@/components/composite/search-table'
import { getErrorMessage } from '@/services/errors'
import { isSilentRequestError } from '@/services/http'
import {
  fetchPlatformLoginLogFilterOptions,
  fetchPlatformLoginLogs,
  fetchTenantLoginLogFilterOptions,
  fetchTenantLoginLogs,

} from '@/services/logs'

/** 登录日志列表所属工作空间。 */
interface LoginLogListProps {
  /** 平台端可查看全局，租户端由服务端强制隔离当前租户。 */
  workspace: 'platform' | 'tenant'
}

/** 登录日志时间范围控件返回的最小日期能力。 */
interface DateValue {
  /** 转换为接口接收的 ISO 时间。 */
  toISOString: () => string
}

/** 登录日志查询表单值。 */
type LoginSearchValues = Pick<
  LogQuery,
  'tenantId' | 'account' | 'result' | 'clientIp'
> & {
  /** 用户选择的开始和结束时间。 */
  timeRange?: [DateValue, DateValue]
}

const resultLabels: Record<LoginLog['result'], string> = {
  success: '成功',
  failed: '失败',
  limited: '已限流',
}

const reasonLabels: Record<string, string> = {
  success: '登录成功',
  captcha_invalid: '验证码错误',
  captcha_unavailable: '验证码服务不可用',
  credentials_invalid: '账号或密码错误',
  account_disabled: '账号已禁用',
  rate_limited: '登录尝试过于频繁',
  security_unavailable: '登录安全服务不可用',
}

/** LoginLogListPage 展示分页登录日志、筛选条件和详情抽屉。 */
export function LoginLogListPage({ workspace }: LoginLogListProps) {
  const { message } = App.useApp()
  // 列表状态：query 保存已提交筛选，page/pageSize 控制服务端分页，selectedLog 控制详情抽屉。
  const [searchForm] = Form.useForm<LoginSearchValues>()
  const [query, setQuery] = useState<LoginSearchValues>({})
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [logs, setLogs] = useState<LoginLog[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [selectedLog, setSelectedLog] = useState<LoginLog | null>(null)
  const [tenantOptions, setTenantOptions] = useState<LogTenantOption[]>([])
  const [optionsLoading, setOptionsLoading] = useState(false)

  /** handleRequestError 忽略主动取消并展示其他登录日志请求错误。 */
  const handleRequestError = useCallback(
    (error: unknown) => {
      if (!isSilentRequestError(error)) {
        void message.error(getErrorMessage(error, '登录日志加载失败'))
      }
    },
    [message],
  )

  useEffect(() => {
    const controller = new AbortController()
    setOptionsLoading(true)
    const request
      = workspace === 'platform'
        ? fetchPlatformLoginLogFilterOptions(controller.signal)
        : fetchTenantLoginLogFilterOptions(controller.signal)
    void request
      .then(options => setTenantOptions(options.tenants ?? []))
      .catch(handleRequestError)
      .finally(() => {
        if (!controller.signal.aborted)
          setOptionsLoading(false)
      })
    return () => controller.abort()
  }, [handleRequestError, workspace])

  useEffect(() => {
    const controller = new AbortController()
    const { timeRange, ...filters } = query
    const requestQuery: LogQuery = {
      page,
      pageSize,
      ...filters,
      startAt: timeRange?.[0].toISOString(),
      endAt: timeRange?.[1].toISOString(),
    }
    setLoading(true)
    const request
      = workspace === 'platform'
        ? fetchPlatformLoginLogs(requestQuery, controller.signal)
        : fetchTenantLoginLogs(requestQuery, controller.signal)
    void request
      .then((result) => {
        setLogs(result.items)
        setTotal(result.total)
      })
      .catch(handleRequestError)
      .finally(() => {
        if (!controller.signal.aborted)
          setLoading(false)
      })
    return () => controller.abort()
  }, [handleRequestError, page, pageSize, query, workspace])

  const searchFields = useMemo<Array<FormFieldConfig<LoginSearchValues>>>(
    () => [
      {
        name: 'timeRange',
        label: '发生时间',
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
                  loading={optionsLoading}
                  options={tenantOptions}
                />
              ),
            },
          ]
        : []),
      {
        name: 'account',
        label: '登录账号',
        componentProps: { maxLength: 100 },
      },
      {
        name: 'result',
        label: '结果',
        type: 'select',
        options: Object.entries(resultLabels).map(([value, label]) => ({
          value,
          label,
        })),
      },
      {
        name: 'clientIp',
        label: '客户端 IP',
        componentProps: { maxLength: 45 },
      },
    ],
    [optionsLoading, tenantOptions, workspace],
  )

  const columns = useMemo<TableColumnsType<LoginLog>>(
    () => [
      {
        title: '时间',
        dataIndex: 'occurredAt',
        width: 190,
        render: formatTime,
      },
      {
        title: '结果',
        dataIndex: 'result',
        width: 100,
        render: (value: LoginLog['result']) => (
          <Tag
            color={
              value === 'success'
                ? 'success'
                : value === 'limited'
                  ? 'warning'
                  : 'error'
            }
          >
            {resultLabels[value]}
          </Tag>
        ),
      },
      {
        title: '原因',
        dataIndex: 'reason',
        width: 190,
        render: (value: string) => reasonLabels[value] ?? value,
      },
      { title: '登录账号', dataIndex: 'account', width: 170, render: nullable },
      {
        title: '员工',
        dataIndex: 'employeeName',
        width: 140,
        render: nullable,
      },
      ...(workspace === 'platform'
        ? [
            {
              title: '所属租户',
              dataIndex: 'tenantName',
              width: 150,
              render: (_: unknown, row: LoginLog) =>
                nullable(row.tenantName || row.tenantId),
            },
          ]
        : []),
      {
        title: '客户端 IP',
        dataIndex: 'clientIp',
        width: 150,
        render: nullable,
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
    ],
    [workspace],
  )

  return (
    <PageContainer>
      <SearchTable<LoginLog, LoginSearchValues>
        columns={columns}
        dataSource={logs}
        loading={loading}
        onChange={(pagination: TablePaginationConfig) => {
          const nextSize = pagination.pageSize ?? 10
          setPageSize(nextSize)
          setPage(nextSize === pageSize ? (pagination.current ?? 1) : 1)
        }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: count => `共 ${count} 条`,
        }}
        rowKey="id"
        scroll={{ x: 1150 }}
        search={{
          fields: searchFields,
          form: searchForm,
          columns: 4,
          onReset: () => {
            setPage(1)
            setQuery({})
          },
          onSearch: (values) => {
            setPage(1)
            setQuery(values)
          },
        }}
      />
      <Drawer
        onClose={() => setSelectedLog(null)}
        open={Boolean(selectedLog)}
        size={680}
        title="登录日志详情"
      >
        {selectedLog
          ? (
              <Descriptions
                bordered
                column={1}
                items={Object.entries(selectedLog).map(([key, value]) => ({
                  key,
                  label: key,
                  children: nullable(value),
                }))}
                size="small"
              />
            )
          : null}
      </Drawer>
    </PageContainer>
  )
}

/** formatTime 将接口时间转换为中文 24 小时制时间。 */
function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

/** nullable 将空值统一展示为短横线。 */
function nullable(value: unknown) {
  return value === null || value === undefined || value === ''
    ? '-'
    : String(value)
}
