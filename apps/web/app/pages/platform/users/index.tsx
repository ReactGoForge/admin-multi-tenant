import type { TableColumnsType, TablePaginationConfig } from 'antd'
import type { FormFieldConfig } from '@/components/composite/schema-form'
import type { PlatformUser, PlatformUserQuery, PlatformUserTenant, PlatformUserTenantOption } from '@/services/users'
import type { EntityStatus } from '@/types/rbac'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { App, Avatar, Button, Form, Modal, Table, Tag } from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { PageContainer } from '@/components/base/page-container'
import { TenantSelect } from '@/components/base/selects'
import { StatusSwitch } from '@/components/base/status-switch'
import { SearchTable } from '@/components/composite/search-table'
import { Permission } from '@/components/domain/auth/permission'
import { DICTIONARY_CODE, useDictionary } from '@/contexts/dictionary'
import {
  platformUserListQueryOptions,
  platformUserQueryKeys,
} from '@/query/platform-users'
import { getErrorMessage } from '@/services/errors'
import { isSilentRequestError } from '@/services/http'
import {
  fetchPlatformUserTenantOptions,
  fetchPlatformUserTenants,

  setPlatformUserStatus,
} from '@/services/users'

/** 平台用户列表已提交的搜索条件，不包含分页参数。 */
type PlatformUserSearch = Omit<PlatformUserQuery, 'page' | 'pageSize'>

/** PlatformUsersPage 展示平台唯一小程序用户并控制全局状态。 */
export default function PlatformUsersPage() {
  const { message } = App.useApp()
  const { getLabel, getOptions } = useDictionary()
  const statusOptions = getOptions(DICTIONARY_CODE.entityStatus)

  // 平台用户查询和列表状态：表单只保存输入，platformUserQuery 保存已提交条件；分页和刷新版本共同触发列表请求。
  const [platformUserSearchForm] = Form.useForm<PlatformUserSearch>()
  const [platformUserQuery, setPlatformUserQuery]
    = useState<PlatformUserSearch>({})
  const [platformUserPage, setPlatformUserPage] = useState(1)
  const [platformUserPageSize, setPlatformUserPageSize] = useState(10)

  // 租户筛选选项独立加载，不阻塞用户列表；加载状态仅控制租户下拉框。
  const [tenantOptions, setTenantOptions] = useState<
    PlatformUserTenantOption[]
  >([])
  const [tenantOptionsLoading, setTenantOptionsLoading] = useState(false)

  // 关联租户弹窗状态：selectedPlatformUser 决定当前查看对象；列表按打开动作加载，关闭时清空避免串数据。
  const [selectedPlatformUser, setSelectedPlatformUser] = useState<PlatformUser | null>(null)
  const [platformUserTenants, setPlatformUserTenants] = useState<PlatformUserTenant[]>([])
  const [platformUserTenantsLoading, setPlatformUserTenantsLoading] = useState(false)

  /** 统一展示平台用户请求错误，并忽略主动取消产生的静默异常。 */
  const handlePlatformUserRequestError = useCallback(
    (error: unknown, fallback = '用户数据加载失败') => {
      if (!isSilentRequestError(error))
        void message.error(getErrorMessage(error, fallback))
    },
    [message],
  )

  const queryClient = useQueryClient()
  const platformUsersQuery = useQuery(
    platformUserListQueryOptions({
      page: platformUserPage,
      pageSize: platformUserPageSize,
      ...platformUserQuery,
    }),
  )
  const platformUserStatusMutation = useMutation({
    mutationFn: ({ id, status }: { id: string, status: EntityStatus }) =>
      setPlatformUserStatus(id, status),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: platformUserQueryKeys.lists(),
      }),
  })

  // Query 已负责旧请求取消、缓存和 Loading；这里只负责将稳定错误转换为页面提示。
  useEffect(() => {
    if (platformUsersQuery.error) {
      handlePlatformUserRequestError(platformUsersQuery.error)
    }
  }, [handlePlatformUserRequestError, platformUsersQuery.error])

  // 页面挂载时加载租户筛选项，卸载时取消请求；失败不会影响用户列表展示。
  useEffect(() => {
    const controller = new AbortController()
    setTenantOptionsLoading(true)
    void fetchPlatformUserTenantOptions(controller.signal)
      .then(setTenantOptions)
      .catch(error =>
        handlePlatformUserRequestError(error, '租户选项加载失败'),
      )
      .finally(() => {
        if (!controller.signal.aborted)
          setTenantOptionsLoading(false)
      })
    return () => controller.abort()
  }, [handlePlatformUserRequestError])

  // 用户变化时按需查询关联租户；关闭弹窗或切换用户会取消旧请求。
  useEffect(() => {
    if (!selectedPlatformUser)
      return
    const controller = new AbortController()
    setPlatformUserTenants([])
    setPlatformUserTenantsLoading(true)
    void fetchPlatformUserTenants(selectedPlatformUser.id, controller.signal)
      .then(setPlatformUserTenants)
      .catch(error =>
        handlePlatformUserRequestError(error, '关联租户加载失败'),
      )
      .finally(() => {
        if (!controller.signal.aborted)
          setPlatformUserTenantsLoading(false)
      })
    return () => controller.abort()
  }, [handlePlatformUserRequestError, selectedPlatformUser])

  // 平台用户支持按昵称、手机号、所属租户和全局状态筛选。
  const platformUserSearchFields = useMemo<
    Array<FormFieldConfig<PlatformUserSearch>>
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
        name: 'tenantId',
        label: '所属租户',
        render: () => (
          <TenantSelect
            loading={tenantOptionsLoading}
            options={tenantOptions}
          />
        ),
      },
      {
        name: 'status',
        label: '用户状态',
        type: 'select',
        options: statusOptions,
      },
    ],
    [statusOptions, tenantOptions, tenantOptionsLoading],
  )

  // 表格展示用户基本资料，并在具备权限时提供全局启停操作。
  const platformUserTableColumns = useMemo<TableColumnsType<PlatformUser>>(
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
        title: '关联租户数',
        dataIndex: 'tenantCount',
        width: 120,
        render: (tenantCount: number, user) => (
          <Button
            type="link"
            onClick={() => setSelectedPlatformUser(user)}
          >
            {tenantCount}
          </Button>
        ),
      },
      {
        title: '全局状态',
        dataIndex: 'status',
        width: 130,
        render: (status: EntityStatus, user) => (
          <Permission
            code="platform:user:status"
            fallback={(
              <Tag color={status === 'enabled' ? 'success' : 'default'}>
                {getLabel(DICTIONARY_CODE.entityStatus, status)}
              </Tag>
            )}
          >
            <StatusSwitch
              onChange={async (value) => {
                try {
                  await platformUserStatusMutation.mutateAsync({
                    id: user.id,
                    status: value,
                  })
                  void message.success('用户状态已更新')
                }
                catch (error) {
                  handlePlatformUserRequestError(error, '操作失败')
                }
              }}
              value={status}
            />
          </Permission>
        ),
      },
      { title: '注册时间', dataIndex: 'createdAt', width: 170 },
    ],
    [
      getLabel,
      handlePlatformUserRequestError,
      message,
      platformUserStatusMutation,
    ],
  )

  return (
    <PageContainer>
      <SearchTable<PlatformUser, PlatformUserSearch>
        search={{
          fields: platformUserSearchFields,
          form: platformUserSearchForm,
          onReset: () => {
            setPlatformUserPage(1)
            setPlatformUserQuery({})
          },
          onSearch: (values) => {
            setPlatformUserPage(1)
            setPlatformUserQuery(values)
          },
        }}
        columns={platformUserTableColumns}
        dataSource={platformUsersQuery.data?.items ?? []}
        loading={platformUsersQuery.isFetching}
        onChange={(pagination: TablePaginationConfig) => {
          const size = pagination.pageSize ?? 10
          setPlatformUserPageSize(size)
          setPlatformUserPage(
            size === platformUserPageSize ? (pagination.current ?? 1) : 1,
          )
        }}
        pagination={{
          current: platformUserPage,
          pageSize: platformUserPageSize,
          showSizeChanger: true,
          showTotal: count => `共 ${count} 条`,
          total: platformUsersQuery.data?.total ?? 0,
        }}
        rowKey="id"
      />
      <Modal
        destroyOnHidden
        footer={null}
        open={selectedPlatformUser !== null}
        title={`关联租户（用户 ID：${selectedPlatformUser?.id ?? '-'}）`}
        width={760}
        onCancel={() => {
          setSelectedPlatformUser(null)
          setPlatformUserTenants([])
        }}
      >
        <Table<PlatformUserTenant>
          columns={[
            { title: '租户名称', dataIndex: 'tenantName' },
            {
              title: '租户状态',
              dataIndex: 'tenantStatus',
              width: 110,
              render: (status: EntityStatus) => (
                <Tag color={status === 'enabled' ? 'success' : 'default'}>
                  {getLabel(DICTIONARY_CODE.entityStatus, status)}
                </Tag>
              ),
            },
            {
              title: '用户状态',
              dataIndex: 'userStatus',
              width: 110,
              render: (status: EntityStatus) => (
                <Tag color={status === 'enabled' ? 'success' : 'default'}>
                  {getLabel(DICTIONARY_CODE.entityStatus, status)}
                </Tag>
              ),
            },
            { title: '加入时间', dataIndex: 'joinedAt', width: 170 },
          ]}
          dataSource={platformUserTenants}
          loading={platformUserTenantsLoading}
          pagination={false}
          rowKey="tenantId"
          size="small"
        />
      </Modal>
    </PageContainer>
  )
}
