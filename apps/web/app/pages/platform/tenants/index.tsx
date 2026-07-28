import type { TableColumnsType, TablePaginationConfig } from 'antd'
import type { FormFieldConfig } from '@/components/composite/schema-form'
import type { PlatformTenant, TenantCreateInput, TenantUpdateInput } from '@/services/platform/tenants'
import type { EntityStatus } from '@/types/rbac'
import { DeleteOutlined } from '@ant-design/icons'
import { App, Button, Form, Image, Modal, Space, Tag, Tooltip } from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { ConfirmDelete } from '@/components/base/confirm-delete'
import { PageContainer } from '@/components/base/page-container'
import { StatusSwitch } from '@/components/base/status-switch'
import { FormDrawer } from '@/components/composite/form-drawer'
import {

  SchemaForm,
} from '@/components/composite/schema-form'
import { SearchTable } from '@/components/composite/search-table'
import { Permission } from '@/components/domain/auth/permission'
import { DICTIONARY_CODE, useDictionary } from '@/contexts/dictionary'
import { getErrorMessage } from '@/services/errors'
import { isSilentRequestError } from '@/services/http'
import {
  createPlatformTenant,
  deletePlatformTenant,
  fetchPlatformTenantMiniappCode,
  fetchPlatformTenants,
  regeneratePlatformTenantMiniappCode,
  resetPlatformTenantOwnerPassword,
  setPlatformTenantStatus,
  updatePlatformTenant,

} from '@/services/platform/tenants'
import { useAuthStore } from '@/stores/auth'

/** 平台租户列表搜索表单值。 */
interface TenantQuery {
  /** 按租户名称模糊筛选。 */
  name?: string
  /** 按启用或禁用状态筛选。 */
  status?: EntityStatus
}

/** 租户所有者密码重置表单值。 */
interface OwnerPasswordFormValues {
  /** 需要替换原密码的新登录密码。 */
  password: string
}

/** PlatformTenantsPage 管理真实租户及所有者初始化。 */
export default function PlatformTenantsPage() {
  // 页面反馈、字典、导航和进入租户能力。
  const { message } = App.useApp()
  const { getLabel, getOptions } = useDictionary()
  const statusOptions = getOptions(DICTIONARY_CODE.entityStatus)
  const navigate = useNavigate()
  const enterTenant = useAuthStore(state => state.enterTenant)

  // 租户查询和列表状态：
  // - tenantQueryForm/tenantQuery：控制搜索输入并保存已提交条件。
  // - tenantPage/tenantPageSize：服务端分页位置和每页数量。
  // - tenants/tenantTotal：当前页租户和满足条件的总数量。
  // - tenantListLoading/tenantListRefreshVersion：列表请求状态和写操作后的刷新触发值。
  const [tenantQueryForm] = Form.useForm<TenantQuery>()
  const [tenantQuery, setTenantQuery] = useState<TenantQuery>({})
  const [tenantPage, setTenantPage] = useState(1)
  const [tenantPageSize, setTenantPageSize] = useState(10)
  const [tenants, setTenants] = useState<PlatformTenant[]>([])
  const [tenantTotal, setTenantTotal] = useState(0)
  const [tenantListLoading, setTenantListLoading] = useState(false)
  const [tenantListRefreshVersion, setTenantListRefreshVersion] = useState(0)

  // 租户操作表单和抽屉状态：
  // - tenantCreateForm/createDrawerOpen：新增租户及所有者初始化。
  // - tenantEditForm/editingTenant：编辑租户名称、账号和备注。
  // - ownerPasswordForm/ownerPasswordTenant：重置指定租户所有者密码。
  // - tenantMutationLoading：上述写操作共用的提交状态。
  const [tenantCreateForm] = Form.useForm<TenantCreateInput>()
  const [tenantEditForm] = Form.useForm<TenantUpdateInput>()
  const [ownerPasswordForm] = Form.useForm<OwnerPasswordFormValues>()
  const [createDrawerOpen, setCreateDrawerOpen] = useState(false)
  const [editingTenant, setEditingTenant] = useState<PlatformTenant | null>(
    null,
  )
  const [ownerPasswordTenant, setOwnerPasswordTenant]
    = useState<PlatformTenant | null>(null)
  const [tenantMutationLoading, setTenantMutationLoading] = useState(false)

  // 小程序码弹窗状态：目标租户决定扫码场景，图片和加载状态只在弹窗内使用。
  const [miniappCodeTenant, setMiniappCodeTenant]
    = useState<PlatformTenant | null>(null)
  const [miniappCodeImage, setMiniappCodeImage] = useState('')
  const [miniappCodeExtension, setMiniappCodeExtension] = useState<'jpg' | 'png'>('jpg')
  const [miniappCodeLoading, setMiniappCodeLoading] = useState(false)

  /** handleTenantRequestError 统一展示租户请求错误，并忽略主动取消的请求。 */
  const handleTenantRequestError = useCallback(
    (error: unknown, fallback = '租户数据加载失败') => {
      if (!isSilentRequestError(error))
        void message.error(getErrorMessage(error, fallback))
    },
    [message],
  )

  /**
   * loadTenantMiniappCode 读取或强制重新生成目标租户的小程序码。
   * 重新生成失败时保留弹窗中原有图片，成功后替换并提示用户。
   */
  const loadTenantMiniappCode = useCallback(
    async (
      tenant: PlatformTenant,
      regenerate = false,
    ) => {
      setMiniappCodeLoading(true)
      try {
        const result = regenerate
          ? await regeneratePlatformTenantMiniappCode(tenant.id)
          : await fetchPlatformTenantMiniappCode(tenant.id)
        setMiniappCodeImage(result.image)
        setMiniappCodeExtension(result.extension)
        if (regenerate)
          void message.success('小程序码已重新生成')
      }
      catch (error) {
        handleTenantRequestError(
          error,
          regenerate ? '小程序码重新生成失败' : '小程序码获取失败',
        )
      }
      finally {
        setMiniappCodeLoading(false)
      }
    },
    [handleTenantRequestError, message],
  )

  // 搜索、分页或刷新版本变化时重新加载租户列表，并在卸载时取消旧请求。
  useEffect(() => {
    void tenantListRefreshVersion
    const controller = new AbortController()
    setTenantListLoading(true)
    void fetchPlatformTenants(
      { page: tenantPage, pageSize: tenantPageSize, ...tenantQuery },
      controller.signal,
    )
      .then((result) => {
        setTenants(result.items)
        setTenantTotal(result.total)
      })
      .catch(handleTenantRequestError)
      .finally(() => {
        if (!controller.signal.aborted)
          setTenantListLoading(false)
      })
    return () => controller.abort()
  }, [
    handleTenantRequestError,
    tenantListRefreshVersion,
    tenantPage,
    tenantPageSize,
    tenantQuery,
  ])

  // 租户列表支持按名称和状态筛选。
  const tenantSearchFields = useMemo<Array<FormFieldConfig<TenantQuery>>>(
    () => [
      { name: 'name', label: '租户名称' },
      {
        name: 'status',
        label: '状态',
        type: 'select',
        options: statusOptions,
      },
    ],
    [statusOptions],
  )
  // 新增租户同时初始化所有者姓名、全局唯一登录账号和初始密码。
  const tenantCreateFields = useMemo<Array<FormFieldConfig<TenantCreateInput>>>(
    () => [
      {
        name: 'name',
        label: '租户名称',
        rules: [{ required: true, message: '请输入租户名称' }],
        componentProps: { maxLength: 100 },
      },
      {
        name: 'ownerName',
        label: '所有者姓名',
        rules: [{ required: true, message: '请输入所有者姓名' }],
        componentProps: { maxLength: 30 },
      },
      {
        name: 'loginAccount',
        label: '登录账号',
        rules: [{ required: true, message: '请输入登录账号' }],
        componentProps: { maxLength: 40 },
      },
      {
        name: 'password',
        label: '初始密码',
        type: 'password',
        rules: [
          { required: true, min: 6, max: 18, message: '请输入 6 至 18 位密码' },
        ],
        componentProps: { maxLength: 18 },
      },
    ],
    [],
  )
  // 编辑租户只维护名称、所有者登录账号和备注。
  const tenantEditFields = useMemo<Array<FormFieldConfig<TenantUpdateInput>>>(
    () => [
      {
        name: 'name',
        label: '租户名称',
        rules: [{ required: true, message: '请输入租户名称' }],
        componentProps: { maxLength: 100 },
      },
      {
        name: 'loginAccount',
        label: '登录账号',
        rules: [{ required: true, message: '请输入登录账号' }],
        componentProps: { maxLength: 40 },
      },
      {
        name: 'remark',
        label: '备注',
        type: 'textarea',
        componentProps: { maxLength: 500, rows: 4, showCount: true },
      },
    ],
    [],
  )
  // 所有者密码重置表单只提交符合长度规则的新密码。
  const ownerPasswordFields = useMemo<
    Array<FormFieldConfig<OwnerPasswordFormValues>>
  >(
    () => [
      {
        name: 'password',
        label: '新密码',
        type: 'password',
        rules: [
          { required: true, min: 6, max: 18, message: '请输入 6 至 18 位密码' },
        ],
        componentProps: { maxLength: 18 },
      },
    ],
    [],
  )
  // 租户表格列提供状态维护、资料编辑、密码重置、小程序码和进入租户操作。
  const tenantTableColumns = useMemo<TableColumnsType<PlatformTenant>>(
    () => [
      { title: '租户名称', dataIndex: 'name' },
      {
        title: '所有者',
        dataIndex: 'ownerName',
        render: value => value ?? '-',
      },
      {
        title: '登录账号',
        dataIndex: 'loginAccount',
        render: value => value ?? '-',
      },
      {
        title: '状态',
        dataIndex: 'status',
        render: (status: EntityStatus, tenant) => (
          <Permission
            code="platform:tenant:status"
            fallback={(
              <Tag color={status === 'enabled' ? 'success' : 'default'}>
                {getLabel(DICTIONARY_CODE.entityStatus, status)}
              </Tag>
            )}
          >
            <StatusSwitch
              onChange={async (value) => {
                try {
                  await setPlatformTenantStatus(tenant.id, value)
                  void message.success('租户状态已更新')
                  setTenantListRefreshVersion(key => key + 1)
                }
                catch (error) {
                  handleTenantRequestError(error, '操作失败')
                }
              }}
              value={status}
            />
          </Permission>
        ),
      },
      {
        title: '标记',
        key: 'tag',
        render: (_, tenant) => (
          <Tag color={tenant.status === 'enabled' ? 'success' : 'default'}>
            {tenant.status === 'enabled' ? '可登录' : '已停用'}
          </Tag>
        ),
      },
      {
        title: '备注',
        dataIndex: 'remark',
        width: 180,
        ellipsis: { showTitle: true },
        render: value => value ?? '-',
      },
      {
        title: '操作',
        key: 'actions',
        fixed: 'right',
        width: 430,
        render: (_, tenant) => (
          <Space size="small" wrap>
            <Permission code="platform:tenant:edit">
              <Button
                size="small"
                type="link"
                onClick={() => {
                  setEditingTenant(tenant)
                  tenantEditForm.setFieldsValue({
                    name: tenant.name,
                    loginAccount: tenant.loginAccount ?? '',
                    remark: tenant.remark ?? undefined,
                  })
                }}
              >
                编辑
              </Button>
            </Permission>
            <Permission code="platform:tenant:reset-password">
              <Button
                size="small"
                type="link"
                onClick={() => {
                  ownerPasswordForm.resetFields()
                  setOwnerPasswordTenant(tenant)
                }}
              >
                重置密码
              </Button>
            </Permission>
            <Permission code="platform:tenant:miniapp-code">
              <Button
                size="small"
                type="link"
                onClick={() => {
                  setMiniappCodeTenant(tenant)
                  setMiniappCodeImage('')
                  setMiniappCodeExtension('jpg')
                  void loadTenantMiniappCode(tenant)
                }}
              >
                查看小程序码
              </Button>
            </Permission>
            <Permission code="platform:tenant:enter">
              <Button
                disabled={tenant.status !== 'enabled'}
                size="small"
                type="link"
                onClick={async () => {
                  try {
                    await enterTenant(tenant.id)
                    navigate('/tenant', { replace: true })
                  }
                  catch (error) {
                    handleTenantRequestError(error, '进入租户失败')
                  }
                }}
              >
                进入租户
              </Button>
            </Permission>
            <Permission code="platform:tenant:delete">
              <Tooltip
                title={
                  tenant.status === 'enabled'
                    ? '请先停用租户后再删除'
                    : undefined
                }
              >
                <span>
                  <ConfirmDelete
                    description="仅会删除已停用且未产生业务数据的空租户，历史日志会保留。"
                    disabled={tenant.status === 'enabled'}
                    onConfirm={async () => {
                      setTenantMutationLoading(true)
                      try {
                        await deletePlatformTenant(tenant.id)
                        void message.success('租户已删除')
                        setTenantListRefreshVersion(key => key + 1)
                      }
                      catch (error) {
                        handleTenantRequestError(error, '删除失败')
                      }
                      finally {
                        setTenantMutationLoading(false)
                      }
                    }}
                    title="确认删除租户？"
                  >
                    <Button
                      danger
                      disabled={tenant.status === 'enabled'}
                      icon={<DeleteOutlined />}
                      size="small"
                      type="link"
                    >
                      删除
                    </Button>
                  </ConfirmDelete>
                </span>
              </Tooltip>
            </Permission>
          </Space>
        ),
      },
    ],
    [
      tenantEditForm,
      enterTenant,
      getLabel,
      handleTenantRequestError,
      loadTenantMiniappCode,
      message,
      navigate,
      ownerPasswordForm,
    ],
  )
  return (
    <PageContainer>
      <SearchTable<PlatformTenant, TenantQuery>
        actions={(
          <Permission code="platform:tenant:create">
            <Button
              onClick={() => {
                tenantCreateForm.resetFields()
                setCreateDrawerOpen(true)
              }}
              type="primary"
            >
              新增租户
            </Button>
          </Permission>
        )}
        search={{
          fields: tenantSearchFields,
          form: tenantQueryForm,
          onReset: () => {
            setTenantPage(1)
            setTenantQuery({})
          },
          onSearch: (values) => {
            setTenantPage(1)
            setTenantQuery(values)
          },
        }}
        columns={tenantTableColumns}
        dataSource={tenants}
        loading={tenantListLoading}
        onChange={(pagination: TablePaginationConfig) => {
          const size = pagination.pageSize ?? 10
          setTenantPageSize(size)
          setTenantPage(
            size === tenantPageSize ? (pagination.current ?? 1) : 1,
          )
        }}
        pagination={{
          current: tenantPage,
          pageSize: tenantPageSize,
          total: tenantTotal,
          showSizeChanger: true,
          showTotal: count => `共 ${count} 条`,
        }}
        rowKey="id"
        scroll={{ x: 'max-content' }}
      />
      <FormDrawer
        loading={tenantMutationLoading}
        onClose={() => setCreateDrawerOpen(false)}
        onSubmit={() => tenantCreateForm.submit()}
        open={createDrawerOpen}
        title="新增租户"
      >
        <SchemaForm
          columns={1}
          fields={tenantCreateFields}
          form={tenantCreateForm}
          onFinish={async (values) => {
            setTenantMutationLoading(true)
            try {
              await createPlatformTenant(values)
              void message.success('租户已创建')
              setCreateDrawerOpen(false)
              setTenantListRefreshVersion(key => key + 1)
            }
            catch (error) {
              handleTenantRequestError(error, '创建失败')
            }
            finally {
              setTenantMutationLoading(false)
            }
          }}
          showActions={false}
        />
      </FormDrawer>
      <FormDrawer
        loading={tenantMutationLoading}
        onClose={() => setEditingTenant(null)}
        onSubmit={() => tenantEditForm.submit()}
        open={Boolean(editingTenant)}
        title="编辑租户"
      >
        <SchemaForm
          columns={1}
          fields={tenantEditFields}
          form={tenantEditForm}
          onFinish={async (values) => {
            if (!editingTenant)
              return
            setTenantMutationLoading(true)
            try {
              await updatePlatformTenant(editingTenant.id, values)
              void message.success('租户已更新')
              setEditingTenant(null)
              setTenantListRefreshVersion(key => key + 1)
            }
            catch (error) {
              handleTenantRequestError(error, '更新失败')
            }
            finally {
              setTenantMutationLoading(false)
            }
          }}
          showActions={false}
        />
      </FormDrawer>
      <FormDrawer
        loading={tenantMutationLoading}
        onClose={() => setOwnerPasswordTenant(null)}
        onSubmit={() => ownerPasswordForm.submit()}
        open={Boolean(ownerPasswordTenant)}
        title={`重置所有者密码${ownerPasswordTenant ? ` - ${ownerPasswordTenant.name}` : ''}`}
      >
        <SchemaForm
          columns={1}
          fields={ownerPasswordFields}
          form={ownerPasswordForm}
          onFinish={async ({ password }) => {
            if (!ownerPasswordTenant)
              return
            setTenantMutationLoading(true)
            try {
              await resetPlatformTenantOwnerPassword(
                ownerPasswordTenant.id,
                password,
              )
              void message.success('所有者密码已重置')
              setOwnerPasswordTenant(null)
            }
            catch (error) {
              handleTenantRequestError(error, '重置失败')
            }
            finally {
              setTenantMutationLoading(false)
            }
          }}
          showActions={false}
        />
      </FormDrawer>
      <Modal
        footer={
          miniappCodeImage
            ? (
                <Space>
                  <Button
                    disabled={miniappCodeLoading}
                    loading={miniappCodeLoading}
                    onClick={() => {
                      if (miniappCodeTenant)
                        void loadTenantMiniappCode(miniappCodeTenant, true)
                    }}
                  >
                    重新生成
                  </Button>
                  <Button
                    disabled={miniappCodeLoading}
                    type="primary"
                    href={miniappCodeImage}
                    download={`${miniappCodeTenant?.name ?? '租户'}-小程序码.${miniappCodeExtension}`}
                  >
                    下载图片
                  </Button>
                </Space>
              )
            : null
        }
        loading={miniappCodeLoading}
        onCancel={() => setMiniappCodeTenant(null)}
        open={Boolean(miniappCodeTenant)}
        title={`小程序码${miniappCodeTenant ? ` - ${miniappCodeTenant.name}` : ''}`}
      >
        {miniappCodeImage
          ? (
              <div className="flex justify-center py-4">
                <Image
                  alt={`${miniappCodeTenant?.name ?? '租户'}小程序码`}
                  preview={false}
                  src={miniappCodeImage}
                  width={280}
                />
              </div>
            )
          : null}
      </Modal>
    </PageContainer>
  )
}
