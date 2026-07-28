import type { EntityStatus } from './rbac'

/** 平台用户与租户之间的加入关系。 */
export interface TenantUserRelation {
  /** 租户用户关系唯一标识。 */
  id: string
  /** 平台用户唯一标识。 */
  userId: string
  /** 关联租户唯一标识。 */
  tenantId: string
  /** 用户加入租户的来源方式。 */
  source: 'scan' | 'invite' | 'created'
  /** 当前租户关系是否可用。 */
  status: EntityStatus
}
