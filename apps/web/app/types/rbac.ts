import type { WorkspaceType } from './auth'

/** 业务实体的通用启停状态。 */
export type EntityStatus = 'enabled' | 'disabled'

/** 员工列表、编辑表单和角色关联使用的员工实体。 */
export interface Employee {
  /** 员工唯一标识。 */
  id: string
  /** 员工关联的平台用户标识。 */
  userId: string
  /** 员工显示名称。 */
  name: string
  /** 员工用于登录系统的账号。 */
  loginAccount: string
  /** 员工所属的平台端或租户端工作空间。 */
  workspace: WorkspaceType
  /** 员工所属租户；平台员工不提供。 */
  tenantId?: string
  /** 员工所属部门；尚未分配部门时不提供。 */
  departmentId?: string
  /** 当前员工已关联的角色标识。 */
  roleIds: string[]
  /** 员工联系电话；未维护时不提供。 */
  phone?: string
  /** 员工账号是否允许正常使用。 */
  status: EntityStatus
  /** 员工记录创建时间，格式由服务端统一返回。 */
  createdAt: string
}

/** 角色生效的工作空间范围。 */
export type RoleScope = WorkspaceType
/** 角色来源类型：系统内置或业务自定义。 */
export type RoleType = 'system' | 'custom'
/** 系统内置角色的稳定业务键。 */
export type RoleSystemKey
  = | 'platform_super_admin'
    | 'platform_admin'
    | 'tenant_owner'

/** 角色列表、编辑和权限配置使用的角色实体。 */
export interface Role {
  /** 角色唯一标识。 */
  id: string
  /** 角色显示名称。 */
  name: string
  /** 角色用途说明；未填写时不提供。 */
  description?: string
  /** 角色生效的平台端或租户端范围。 */
  scope: RoleScope
  /** 角色所属租户；平台角色不提供。 */
  tenantId?: string
  /** 角色为系统内置还是业务自定义。 */
  type: RoleType
  /** 系统内置角色键；自定义角色不提供。 */
  systemKey?: RoleSystemKey
  /** 角色是否拥有跳过普通权限项判断的超级管理员能力。 */
  isSuperAdmin: boolean
  /** 角色是否允许分配和使用。 */
  status: EntityStatus
  /** 当前角色已关联的权限节点标识。 */
  permissionIds: string[]
  /** 当前关联该角色的员工数量。 */
  userCount: number
  /** 角色记录创建时间，格式由服务端统一返回。 */
  createdAt: string
}

/** 菜单节点的业务类型。 */
export type MenuNodeType = 'directory' | 'menu' | 'permission'
/** 菜单节点生效的工作空间范围。 */
export type MenuScope = WorkspaceType

/** 路由菜单和操作权限共同使用的树形节点。 */
export interface MenuNode {
  /** 菜单或权限节点唯一标识。 */
  id: string
  /** 父节点标识；根节点不提供。 */
  parentId?: string
  /** 节点显示名称。 */
  name: string
  /** 节点承担目录、页面菜单或操作权限中的哪一种职责。 */
  type: MenuNodeType
  /** 节点所属的平台端或租户端范围。 */
  scope: MenuScope
  /** 菜单节点对应的前端路由；目录和操作权限通常不提供。 */
  path?: string
  /** 菜单节点关联的组件标识；非页面节点不提供。 */
  component?: string
  /** 菜单展示使用的图标名称；未配置时不提供。 */
  icon?: string
  /** 权限校验使用的稳定编码；无权限要求的节点不提供。 */
  permissionCode?: string
  /** 平台菜单是否允许分配给租户侧角色。 */
  tenantAssignable: boolean
  /** 同级节点的展示顺序，数值越小越靠前。 */
  sort: number
  /** 节点是否出现在导航菜单中。 */
  visible: boolean
  /** 节点当前是否启用。 */
  status: EntityStatus
  /** 已按层级组装的子节点；叶子节点可不提供。 */
  children?: MenuNode[]
}

/** 组织架构树中的部门实体。 */
export interface Department {
  /** 部门唯一标识。 */
  id: string
  /** 上级部门标识；根部门不提供。 */
  parentId?: string
  /** 部门显示名称。 */
  name: string
  /** 部门所属的平台端或租户端工作空间。 */
  workspace: WorkspaceType
  /** 部门所属租户；平台部门不提供。 */
  tenantId?: string
  /** 部门负责人对应的员工标识；未指定时不提供。 */
  leaderEmployeeId?: string
  /** 部门负责人显示名称；未指定时不提供。 */
  leaderName?: string
  /** 部门当前直接或服务端口径下统计的员工数量。 */
  employeeCount: number
  /** 同级部门的展示顺序，数值越小越靠前。 */
  sort: number
  /** 部门当前是否启用。 */
  status: EntityStatus
  /** 已按组织层级组装的下级部门。 */
  children?: Department[]
}

/** 可供平台选择或进入的租户摘要。 */
export interface Tenant {
  /** 租户唯一标识。 */
  id: string
  /** 租户显示名称。 */
  name: string
  /** 租户当前是否可用。 */
  status: EntityStatus
}

/** 需要限定平台端或租户端数据范围的组件公共参数。 */
export interface WorkspaceProps {
  /** 组件当前操作的平台端或租户端工作空间。 */
  workspace: WorkspaceType
  /** 租户端数据范围的租户标识；平台端不提供。 */
  tenantId?: string
}
