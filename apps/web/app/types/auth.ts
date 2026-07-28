import type { MenuNode } from './rbac'

/** 当前会话所在的业务空间：平台管理端或具体租户端。 */
export type WorkspaceType = 'platform' | 'tenant'

/** 当前访问模式：正常登录或由平台进入租户的代管模式。 */
export type AccessMode = 'normal' | 'managed'

/** 当前会话中用于展示和识别权限来源的角色摘要。 */
export interface CurrentSessionRole {
  /** 角色唯一标识。 */
  id: string
  /** 角色显示名称。 */
  name: string
  /** 系统内置角色的稳定键；自定义角色没有该值。 */
  systemKey: string | null
}

/** 权限计算所需的当前工作空间、角色和权限集合。 */
export interface AuthContext {
  /** 权限判断所属的平台端或租户端。 */
  workspace: WorkspaceType
  /** 当前是否处于平台代管租户的访问模式。 */
  mode: AccessMode
  /** 租户端权限所属租户；平台端不提供。 */
  tenantId?: string
  /** 当前用户在本工作空间内是否拥有超级管理员能力。 */
  isSuperAdmin: boolean
  /** 当前工作空间内生效的角色标识集合。 */
  roleIds: string[]
  /** 当前工作空间内已授权的权限编码集合。 */
  permissions: string[]
}

/** 登录成功后由当前用户接口返回的完整会话身份。 */
export interface CurrentSessionUser {
  /** 当前工作空间中的员工标识。 */
  employeeId: string
  /** 当前用户显示名称。 */
  name: string
  /** 当前员工用于登录后台的账号。 */
  loginAccount: string
  /** 当前员工手机号；未填写时为 null。 */
  phone: string | null
  /** 头像缺失时展示的文本缩写。 */
  avatarText: string
  /** 当前员工头像的临时访问地址；未设置时为 null。 */
  avatarUrl: string | null
  /** 当前会话所在工作空间。 */
  workspace: WorkspaceType
  /** 当前租户标识；平台端会话为 null。 */
  tenantId: string | null
  /** 当前租户名称；平台端会话为 null。 */
  tenantName: string | null
  /** 当前租户图标地址；未配置或平台端会话为 null。 */
  tenantIconUrl: string | null
  /** 平台品牌显示名称。 */
  platformName: string
  /** 平台品牌图标地址；未配置时为 null。 */
  platformIconUrl: string | null
  /** 当前会话的访问模式。 */
  mode: AccessMode
  /** 当前用户是否拥有超级管理员能力。 */
  isSuperAdmin: boolean
  /** 当前工作空间中生效的角色摘要。 */
  roles: CurrentSessionRole[]
  /** 当前工作空间中生效的权限编码。 */
  permissions: string[]
  /** 当前用户可访问的菜单树。 */
  menus: MenuNode[]
}
