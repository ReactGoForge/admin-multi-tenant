import type { FormFieldConfig } from '@/components/composite/schema-form'
import type { MenuMutation } from '@/services/platform/menus'

import type { WorkspaceType } from '@/types/auth'
import type { MenuNode, MenuNodeType } from '@/types/rbac'
import { App, Button, Form, Tabs } from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { buildMenuTree } from '@/auth/permission'
import { IconPicker } from '@/components/base/icon-picker'
import { PageContainer } from '@/components/base/page-container'
import { FormDrawer } from '@/components/composite/form-drawer'
import {

  SchemaForm,
} from '@/components/composite/schema-form'
import { SearchTable } from '@/components/composite/search-table'
import { Permission } from '@/components/domain/auth/permission'
import { useMenuTreeTableProps } from '@/components/domain/rbac/menu-management/menu-tree-table'
import { DICTIONARY_CODE, useDictionary } from '@/contexts/dictionary'
import { getErrorMessage } from '@/services/errors'
import { isSilentRequestError } from '@/services/http'
import {
  createPlatformMenu,
  deletePlatformMenu,
  fetchPlatformMenus,
  fetchWorkspaceMenus,
  setPlatformMenuStatus,
  updatePlatformMenu,

} from '@/services/platform/menus'

/** 菜单树搜索表单值。 */
interface MenuQuery {
  /** 按菜单名称模糊筛选，并保留命中节点的祖先路径。 */
  name?: string
  /** 按目录、菜单或权限节点类型筛选。 */
  type?: MenuNode['type']
  /** 按启用或禁用状态筛选。 */
  status?: MenuNode['status']
}

/** 菜单节点允许绑定的前端静态页面。 */
interface RegisteredPage {
  /** 页面选择器展示的中文名称。 */
  label: string
  /** 页面对应的稳定路由路径。 */
  path: string
  /** 数据库菜单记录保存的组件入口标识。 */
  component: string
}

/** 平台端和租户端允许登记到菜单节点的静态页面白名单。 */
const registeredPages: Record<WorkspaceType, RegisteredPage[]> = {
  platform: [
    {
      label: '租户管理',
      path: '/platform/tenants',
      component: 'pages/platform/tenants/index.tsx',
    },
    {
      label: '图片库',
      path: '/platform/images',
      component: 'pages/platform/images/index.tsx',
    },
    {
      label: '员工管理',
      path: '/platform/system/employees',
      component: 'pages/platform/system/employees/index.tsx',
    },
    {
      label: '角色管理',
      path: '/platform/system/roles',
      component: 'pages/platform/system/roles/index.tsx',
    },
    {
      label: '菜单管理',
      path: '/platform/system/menus',
      component: 'pages/platform/system/menus/index.tsx',
    },
    {
      label: '部门管理',
      path: '/platform/system/departments',
      component: 'pages/platform/system/departments/index.tsx',
    },
    {
      label: '基础设置',
      path: '/platform/system/basic',
      component: 'pages/platform/system/basic/index.tsx',
    },
    {
      label: '字典管理',
      path: '/platform/system/fields',
      component: 'pages/platform/system/fields/index.tsx',
    },
    {
      label: '用户',
      path: '/platform/users',
      component: 'pages/platform/users/index.tsx',
    },
  ],
  tenant: [
    {
      label: '图片库',
      path: '/tenant/images',
      component: 'pages/tenant/images/index.tsx',
    },
    {
      label: '员工管理',
      path: '/tenant/system/employees',
      component: 'pages/tenant/system/employees/index.tsx',
    },
    {
      label: '角色管理',
      path: '/tenant/system/roles',
      component: 'pages/tenant/system/roles/index.tsx',
    },
    {
      label: '菜单',
      path: '/tenant/system/menus',
      component: 'pages/tenant/system/menus/index.tsx',
    },
    {
      label: '部门管理',
      path: '/tenant/system/departments',
      component: 'pages/tenant/system/departments/index.tsx',
    },
    {
      label: '用户',
      path: '/tenant/users',
      component: 'pages/tenant/users/index.tsx',
    },
  ],
}

/** collectMenuKeys 递归收集菜单树中的全部节点 ID，用于展开全部树节点。 */
function collectMenuKeys(nodes: MenuNode[]): string[] {
  return nodes.flatMap(node => [
    node.id,
    ...(node.children ? collectMenuKeys(node.children) : []),
  ])
}

/** filterMenuTree 按查询条件筛选菜单，同时保留命中子节点所需的完整父级路径。 */
function filterMenuTree(nodes: MenuNode[], query: MenuQuery): MenuNode[] {
  return nodes.flatMap((node) => {
    const children = node.children
      ? filterMenuTree(node.children, query)
      : undefined
    const matched
      = (!query.name || node.name.includes(query.name))
        && (!query.type || node.type === query.type)
        && (!query.status || node.status === query.status)
    if (!matched && !children?.length)
      return []
    return [{ ...node, children }]
  })
}

/** isProtectedPlatformMenu 判断节点是否属于防止平台超级管理员自锁的核心菜单权限。 */
function isProtectedPlatformMenu(node: MenuNode | null) {
  return Boolean(
    node?.scope === 'platform'
    && node.permissionCode?.startsWith('platform:menu:'),
  )
}

/** allowedChildTypes 根据父节点层级返回允许创建的目录、菜单或操作权限类型。 */
function allowedChildTypes(parent: MenuNode | null): MenuNodeType[] {
  if (!parent)
    return ['directory', 'menu']
  if (parent.type === 'directory')
    return ['directory', 'menu']
  if (parent.type === 'menu')
    return ['permission']
  return []
}

/** 菜单管理组件配置。 */
interface PlatformMenuManagementProps {
  /** 当前页面工作空间，默认 platform；租户工作空间保持只读。 */
  workspace?: WorkspaceType
}

/** PlatformMenuManagement 管理平台统一维护的真实平台端与租户端菜单。 */
export function PlatformMenuManagement({
  workspace = 'platform',
}: PlatformMenuManagementProps) {
  // 页面反馈和字典选项：分别提供菜单节点类型、状态和工作空间展示文案。
  const { message } = App.useApp()
  const { getLabel, getOptions } = useDictionary()
  const nodeTypeOptions = getOptions(DICTIONARY_CODE.menuNodeType)
  const statusOptions = getOptions(DICTIONARY_CODE.entityStatus)

  // 菜单查询、列表和树展开状态：
  // - menuQueryForm/menuQuery：控制搜索输入并保存已提交条件。
  // - activeScope：平台工作空间内当前维护的平台端或租户端菜单范围。
  // - menus：接口返回的平铺菜单数据，派生为完整树和筛选树。
  // - menuListLoading/menuListRefreshVersion：菜单请求状态和写操作后的刷新触发值。
  // - expandedRowKeys：菜单树当前展开的节点 ID。
  const [menuQueryForm] = Form.useForm<MenuQuery>()
  const [activeScope, setActiveScope] = useState<WorkspaceType>('platform')
  const [menus, setMenus] = useState<MenuNode[]>([])
  const [menuQuery, setMenuQuery] = useState<MenuQuery>({})
  const [menuListLoading, setMenuListLoading] = useState(false)
  const [menuListRefreshVersion, setMenuListRefreshVersion] = useState(0)
  const [expandedRowKeys, setExpandedRowKeys] = useState<React.Key[]>([])

  // 菜单表单抽屉状态：
  // - menuForm/editingMenu：控制新增或编辑数据及当前目标节点。
  // - fixedParentMenu：从“新增子节点”进入时锁定的父节点。
  // - menuFormOpen/menuMutationLoading：抽屉开关和保存状态。
  // - watchedNodeType/protectedEditingMenu：根据表单类型和核心节点保护规则控制字段。
  const [menuForm] = Form.useForm<MenuMutation>()
  const [editingMenu, setEditingMenu] = useState<MenuNode | null>(null)
  const [fixedParentMenu, setFixedParentMenu] = useState<MenuNode | null>(null)
  const [menuFormOpen, setMenuFormOpen] = useState(false)
  const [menuMutationLoading, setMenuMutationLoading] = useState(false)
  const watchedNodeType = Form.useWatch('type', menuForm)
  const protectedEditingMenu = isProtectedPlatformMenu(editingMenu)

  /** handleMenuRequestError 统一展示菜单请求错误，并忽略主动取消的请求。 */
  const handleMenuRequestError = useCallback(
    (error: unknown, fallback = '菜单数据加载失败') => {
      if (!isSilentRequestError(error)) {
        void message.error(getErrorMessage(error, fallback))
      }
    },
    [message],
  )

  // 工作空间、菜单范围或刷新版本变化时重新加载菜单，并重置旧树的展开状态。
  useEffect(() => {
    void menuListRefreshVersion
    const controller = new AbortController()
    setMenus([])
    setExpandedRowKeys([])
    setMenuListLoading(true)
    void (
      workspace === 'platform'
        ? fetchPlatformMenus(activeScope, controller.signal)
        : fetchWorkspaceMenus('tenant', controller.signal)
    )
      .then((result) => {
        setMenus(result)
        setExpandedRowKeys(collectMenuKeys(buildMenuTree(result)))
      })
      .catch(handleMenuRequestError)
      .finally(() => {
        if (!controller.signal.aborted)
          setMenuListLoading(false)
      })
    return () => controller.abort()
  }, [activeScope, handleMenuRequestError, menuListRefreshVersion, workspace])

  // 将平铺菜单转换为树，再按查询条件生成保留祖先路径的展示树。
  const fullMenuTree = useMemo(() => buildMenuTree(menus), [menus])
  const filteredMenuTree = useMemo(
    () => filterMenuTree(fullMenuTree, menuQuery),
    [fullMenuTree, menuQuery],
  )
  const allMenuKeys = useMemo(
    () => collectMenuKeys(fullMenuTree),
    [fullMenuTree],
  )

  // 菜单树支持按名称、节点类型和状态筛选。
  const menuSearchFields = useMemo<Array<FormFieldConfig<MenuQuery>>>(
    () => [
      { name: 'name', label: '菜单名称' },
      {
        name: 'type',
        label: '菜单类型',
        type: 'select',
        options: nodeTypeOptions,
      },
      {
        name: 'status',
        label: '状态',
        type: 'select',
        options: statusOptions,
      },
    ],
    [nodeTypeOptions, statusOptions],
  )

  /** openMenuCreateDrawer 根据目标父节点设置允许的默认节点类型并打开新增抽屉。 */
  const openMenuCreateDrawer = (parent: MenuNode | null) => {
    const type = allowedChildTypes(parent)[0] ?? 'directory'
    setEditingMenu(null)
    setFixedParentMenu(parent)
    menuForm.resetFields()
    menuForm.setFieldsValue({
      scope: activeScope,
      parentId: parent?.id,
      type,
      tenantAssignable: activeScope === 'tenant',
      sort: 10,
      visible: type !== 'permission',
      status: 'enabled',
    })
    setMenuFormOpen(true)
  }

  /** openMenuEditDrawer 回填所选菜单节点的可编辑字段并打开编辑抽屉。 */
  const openMenuEditDrawer = (node: MenuNode) => {
    setEditingMenu(node)
    setFixedParentMenu(null)
    menuForm.setFieldsValue({
      scope: node.scope,
      parentId: node.parentId,
      name: node.name,
      type: node.type,
      path: node.path,
      component: node.component,
      icon: node.icon,
      permissionCode: node.permissionCode,
      tenantAssignable: node.tenantAssignable,
      sort: node.sort,
      visible: node.visible,
      status: node.status,
    })
    setMenuFormOpen(true)
  }

  /** handleMenuSubmit 按节点类型清理无效字段，并提交菜单新增或编辑。 */
  const handleMenuSubmit = async (values: MenuMutation) => {
    const page = values.path
      ? registeredPages[activeScope].find(item => item.path === values.path)
      : undefined
    const payload: MenuMutation = {
      scope: activeScope,
      parentId: fixedParentMenu?.id ?? values.parentId,
      name: values.name,
      type: values.type,
      path: values.type === 'menu' ? page?.path : undefined,
      component: values.type === 'menu' ? page?.component : undefined,
      icon: values.type === 'permission' ? undefined : values.icon,
      permissionCode:
        values.type === 'directory' ? undefined : values.permissionCode,
      tenantAssignable: activeScope === 'tenant' && values.tenantAssignable,
      sort: values.sort,
      visible: values.type !== 'permission' && values.visible,
      status: values.status,
    }
    setMenuMutationLoading(true)
    try {
      if (editingMenu)
        await updatePlatformMenu(editingMenu.id, payload)
      else await createPlatformMenu(payload)
      void message.success(editingMenu ? '菜单已更新' : '菜单已新增')
      setMenuFormOpen(false)
      setMenuListRefreshVersion(value => value + 1)
    }
    catch (error) {
      handleMenuRequestError(error, '操作失败')
    }
    finally {
      setMenuMutationLoading(false)
    }
  }

  // 父节点选项根据当前节点类型限制为目录或页面菜单，并排除正在编辑的节点自身。
  const parentMenuOptions = menus
    .filter(item =>
      watchedNodeType === 'permission'
        ? item.type === 'menu'
        : item.type === 'directory',
    )
    .filter(item => item.id !== editingMenu?.id)
    .map(item => ({ label: item.name, value: item.id }))
  const menuTypeOptions = (
    editingMenu ? [editingMenu.type] : allowedChildTypes(fixedParentMenu)
  ).map(type => ({
    label: getLabel(DICTIONARY_CODE.menuNodeType, type),
    value: type,
  }))

  // 菜单表单字段随节点类型、父节点锁定状态和核心节点保护规则动态变化。
  const menuFormFields: Array<FormFieldConfig<MenuMutation>> = [
    {
      name: 'parentId',
      label: '上级节点',
      type: 'select',
      disabled: Boolean(fixedParentMenu) || protectedEditingMenu,
      options: parentMenuOptions,
      componentProps: { allowClear: watchedNodeType !== 'permission' },
    },
    {
      name: 'type',
      label: '节点类型',
      type: 'select',
      disabled: Boolean(editingMenu),
      rules: [{ required: true, message: '请选择节点类型' }],
      options: menuTypeOptions,
    },
    {
      name: 'name',
      label: '名称',
      rules: [{ required: true, message: '请输入名称' }],
      componentProps: { maxLength: 40 },
    },
    {
      name: 'path',
      label: '静态页面',
      type: 'select',
      hidden: watchedNodeType !== 'menu',
      disabled: protectedEditingMenu,
      rules: [{ required: true, message: '请选择静态页面' }],
      options: registeredPages[activeScope].map(page => ({
        label: `${page.label}（${page.path}）`,
        value: page.path,
        disabled: menus.some(
          item => item.path === page.path && item.id !== editingMenu?.id,
        ),
      })),
    },
    {
      name: 'permissionCode',
      label: '权限标识',
      hidden: watchedNodeType === 'directory',
      disabled: protectedEditingMenu,
      rules: [
        { required: true, message: '请输入权限标识' },
        {
          pattern: /^[a-z][a-z0-9-]*(?::[a-z][a-z0-9-]*)+$/,
          message: '权限标识格式不正确',
        },
      ],
      componentProps: { maxLength: 100 },
    },
    {
      name: 'icon',
      label: '图标',
      hidden: watchedNodeType === 'permission',
      render: () => <IconPicker />,
    },
    {
      name: 'visible',
      label: '导航显示',
      type: 'switch',
      hidden: watchedNodeType === 'permission',
      disabled: protectedEditingMenu,
    },
    {
      name: 'tenantAssignable',
      label: '租户角色可分配',
      type: 'switch',
      hidden: activeScope !== 'tenant',
    },
    {
      name: 'sort',
      label: '排序',
      type: 'number',
      rules: [{ required: true, message: '请输入排序' }],
      componentProps: { min: 0 },
    },
    {
      name: 'status',
      label: '状态',
      type: 'select',
      disabled: Boolean(editingMenu),
      rules: [{ required: true, message: '请选择状态' }],
      options: statusOptions,
    },
  ]

  // 将筛选后的菜单树、展开状态和受权限控制的行操作组装为公共树表格配置。
  const menuTableProps = useMenuTreeTableProps({
    data: filteredMenuTree,
    editable: workspace === 'platform',
    expandedRowKeys,
    loading: menuListLoading,
    onAddChild: openMenuCreateDrawer,
    onDelete: async (node) => {
      try {
        await deletePlatformMenu(node.id)
        void message.success('菜单已删除')
        setMenuListRefreshVersion(value => value + 1)
      }
      catch (error) {
        handleMenuRequestError(error, '删除失败')
      }
    },
    onEdit: openMenuEditDrawer,
    onExpandedRowsChange: keys => setExpandedRowKeys([...keys]),
    onStatusChange: async (node, enabled) => {
      try {
        await setPlatformMenuStatus(node.id, enabled ? 'enabled' : 'disabled')
        void message.success('菜单状态已更新')
        setMenuListRefreshVersion(value => value + 1)
      }
      catch (error) {
        handleMenuRequestError(error, '状态更新失败')
      }
    },
  })

  return (
    <PageContainer>
      <SearchTable<MenuNode, MenuQuery>
        actions={(
          <>
            {workspace === 'platform'
              ? (
                  <Permission code="platform:menu:create">
                    <Button
                      onClick={() => openMenuCreateDrawer(null)}
                      type="primary"
                    >
                      新增根节点
                    </Button>
                  </Permission>
                )
              : null}
            <Button onClick={() => setExpandedRowKeys(allMenuKeys)}>
              展开全部
            </Button>
            <Button onClick={() => setExpandedRowKeys([])}>折叠全部</Button>
          </>
        )}
        search={{
          fields: menuSearchFields,
          form: menuQueryForm,
          onReset: () => setMenuQuery({}),
          onSearch: setMenuQuery,
        }}
        tableHeader={
          workspace === 'platform'
            ? (
                <Tabs
                  activeKey={activeScope}
                  items={[
                    {
                      key: 'platform',
                      label: getLabel(DICTIONARY_CODE.workspaceScope, 'platform'),
                    },
                    {
                      key: 'tenant',
                      label: getLabel(DICTIONARY_CODE.workspaceScope, 'tenant'),
                    },
                  ]}
                  onChange={(scope) => {
                    setActiveScope(scope as WorkspaceType)
                    setMenuQuery({})
                    setMenuFormOpen(false)
                    menuQueryForm.resetFields()
                  }}
                />
              )
            : undefined
        }
        {...menuTableProps}
      />

      <FormDrawer
        loading={menuMutationLoading}
        onClose={() => setMenuFormOpen(false)}
        onSubmit={() => menuForm.submit()}
        open={menuFormOpen}
        title={
          editingMenu
            ? '编辑菜单节点'
            : fixedParentMenu
              ? '新增子节点'
              : '新增根节点'
        }
      >
        <SchemaForm
          columns={1}
          fields={menuFormFields}
          form={menuForm}
          onFinish={handleMenuSubmit}
          showActions={false}
        />
      </FormDrawer>
    </PageContainer>
  )
}
