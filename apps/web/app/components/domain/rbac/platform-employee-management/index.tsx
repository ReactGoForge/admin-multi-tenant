import type { FormFieldConfig } from '@/components/composite/schema-form'
import type { EmployeeMutation, PlatformEmployee, PlatformEmployeeOptions } from '@/services/platform/employees'

import type { WorkspaceType } from '@/types/auth'
import type { EntityStatus } from '@/types/rbac'
import { App, Button, Form } from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { PageContainer } from '@/components/base/page-container'
import { DepartmentSelect, RoleSelect } from '@/components/base/selects'
import { FormDrawer } from '@/components/composite/form-drawer'
import {

  SchemaForm,
} from '@/components/composite/schema-form'
import { SearchTable } from '@/components/composite/search-table'
import { Permission } from '@/components/domain/auth/permission'
import { usePlatformEmployeeTableProps } from '@/components/domain/rbac/platform-employee-table'
import { DICTIONARY_CODE, useDictionary } from '@/contexts/dictionary'
import { getErrorMessage } from '@/services/errors'
import { isSilentRequestError } from '@/services/http'
import {
  assignWorkspaceEmployeeRoles,
  createWorkspaceEmployee,
  deleteWorkspaceEmployee,
  fetchWorkspaceEmployeeOptions,
  fetchWorkspaceEmployees,
  resetWorkspaceEmployeePassword,
  setWorkspaceEmployeeStatus,
  updateWorkspaceEmployee,

} from '@/services/platform/employees'
import { useAuthStore } from '@/stores/auth'

/** 员工列表搜索表单值。 */
interface EmployeeSearchValues {
  /** 按员工姓名模糊筛选。 */
  name?: string
  /** 按登录账号模糊筛选。 */
  loginAccount?: string
  /** 按所属部门 ID 精确筛选。 */
  departmentId?: string
  /** 按所属角色 ID 精确筛选。 */
  roleId?: string
  /** 按启用或禁用状态筛选。 */
  status?: EntityStatus
}

/** 员工新增或编辑表单值，在接口字段基础上固定角色和状态类型。 */
type EmployeeFormValues = EmployeeMutation & {
  /** 新增员工时选择的角色 ID；编辑后通过独立角色抽屉维护。 */
  roleIds: string[]
  /** 员工启用状态；新增时默认启用。 */
  status: EntityStatus
}

/** 员工密码重置表单值。 */
interface PasswordFormValues {
  /** 需要写入的新密码。 */
  password: string
}

/** 员工角色分配表单值。 */
interface RoleAssignmentValues {
  /** 保存后需要关联到员工的角色 ID。 */
  roleIds: string[]
}

/** 员工筛选和表单下拉选项的空值。 */
const emptyOptions: PlatformEmployeeOptions = { roles: [], departments: [] }

/** 员工管理组件配置。 */
interface PlatformEmployeeManagementProps {
  /** 管理的平台或租户工作空间，默认 platform。 */
  workspace?: WorkspaceType
}

/** PlatformEmployeeManagement 管理平台或当前租户的真实员工数据。 */
export function PlatformEmployeeManagement({
  workspace = 'platform',
}: PlatformEmployeeManagementProps) {
  // 页面基础能力和字典选项：message 负责操作反馈，statusOptions 提供状态表单选项。
  const { message } = App.useApp()
  const { getOptions } = useDictionary()
  const statusOptions = getOptions(DICTIONARY_CODE.entityStatus)
  const refreshCurrentUser = useAuthStore(state => state.refreshCurrentUser)
  const currentEmployeeId = useAuthStore((state) => {
    const currentUser = state.currentUser
    return currentUser?.mode === 'normal' && currentUser.workspace === workspace
      ? currentUser.employeeId
      : undefined
  })

  // 员工查询和列表状态：
  // - employeeQueryForm/employeeQuery：控制搜索输入并保存已提交条件。
  // - employeePage/employeePageSize：服务端分页位置和每页数量。
  // - employees/employeeTotal：当前页员工和满足条件的总数量。
  // - employeeOptions：角色和部门筛选、表单选项。
  // - employeeListLoading/employeeListRefreshVersion：列表加载状态和写操作后的刷新触发值。
  const [employeeQueryForm] = Form.useForm<EmployeeSearchValues>()
  const [employeeQuery, setEmployeeQuery] = useState<EmployeeSearchValues>({})
  const [employeePage, setEmployeePage] = useState(1)
  const [employeePageSize, setEmployeePageSize] = useState(10)
  const [employees, setEmployees] = useState<PlatformEmployee[]>([])
  const [employeeTotal, setEmployeeTotal] = useState(0)
  const [employeeOptions, setEmployeeOptions]
    = useState<PlatformEmployeeOptions>(emptyOptions)
  const [employeeListLoading, setEmployeeListLoading] = useState(false)
  const [employeeListRefreshVersion, setEmployeeListRefreshVersion]
    = useState(0)

  // 员工操作抽屉状态：
  // - employeeForm/editingEmployee/employeeFormOpen：控制员工新增或编辑。
  // - employeeRoleForm/roleAssignmentEmployee：控制目标员工的角色分配。
  // - employeePasswordForm/passwordResetEmployee：控制目标员工的密码重置。
  // - employeeMutationLoading：所有员工写操作共用的提交状态。
  const [employeeForm] = Form.useForm<EmployeeFormValues>()
  const [employeeRoleForm] = Form.useForm<RoleAssignmentValues>()
  const [employeePasswordForm] = Form.useForm<PasswordFormValues>()
  const [employeeMutationLoading, setEmployeeMutationLoading] = useState(false)
  const [editingEmployee, setEditingEmployee]
    = useState<PlatformEmployee | null>(null)
  const [roleAssignmentEmployee, setRoleAssignmentEmployee]
    = useState<PlatformEmployee | null>(null)
  const [passwordResetEmployee, setPasswordResetEmployee]
    = useState<PlatformEmployee | null>(null)
  const [employeeFormOpen, setEmployeeFormOpen] = useState(false)

  /** handleEmployeeRequestError 统一展示员工请求错误，并忽略主动取消的请求。 */
  const handleEmployeeRequestError = useCallback(
    (error: unknown, fallback = '员工数据加载失败') => {
      if (!isSilentRequestError(error))
        void message.error(getErrorMessage(error, fallback))
    },
    [message],
  )

  // 工作空间变化时重新加载可用角色和部门选项，并在卸载时取消旧请求。
  useEffect(() => {
    const controller = new AbortController()
    void fetchWorkspaceEmployeeOptions(workspace, controller.signal)
      .then(setEmployeeOptions)
      .catch(handleEmployeeRequestError)
    return () => controller.abort()
  }, [handleEmployeeRequestError, workspace])

  // 搜索、分页或刷新版本变化时重新加载员工列表，取消请求后不再关闭新请求的加载状态。
  useEffect(() => {
    void employeeListRefreshVersion
    const controller = new AbortController()
    setEmployeeListLoading(true)
    void fetchWorkspaceEmployees(
      workspace,
      {
        page: employeePage,
        pageSize: employeePageSize,
        ...employeeQuery,
      },
      controller.signal,
    )
      .then((result) => {
        setEmployees(result.items)
        setEmployeeTotal(result.total)
      })
      .catch(handleEmployeeRequestError)
      .finally(() => {
        if (!controller.signal.aborted)
          setEmployeeListLoading(false)
      })
    return () => controller.abort()
  }, [
    employeeListRefreshVersion,
    employeePage,
    employeePageSize,
    employeeQuery,
    handleEmployeeRequestError,
    workspace,
  ])

  // 员工列表支持按姓名、账号、部门、角色和状态筛选。
  const employeeSearchFields = useMemo<
    Array<FormFieldConfig<EmployeeSearchValues>>
  >(
    () => [
      { name: 'name', label: '员工姓名' },
      { name: 'loginAccount', label: '登录账号' },
      {
        name: 'departmentId',
        label: '所属部门',
        render: () => (
          <DepartmentSelect options={employeeOptions.departments} />
        ),
      },
      {
        name: 'roleId',
        label: '所属角色',
        render: () => <RoleSelect options={employeeOptions.roles} />,
      },
      {
        name: 'status',
        label: '状态',
        type: 'select',
        options: statusOptions,
      },
    ],
    [employeeOptions, statusOptions],
  )

  // 员工新增与编辑字段；编辑时隐藏初始密码和角色，分别由独立操作维护。
  const employeeFormFields = useMemo<
    Array<FormFieldConfig<EmployeeFormValues>>
  >(
    () => [
      {
        name: 'name',
        label: '员工姓名',
        rules: [{ required: true, message: '请输入员工姓名' }],
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
        hidden: Boolean(editingEmployee),
        rules: [
          { required: true, min: 6, max: 18, message: '请输入 6 至 18 位密码' },
        ],
        componentProps: { maxLength: 18 },
      },
      { name: 'phone', label: '手机号', componentProps: { maxLength: 20 } },
      {
        name: 'departmentId',
        label: '所属部门',
        render: () => (
          <DepartmentSelect
            options={employeeOptions.departments}
            showStatus={false}
          />
        ),
      },
      {
        name: 'roleIds',
        label: '所属角色',
        hidden: Boolean(editingEmployee),
        rules: [{ required: true, message: '请选择所属角色' }],
        render: () => (
          <RoleSelect
            mode="multiple"
            options={employeeOptions.roles.map(role => ({
              ...role,
              disabled: !role.assignable,
            }))}
            showStatus={false}
          />
        ),
      },
    ],
    [editingEmployee, employeeOptions],
  )

  // 角色分配字段保留员工已有但当前不可分配的受保护角色，并将其设为禁用选项。
  const roleAssignmentFields = useMemo<
    Array<FormFieldConfig<RoleAssignmentValues>>
  >(() => {
    const assignableRoleIds = new Set(
      employeeOptions.roles.map(role => role.id),
    )
    const protectedRoles
      = roleAssignmentEmployee?.roles.filter(
        role => !assignableRoleIds.has(role.id),
      ) ?? []

    return [
      {
        name: 'roleIds',
        label: '所属角色',
        rules: [{ required: true, message: '请选择所属角色' }],
        render: () => (
          <RoleSelect
            mode="multiple"
            options={[
              ...employeeOptions.roles.map(role => ({
                ...role,
                disabled: !role.assignable,
              })),
              ...protectedRoles.map(role => ({
                ...role,
                disabled: true,
              })),
            ]}
            showStatus={false}
          />
        ),
      },
    ]
  }, [employeeOptions.roles, roleAssignmentEmployee])

  // 密码重置抽屉只提交新密码。
  const passwordResetFields = useMemo<
    Array<FormFieldConfig<PasswordFormValues>>
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

  /** openEmployeeCreateDrawer 重置员工表单并以启用状态和空角色打开新增抽屉。 */
  const openEmployeeCreateDrawer = () => {
    setEditingEmployee(null)
    employeeForm.resetFields()
    employeeForm.setFieldsValue({ status: 'enabled', roleIds: [] })
    setEmployeeFormOpen(true)
  }

  /** openEmployeeEditDrawer 回填所选员工的可编辑资料并打开编辑抽屉。 */
  const openEmployeeEditDrawer = (employee: PlatformEmployee) => {
    setEditingEmployee(employee)
    employeeForm.setFieldsValue({
      name: employee.name,
      loginAccount: employee.loginAccount,
      phone: employee.phone ?? undefined,
      departmentId: employee.department?.id,
      roleIds: employee.roles.map(role => role.id),
      status: employee.status,
    })
    setEmployeeFormOpen(true)
  }

  /** refreshEmployeeList 递增刷新版本，使员工列表重新请求当前页。 */
  const refreshEmployeeList = () =>
    setEmployeeListRefreshVersion(value => value + 1)

  /** runEmployeeMutation 统一执行员工写操作，并处理提交状态、反馈和列表刷新。 */
  const runEmployeeMutation = async (
    operation: () => Promise<unknown>,
    success: string,
  ) => {
    setEmployeeMutationLoading(true)
    try {
      await operation()
      void message.success(success)
      refreshEmployeeList()
      return true
    }
    catch (error) {
      handleEmployeeRequestError(error, '操作失败')
      return false
    }
    finally {
      setEmployeeMutationLoading(false)
    }
  }

  /** handleEmployeeSubmit 根据是否存在编辑目标提交新增或编辑，并在成功后关闭抽屉。 */
  const handleEmployeeSubmit = async (values: EmployeeFormValues) => {
    const success = await runEmployeeMutation(
      () =>
        editingEmployee
          ? updateWorkspaceEmployee(workspace, editingEmployee.id, values)
          : createWorkspaceEmployee(workspace, values),
      editingEmployee ? '员工已更新' : '员工已新增',
    )
    if (success) {
      if (editingEmployee?.id === currentEmployeeId) {
        try {
          await refreshCurrentUser()
        }
        catch (error) {
          handleEmployeeRequestError(error, '员工已更新，但当前会话刷新失败，请刷新页面')
        }
      }
      setEmployeeFormOpen(false)
    }
  }

  // 将员工列表数据、分页状态和行操作组装成公共员工表格配置。
  const employeeTableProps = usePlatformEmployeeTableProps({
    data: employees,
    loading: employeeListLoading,
    onAssignRole: (employee) => {
      setRoleAssignmentEmployee(employee)
      employeeRoleForm.setFieldsValue({
        roleIds: employee.roles.map(role => role.id),
      })
    },
    onDelete: (employee) => {
      void runEmployeeMutation(
        () => deleteWorkspaceEmployee(workspace, employee.id),
        '员工已删除',
      )
    },
    onEdit: openEmployeeEditDrawer,
    onPaginationChange: (nextPage, nextSize) => {
      setEmployeePageSize(nextSize)
      setEmployeePage(nextSize === employeePageSize ? nextPage : 1)
    },
    onResetPassword: (employee) => {
      setPasswordResetEmployee(employee)
      employeePasswordForm.resetFields()
    },
    onStatusChange: (employee, status) => {
      void runEmployeeMutation(
        () => setWorkspaceEmployeeStatus(workspace, employee.id, status),
        '员工状态已更新',
      )
    },
    currentEmployeeId,
    page: employeePage,
    pageSize: employeePageSize,
    total: employeeTotal,
    workspace,
  })

  return (
    <PageContainer>
      <SearchTable<PlatformEmployee, EmployeeSearchValues>
        actions={(
          <Permission code={`${workspace}:employee:create`}>
            <Button onClick={openEmployeeCreateDrawer} type="primary">
              新增员工
            </Button>
          </Permission>
        )}
        search={{
          fields: employeeSearchFields,
          form: employeeQueryForm,
          onReset: () => {
            setEmployeePage(1)
            setEmployeeQuery({})
          },
          onSearch: (values) => {
            setEmployeePage(1)
            setEmployeeQuery(values)
          },
        }}
        {...employeeTableProps}
      />

      <FormDrawer
        loading={employeeMutationLoading}
        onClose={() => setEmployeeFormOpen(false)}
        onSubmit={() => employeeForm.submit()}
        open={employeeFormOpen}
        title={editingEmployee ? '编辑员工' : '新增员工'}
      >
        <SchemaForm<EmployeeFormValues>
          columns={1}
          fields={employeeFormFields}
          form={employeeForm}
          onFinish={handleEmployeeSubmit}
          showActions={false}
        />
      </FormDrawer>

      <FormDrawer
        loading={employeeMutationLoading}
        onClose={() => setRoleAssignmentEmployee(null)}
        onSubmit={() => employeeRoleForm.submit()}
        open={Boolean(roleAssignmentEmployee)}
        title={`分配角色${roleAssignmentEmployee ? ` - ${roleAssignmentEmployee.name}` : ''}`}
      >
        <SchemaForm
          columns={1}
          fields={roleAssignmentFields}
          form={employeeRoleForm}
          onFinish={async ({ roleIds }) => {
            if (
              roleAssignmentEmployee
              && (await runEmployeeMutation(
                () =>
                  assignWorkspaceEmployeeRoles(
                    workspace,
                    roleAssignmentEmployee.id,
                    roleIds,
                  ),
                '角色已更新',
              ))
            ) {
              setRoleAssignmentEmployee(null)
            }
          }}
          showActions={false}
        />
      </FormDrawer>

      <FormDrawer
        loading={employeeMutationLoading}
        onClose={() => setPasswordResetEmployee(null)}
        onSubmit={() => employeePasswordForm.submit()}
        open={Boolean(passwordResetEmployee)}
        title={`重置密码${passwordResetEmployee ? ` - ${passwordResetEmployee.name}` : ''}`}
      >
        <SchemaForm
          columns={1}
          fields={passwordResetFields}
          form={employeePasswordForm}
          onFinish={async ({ password }) => {
            if (
              passwordResetEmployee
              && (await runEmployeeMutation(
                () =>
                  resetWorkspaceEmployeePassword(
                    workspace,
                    passwordResetEmployee.id,
                    password,
                  ),
                '密码已重置',
              ))
            ) {
              setPasswordResetEmployee(null)
            }
          }}
          showActions={false}
        />
      </FormDrawer>
    </PageContainer>
  )
}
