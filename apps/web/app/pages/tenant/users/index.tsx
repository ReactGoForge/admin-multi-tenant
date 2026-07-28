import type { TableColumnsType, TablePaginationConfig } from 'antd'
import type { FormFieldConfig } from '@/components/composite/schema-form'
import type { TenantUser, TenantUserQuery } from '@/services/users'
import type { EntityStatus } from '@/types/rbac'
import { App, Avatar, Form, Tag } from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { PageContainer } from '@/components/base/page-container'
import { StatusSwitch } from '@/components/base/status-switch'
import { SearchTable } from '@/components/composite/search-table'
import { Permission } from '@/components/domain/auth/permission'
import { DICTIONARY_CODE, useDictionary } from '@/contexts/dictionary'
import { getErrorMessage } from '@/services/errors'
import { isSilentRequestError } from '@/services/http'
import {
  fetchTenantUsers,
  setTenantUserStatus,

} from '@/services/users'

/** 租户用户列表已提交的搜索条件，不包含分页参数。 */
type TenantUserSearch = Omit<TenantUserQuery, 'page' | 'pageSize'>

/** TenantUsersPage 展示当前租户的小程序用户并控制租户侧状态。 */
export default function TenantUsersPage() {
  const { message } = App.useApp()
  const { getLabel, getOptions } = useDictionary()
  const statusOptions = getOptions(DICTIONARY_CODE.entityStatus)

  // 租户用户查询和列表状态：表单保存输入，tenantUserQuery 保存已提交条件；分页和刷新版本共同触发列表请求。
  const [tenantUserSearchForm] = Form.useForm<TenantUserSearch>()
  const [tenantUserQuery, setTenantUserQuery] = useState<TenantUserSearch>({})
  const [tenantUserPage, setTenantUserPage] = useState(1)
  const [tenantUserPageSize, setTenantUserPageSize] = useState(10)
  const [tenantUsers, setTenantUsers] = useState<TenantUser[]>([])
  const [tenantUserTotal, setTenantUserTotal] = useState(0)
  const [tenantUserListLoading, setTenantUserListLoading] = useState(false)
  const [tenantUserListRefreshVersion, setTenantUserListRefreshVersion]
    = useState(0)

  /** 统一展示租户用户请求错误，并忽略主动取消产生的静默异常。 */
  const handleTenantUserRequestError = useCallback(
    (error: unknown, fallback = '用户数据加载失败') => {
      if (!isSilentRequestError(error))
        void message.error(getErrorMessage(error, fallback))
    },
    [message],
  )

  // 搜索、分页或写操作刷新版本变化时重新请求租户用户，清理阶段取消旧请求避免过期响应覆盖。
  useEffect(() => {
    void tenantUserListRefreshVersion
    const controller = new AbortController()
    setTenantUserListLoading(true)
    void fetchTenantUsers(
      {
        page: tenantUserPage,
        pageSize: tenantUserPageSize,
        ...tenantUserQuery,
      },
      controller.signal,
    )
      .then((result) => {
        setTenantUsers(result.items)
        setTenantUserTotal(result.total)
      })
      .catch(handleTenantUserRequestError)
      .finally(() => {
        if (!controller.signal.aborted)
          setTenantUserListLoading(false)
      })
    return () => controller.abort()
  }, [
    handleTenantUserRequestError,
    tenantUserListRefreshVersion,
    tenantUserPage,
    tenantUserPageSize,
    tenantUserQuery,
  ])

  // 租户用户支持按昵称、手机号和租户内状态筛选。
  const tenantUserSearchFields = useMemo<
    Array<FormFieldConfig<TenantUserSearch>>
  >(
    () => [
      {
        name: 'nickname',
        label: '昵称',
        componentProps: { allowClear: true, maxLength: 64 },
      },
      {
        name: 'phone',
        label: '手机号',
        componentProps: { allowClear: true, maxLength: 20 },
      },
      {
        name: 'status',
        label: '用户状态',
        type: 'select',
        options: statusOptions,
      },
    ],
    [statusOptions],
  )

  // 表格同时展示平台全局状态和租户内状态；平台禁用时禁止租户侧再次操作。
  const tenantUserTableColumns = useMemo<TableColumnsType<TenantUser>>(
    () => [
      { title: '用户ID', dataIndex: 'id', width: 120 },
      {
        title: '头像',
        dataIndex: 'avatarUrl',
        width: 80,
        render: (value: string | null) =>
          value ? <Avatar alt="用户头像" size="small" src={value} /> : '-',
      },
      {
        title: '昵称',
        dataIndex: 'nickname',
        render: (value: string | null) => value || '-',
      },
      {
        title: '手机号',
        dataIndex: 'phone',
        render: (value: string | null) => value || '-',
      },
      {
        title: '平台状态',
        dataIndex: 'platformStatus',
        width: 100,
        render: (status: EntityStatus) => (
          <Tag color={status === 'enabled' ? 'success' : 'error'}>
            {status === 'enabled' ? '正常' : '平台禁用'}
          </Tag>
        ),
      },
      {
        title: '用户状态',
        dataIndex: 'tenantStatus',
        width: 130,
        render: (status: EntityStatus, user) => {
          if (user.platformStatus === 'disabled') {
            return <Tag color="error">平台已禁用</Tag>
          }
          return (
            <Permission
              code="tenant:user:status"
              fallback={(
                <Tag color={status === 'enabled' ? 'success' : 'default'}>
                  {getLabel(DICTIONARY_CODE.entityStatus, status)}
                </Tag>
              )}
            >
              <StatusSwitch
                onChange={async (value) => {
                  try {
                    await setTenantUserStatus(user.id, value)
                    void message.success('用户状态已更新')
                    setTenantUserListRefreshVersion(version => version + 1)
                  }
                  catch (error) {
                    handleTenantUserRequestError(error, '操作失败')
                  }
                }}
                value={status}
              />
            </Permission>
          )
        },
      },
      { title: '加入时间', dataIndex: 'joinedAt', width: 170 },
    ],
    [getLabel, handleTenantUserRequestError, message],
  )

  return (
    <PageContainer>
      <SearchTable<TenantUser, TenantUserSearch>
        search={{
          fields: tenantUserSearchFields,
          form: tenantUserSearchForm,
          onReset: () => {
            setTenantUserPage(1)
            setTenantUserQuery({})
          },
          onSearch: (values) => {
            setTenantUserPage(1)
            setTenantUserQuery(values)
          },
        }}
        columns={tenantUserTableColumns}
        dataSource={tenantUsers}
        loading={tenantUserListLoading}
        onChange={(pagination: TablePaginationConfig) => {
          const size = pagination.pageSize ?? 10
          setTenantUserPageSize(size)
          setTenantUserPage(
            size === tenantUserPageSize ? (pagination.current ?? 1) : 1,
          )
        }}
        pagination={{
          current: tenantUserPage,
          pageSize: tenantUserPageSize,
          showSizeChanger: true,
          showTotal: count => `共 ${count} 条`,
          total: tenantUserTotal,
        }}
        rowKey="id"
      />
    </PageContainer>
  )
}
