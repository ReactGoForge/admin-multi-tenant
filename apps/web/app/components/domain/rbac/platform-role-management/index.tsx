import type { TableColumnsType, TablePaginationConfig } from 'antd'
import type { FormContentConfig, FormFieldConfig } from '@/components/composite/schema-form'
import type { PlatformEmployee } from '@/services/platform/employees'
import type { PlatformRole, PlatformRoleDetail, RoleMutation } from '@/services/platform/roles'
import type { WorkspaceType } from '@/types/auth'
import type { EntityStatus, MenuNode, RoleType } from '@/types/rbac'
import { App, Button, Form, Space, Tabs, Tag } from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { ConfirmDelete } from '@/components/base/confirm-delete'
import { PageContainer } from '@/components/base/page-container'
import { StatusSwitch } from '@/components/base/status-switch'
import { FormDrawer } from '@/components/composite/form-drawer'
import {

  SchemaForm,
} from '@/components/composite/schema-form'
import { SearchTable } from '@/components/composite/search-table'
import { Permission } from '@/components/domain/auth/permission'
import { PermissionTree } from '@/components/domain/rbac/permission-tree'
import { PlatformEmployeeTable } from '@/components/domain/rbac/platform-employee-table'
import { DICTIONARY_CODE, useDictionary } from '@/contexts/dictionary'
import { getErrorMessage } from '@/services/errors'
import { isSilentRequestError } from '@/services/http'
import {
  assignWorkspaceRolePermissions,
  createWorkspaceRole,
  deleteWorkspaceRole,
  fetchPlatformRolePermissionOptions,
  fetchWorkspaceRoleDetail,
  fetchWorkspaceRoleEmployees,
  fetchWorkspaceRoles,
  setWorkspaceRoleStatus,
  updateWorkspaceRole,

} from '@/services/platform/roles'
import { useAuthStore } from '@/stores/auth'

/** 角色列表搜索表单值。 */
interface RoleSearchValues {
  /** 按角色名称模糊筛选。 */
  name?: string
  /** 按系统或自定义角色类型筛选。 */
  type?: RoleType
  /** 按启用或禁用状态筛选。 */
  status?: EntityStatus
}

/** 角色新增、编辑和权限表单值，沿用接口写入结构。 */
type RoleFormValues = RoleMutation

/** 角色管理组件配置。 */
interface PlatformRoleManagementProps {
  /** 管理的平台或租户工作空间，默认 platform。 */
  workspace?: WorkspaceType
}

/**
 * filterTenantMenuPermissions 按角色配置场景移除不可分配的租户菜单管理节点。
 * 只过滤 permissionCode 以 tenant:menu: 开头的节点，不修改原数组或其他租户权限。
 * @param menus 权限树使用的扁平租户菜单节点。
 * @param restricted 当前角色是否禁止分配租户菜单管理权限。
 * @returns 可用于当前角色权限树的新节点数组；无需限制时返回原数组。
 */
function filterTenantMenuPermissions(menus: MenuNode[], restricted: boolean) {
  if (!restricted)
    return menus
  return menus.filter(
    menu => !menu.permissionCode?.startsWith('tenant:menu:'),
  )
}

/** PlatformRoleManagement 管理平台或当前租户的真实角色与权限。 */
export function PlatformRoleManagement({
  workspace = 'platform',
}: PlatformRoleManagementProps) {
  // 页面基础能力：
  // - message：通过 Ant Design App 上下文展示成功或错误提示。
  // - getLabel：根据字典编码和值取得展示文案。
  // - getOptions：把指定字典转换为表单 Select 可直接使用的选项数组。
  // - roleTypeOptions/statusOptions：分别提供角色类型和实体状态选项。
  const { message } = App.useApp()
  const { getLabel, getOptions } = useDictionary()
  const roleTypeOptions = getOptions(DICTIONARY_CODE.roleType)
  const statusOptions = getOptions(DICTIONARY_CODE.entityStatus)

  // 角色搜索与列表状态：
  // - roleQueryForm：控制搜索区域的输入、校验和重置。
  // - roleQuery：保存已经提交的搜索条件；仅修改输入框不会立即请求列表。
  // - rolePage/rolePageSize：当前服务端分页位置和每页数量。
  // - roles/roleTotal：接口返回的当前页角色数据和满足条件的总数量。
  // - roleListLoading：角色列表请求期间的表格加载状态。
  // - roleListRefreshVersion：写操作成功后递增，用于触发列表重新加载。
  const [roleQueryForm] = Form.useForm<RoleSearchValues>()
  const [roleQuery, setRoleQuery] = useState<RoleSearchValues>({})
  const [rolePage, setRolePage] = useState(1)
  const [rolePageSize, setRolePageSize] = useState(10)
  const [roles, setRoles] = useState<PlatformRole[]>([])
  const [roleTotal, setRoleTotal] = useState(0)
  const [roleListLoading, setRoleListLoading] = useState(false)
  const [roleListRefreshVersion, setRoleListRefreshVersion] = useState(0)

  // 当前登录上下文：
  // - currentWorkspaceMenus：/me 返回的当前工作空间可见菜单，用作租户权限树或平台权限树的安全兜底数据。
  // - isSuperAdmin：标识当前账号是否为平台超级管理员，用于判断能否配置受支持的内置角色。
  const currentWorkspaceMenus = useAuthStore(
    state => state.currentUser?.menus ?? [],
  ) as MenuNode[]
  const isSuperAdmin = useAuthStore(
    state => state.currentUser?.isSuperAdmin ?? false,
  )

  // 角色表单抽屉状态：
  // - roleForm：新增、编辑、查看权限和配置权限共用的表单实例。
  // - roleMutationLoading：新增、编辑、删除、启停或保存权限期间的统一提交状态。
  // - roleDrawerMode：决定抽屉当前执行新增、编辑、权限配置、权限查看或关闭。
  // - activeRole：当前正在编辑或查看权限的角色；新增角色时为 null。
  // - roleDetail：角色权限详情及可分配菜单树；切换目标角色时先清空，接口成功后回填。
  const [roleForm] = Form.useForm<RoleFormValues>()
  const [roleMutationLoading, setRoleMutationLoading] = useState(false)
  const [roleDrawerMode, setRoleDrawerMode] = useState<
    'create' | 'edit' | 'permission' | 'view' | null
  >(null)
  const [activeRole, setActiveRole] = useState<PlatformRole | null>(null)
  const [roleDetail, setRoleDetail] = useState<PlatformRoleDetail | null>(null)

  // “角色员工”只读抽屉状态：
  // - employeeDrawerRole：当前正在查看关联员工的角色；为 null 时抽屉关闭。
  // - roleEmployees/roleEmployeeTotal：当前页关联员工和该角色的员工总数。
  // - roleEmployeePage/roleEmployeePageSize：关联员工列表自己的分页状态，不影响角色主列表分页。
  const [employeeDrawerRole, setEmployeeDrawerRole]
    = useState<PlatformRole | null>(null)
  const [roleEmployees, setRoleEmployees] = useState<PlatformEmployee[]>([])
  const [roleEmployeeTotal, setRoleEmployeeTotal] = useState(0)
  const [roleEmployeePage, setRoleEmployeePage] = useState(1)
  const [roleEmployeePageSize, setRoleEmployeePageSize] = useState(10)

  /**
   * handleRoleRequestError 统一处理角色管理请求错误。
   * 请求被 AbortController 主动取消时不提示；其他错误优先展示服务端文案，缺少明确文案时使用 fallback。
   * @param error 请求抛出的未知错误。
   * @param fallback 无法解析错误文案时展示的中文兜底提示。
   */
  const handleRoleRequestError = useCallback(
    (error: unknown, fallback = '角色数据加载失败') => {
      if (!isSilentRequestError(error))
        void message.error(getErrorMessage(error, fallback))
    },
    [message],
  )

  // 加载角色主列表：搜索条件、页码、每页数量或刷新版本变化时重新请求。
  // 每次 effect 创建独立 AbortController，依赖变化或组件卸载时取消旧请求，避免旧响应覆盖新列表。
  useEffect(() => {
    void roleListRefreshVersion
    const controller = new AbortController()
    setRoleListLoading(true)
    void fetchWorkspaceRoles(
      workspace,
      { page: rolePage, pageSize: rolePageSize, ...roleQuery },
      controller.signal,
    )
      .then((result) => {
        setRoles(result.items)
        setRoleTotal(result.total)
      })
      .catch(handleRoleRequestError)
      .finally(() => {
        if (!controller.signal.aborted)
          setRoleListLoading(false)
      })
    return () => controller.abort()
  }, [
    handleRoleRequestError,
    roleListRefreshVersion,
    rolePage,
    rolePageSize,
    roleQuery,
    workspace,
  ])

  // 加载所选角色的关联员工：打开抽屉或员工分页变化时重新请求。
  // 关闭抽屉后立即清空旧员工数据；请求取消不显示错误，也不会继续更新列表。
  useEffect(() => {
    if (!employeeDrawerRole) {
      setRoleEmployees([])
      return
    }
    const controller = new AbortController()
    void fetchWorkspaceRoleEmployees(
      workspace,
      employeeDrawerRole.id,
      roleEmployeePage,
      roleEmployeePageSize,
      controller.signal,
    )
      .then((result) => {
        setRoleEmployees(result.items)
        setRoleEmployeeTotal(result.total)
      })
      .catch(handleRoleRequestError)
    return () => controller.abort()
  }, [
    employeeDrawerRole,
    handleRoleRequestError,
    roleEmployeePage,
    roleEmployeePageSize,
    workspace,
  ])

  /**
   * runRoleMutation 执行角色新增、编辑、删除、启停或权限保存操作。
   * 执行期间开启统一提交状态；成功后提示操作结果并递增列表刷新版本，失败时统一提示，结束后恢复提交状态。
   * @param operation 实际调用后端接口的异步操作。
   * @param success 操作成功后展示的中文提示。
   * @returns 操作成功返回 true，失败返回 false，供提交方法判断是否关闭抽屉。
   */
  const runRoleMutation = async (
    operation: () => Promise<unknown>,
    success: string,
  ) => {
    setRoleMutationLoading(true)
    try {
      await operation()
      void message.success(success)
      setRoleListRefreshVersion(value => value + 1)
      return true
    }
    catch (error) {
      handleRoleRequestError(error, '操作失败')
      return false
    }
    finally {
      setRoleMutationLoading(false)
    }
  }

  /**
   * openRoleEditDrawer 打开角色基础信息编辑抽屉。
   * 保存目标角色、清空上一次权限详情，重置表单后回填角色名称和描述，最后切换到编辑模式。
   * @param role 用户在表格中选择编辑的角色。
   */
  const openRoleEditDrawer = (role: PlatformRole) => {
    setActiveRole(role)
    setRoleDetail(null)
    roleForm.resetFields()
    roleForm.setFieldsValue({
      name: role.name,
      description: role.description ?? undefined,
    })
    setRoleDrawerMode('edit')
  }

  /**
   * openRolePermissionDrawer 打开角色权限查看或配置抽屉。
   * 先保存目标角色和抽屉模式，再清空旧详情及表单；详情接口成功后回填各工作空间的权限 ID 和菜单树。
   * @param role 用户在表格中选择的角色。
   * @param mode permission 表示可编辑权限，view 表示只读查看系统角色权限。
   */
  const openRolePermissionDrawer = async (
    role: PlatformRole,
    mode: 'permission' | 'view',
  ) => {
    setActiveRole(role)
    setRoleDrawerMode(mode)
    setRoleDetail(null)
    roleForm.resetFields()
    try {
      const result = await fetchWorkspaceRoleDetail(workspace, role.id)
      setRoleDetail(result)
      roleForm.setFieldsValue({
        permissionIds: result.permissionIds ?? [],
        platformPermissionIds: result.platformPermissionIds ?? [],
        tenantPermissionIds: result.tenantPermissionIds ?? [],
      })
    }
    catch (error) {
      handleRoleRequestError(error)
    }
  }

  /**
   * openRoleCreateDrawer 打开新增角色抽屉并准备默认值。
   * 新角色默认启用且权限为空；平台工作空间额外加载平台权限树和租户代管权限树，租户工作空间直接使用当前可见菜单。
   */
  const openRoleCreateDrawer = () => {
    setActiveRole(null)
    setRoleDetail(null)
    roleForm.resetFields()
    roleForm.setFieldsValue({
      status: 'enabled',
      permissionIds: [],
      platformPermissionIds: [],
      tenantPermissionIds: [],
    })
    setRoleDrawerMode('create')
    if (workspace === 'platform') {
      void fetchPlatformRolePermissionOptions()
        .then(options =>
          setRoleDetail({
            role: {
              id: '',
              name: '',
              description: null,
              type: 'custom',
              systemKey: null,
              status: 'enabled',
              employeeCount: 0,
              permissionCount: 0,
              permissionConfigurable: true,
              createdAt: '',
            },
            platformMenus: options.platformMenus,
            tenantMenus: options.tenantMenus,
          }),
        )
        .catch(handleRoleRequestError)
    }
  }

  /**
   * handleRoleFormSubmit 根据抽屉模式提交角色表单。
   * 权限模式只提交权限 ID，编辑模式只提交名称和描述，新增模式提交完整角色表单；仅在接口成功时关闭抽屉。
   * @param values SchemaForm 校验通过后的角色表单值。
   */
  const handleRoleFormSubmit = async (values: RoleFormValues) => {
    let ok = false
    if (roleDrawerMode === 'permission' && activeRole) {
      ok = await runRoleMutation(
        () =>
          assignWorkspaceRolePermissions(workspace, activeRole.id, {
            permissionIds: values.permissionIds,
            platformPermissionIds: values.platformPermissionIds,
            tenantPermissionIds: values.tenantPermissionIds,
          }),
        '角色权限已更新',
      )
    }
    else if (roleDrawerMode === 'edit' && activeRole) {
      ok = await runRoleMutation(
        () =>
          updateWorkspaceRole(workspace, activeRole.id, {
            name: values.name,
            description: values.description,
          }),
        '角色已更新',
      )
    }
    else if (roleDrawerMode === 'create') {
      ok = await runRoleMutation(
        () => createWorkspaceRole(workspace, values),
        '角色已新增',
      )
    }
    if (ok)
      setRoleDrawerMode(null)
  }

  // 角色搜索字段配置：支持按名称、角色类型和状态筛选。
  const roleSearchFields = useMemo<Array<FormFieldConfig<RoleSearchValues>>>(
    () => [
      { name: 'name', label: '角色名称' },
      {
        name: 'type',
        label: '角色类型',
        type: 'select',
        options: roleTypeOptions,
      },
      {
        name: 'status',
        label: '状态',
        type: 'select',
        options: statusOptions,
      },
    ],
    [roleTypeOptions, statusOptions],
  )
  // 新增和编辑角色都会使用的名称、描述基础字段。
  const roleBaseFields: Array<FormContentConfig<RoleFormValues>> = [
    {
      name: 'name',
      label: '角色名称',
      rules: [{ required: true, message: '请输入角色名称' }],
      componentProps: { maxLength: 30 },
    },
    {
      name: 'description',
      label: '角色描述',
      type: 'textarea',
      componentProps: { maxLength: 200 },
    },
  ]
  // 角色状态仅在新增时填写，已有角色通过列表中的状态开关维护。
  const roleStatusField: FormContentConfig<RoleFormValues> = {
    name: 'status',
    label: '状态',
    type: 'select',
    rules: [{ required: true, message: '请选择状态' }],
    options: statusOptions,
  }
  // 租户菜单管理只允许平台超级管理员配置企业管理员时选择；只读模式始终展示真实权限。
  const restrictTenantMenuPermissions
    = roleDrawerMode !== 'view'
      && !(
        workspace === 'tenant'
        && roleDrawerMode === 'permission'
        && isSuperAdmin
        && activeRole?.systemKey === 'tenant_owner'
      )
  const tenantRolePermissionMenus = filterTenantMenuPermissions(
    roleDetail?.menus ?? currentWorkspaceMenus,
    restrictTenantMenuPermissions,
  )
  const managedTenantPermissionMenus = filterTenantMenuPermissions(
    roleDetail?.tenantMenus ?? [],
    restrictTenantMenuPermissions,
  )
  // 权限字段按工作空间切换：平台角色分别配置平台权限和租户代管权限，租户角色只配置当前租户权限。
  const rolePermissionField: FormContentConfig<RoleFormValues>
    = workspace === 'platform'
      ? {
          key: 'platform-permission-tabs',
          colSpan: 24,
          renderItem: () => (
            <Tabs
              items={[
                {
                  key: 'platform',
                  label: '平台权限',
                  children: (
                    <Form.Item
                      name="platformPermissionIds"
                      valuePropName="value"
                    >
                      <PermissionTree
                        menus={
                          roleDetail?.platformMenus ?? currentWorkspaceMenus
                        }
                        readonly={roleDrawerMode === 'view'}
                        workspace="platform"
                      />
                    </Form.Item>
                  ),
                },
                {
                  key: 'tenant',
                  label: '租户代管权限',
                  children: (
                    <Form.Item name="tenantPermissionIds" valuePropName="value">
                      <PermissionTree
                        menus={managedTenantPermissionMenus}
                        readonly={roleDrawerMode === 'view'}
                        workspace="tenant"
                      />
                    </Form.Item>
                  ),
                },
              ]}
            />
          ),
        }
      : {
          name: 'permissionIds',
          label: '权限树',
          colSpan: 24,
          render: () => (
            <PermissionTree
              menus={tenantRolePermissionMenus}
              readonly={roleDrawerMode === 'view'}
              workspace={workspace}
            />
          ),
        }
  // 根据抽屉模式组合最终字段：新增显示完整字段，编辑只显示基础字段，权限模式只显示权限树。
  const roleFields: Array<FormContentConfig<RoleFormValues>>
    = roleDrawerMode === 'create'
      ? [...roleBaseFields, roleStatusField, rolePermissionField]
      : roleDrawerMode === 'edit'
        ? roleBaseFields
        : roleDrawerMode === 'permission' || roleDrawerMode === 'view'
          ? [rolePermissionField]
          : []
  // 角色表格列包含基础信息、字典文案、状态维护及受权限控制的行操作。
  const roleTableColumns: TableColumnsType<PlatformRole> = [
    { title: '角色名称', dataIndex: 'name', width: 160 },
    {
      title: '角色类型',
      dataIndex: 'type',
      width: 110,
      render: (type: RoleType) => (
        <Tag color={type === 'system' ? 'gold' : 'blue'}>
          {getLabel(DICTIONARY_CODE.roleType, type)}
        </Tag>
      ),
    },
    { title: '员工数量', dataIndex: 'employeeCount', width: 100 },
    { title: '权限数量', dataIndex: 'permissionCount', width: 100 },
    {
      title: '状态',
      dataIndex: 'status',
      width: 110,
      render: (status: EntityStatus, role) => (
        <Permission
          code={`${workspace}:role:status`}
          fallback={(
            <Tag color={status === 'enabled' ? 'success' : 'default'}>
              {getLabel(DICTIONARY_CODE.entityStatus, status)}
            </Tag>
          )}
        >
          <StatusSwitch
            disabled={role.type === 'system' || !role.permissionConfigurable}
            onChange={(value) => {
              void runRoleMutation(
                () => setWorkspaceRoleStatus(workspace, role.id, value),
                '角色状态已更新',
              )
            }}
            value={status}
          />
        </Permission>
      ),
    },
    { title: '创建时间', dataIndex: 'createdAt', width: 170 },
    {
      title: '操作',
      key: 'actions',
      fixed: 'right',
      width: 300,
      render: (_, role) => {
        return (
          <Space size="small" wrap>
            {role.type === 'custom' && role.permissionConfigurable
              ? (
                  <Permission code={`${workspace}:role:edit`}>
                    <Button
                      onClick={() => openRoleEditDrawer(role)}
                      size="small"
                      type="link"
                    >
                      编辑
                    </Button>
                  </Permission>
                )
              : null}
            <Permission code={`${workspace}:role:permission`}>
              <Button
                onClick={() => {
                  void openRolePermissionDrawer(
                    role,
                    role.permissionConfigurable
                      ? 'permission'
                      : 'view',
                  )
                }}
                size="small"
                type="link"
              >
                {role.permissionConfigurable ? '配置权限' : '查看权限'}
              </Button>
            </Permission>
            <Permission code={`${workspace}:role:employees`}>
              <Button
                onClick={() => {
                  setRoleEmployeePage(1)
                  setEmployeeDrawerRole(role)
                }}
                size="small"
                type="link"
              >
                查看员工
              </Button>
            </Permission>
            {role.type === 'custom' && role.permissionConfigurable
              ? (
                  <Permission code={`${workspace}:role:delete`}>
                    <ConfirmDelete
                      onConfirm={() => {
                        void runRoleMutation(
                          () => deleteWorkspaceRole(workspace, role.id),
                          '角色已删除',
                        )
                      }}
                    >
                      <Button danger size="small" type="link">
                        删除
                      </Button>
                    </ConfirmDelete>
                  </Permission>
                )
              : null}
          </Space>
        )
      },
    },
  ]
  return (
    <PageContainer>
      <SearchTable<PlatformRole, RoleSearchValues>
        actions={(
          <Permission code={`${workspace}:role:create`}>
            <Button onClick={openRoleCreateDrawer} type="primary">
              新增角色
            </Button>
          </Permission>
        )}
        search={{
          fields: roleSearchFields,
          form: roleQueryForm,
          onReset: () => {
            setRolePage(1)
            setRoleQuery({})
          },
          onSearch: (values) => {
            setRolePage(1)
            setRoleQuery(values)
          },
        }}
        columns={roleTableColumns}
        dataSource={roles}
        loading={roleListLoading}
        onChange={(pagination: TablePaginationConfig) => {
          const size = pagination.pageSize ?? 10
          setRolePageSize(size)
          setRolePage(size === rolePageSize ? (pagination.current ?? 1) : 1)
        }}
        pagination={{
          current: rolePage,
          pageSize: rolePageSize,
          pageSizeOptions: [10, 20, 50, 100],
          showSizeChanger: true,
          showTotal: count => `共 ${count} 条`,
          total: roleTotal,
        }}
        rowKey="id"
        scroll={{ x: 'max-content' }}
      />
      <FormDrawer
        loading={roleMutationLoading}
        onClose={() => setRoleDrawerMode(null)}
        onSubmit={
          roleDrawerMode === 'view' ? undefined : () => roleForm.submit()
        }
        open={Boolean(roleDrawerMode)}
        readonly={roleDrawerMode === 'view'}
        title={
          roleDrawerMode === 'create'
            ? '新增角色'
            : roleDrawerMode === 'edit'
              ? '编辑角色'
              : roleDrawerMode === 'view'
                ? `查看角色权限${activeRole ? ` - ${activeRole.name}` : ''}`
                : `配置角色权限${activeRole ? ` - ${activeRole.name}` : ''}`
        }
        width={680}
      >
        <SchemaForm
          columns={1}
          fields={roleFields}
          form={roleForm}
          onFinish={handleRoleFormSubmit}
          showActions={false}
        />
      </FormDrawer>
      <FormDrawer
        onClose={() => setEmployeeDrawerRole(null)}
        open={Boolean(employeeDrawerRole)}
        readonly
        title={`角色员工${employeeDrawerRole ? ` - ${employeeDrawerRole.name}` : ''}`}
        width={900}
      >
        <PlatformEmployeeTable
          data={roleEmployees}
          loading={false}
          onPaginationChange={(next, size) => {
            setRoleEmployeePageSize(size)
            setRoleEmployeePage(size === roleEmployeePageSize ? next : 1)
          }}
          page={roleEmployeePage}
          pageSize={roleEmployeePageSize}
          total={roleEmployeeTotal}
        />
      </FormDrawer>
    </PageContainer>
  )
}
