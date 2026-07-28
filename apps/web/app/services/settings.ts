import type { ImageValue } from '@/services/media'
import { requestAdminJSON } from '@/services/http'

/** 平台或租户基础品牌设置。 */
export interface BasicSettings {
  /** 平台或租户显示名称。 */
  name: string
  /** 从图片库选择的品牌图标；未配置时为 null。 */
  icon: ImageValue | null
}
/** 当前租户可修改的基础设置。 */
export type TenantBasicSettings = BasicSettings
/** 平台超级管理员可修改的基础设置。 */
export type PlatformBasicSettings = BasicSettings
/** 微信小程序配置的非敏感状态。 */
export interface MiniappSettings {
  /** 全平台唯一的微信小程序 AppID。 */
  appId: string
  /** 服务端是否已经安全配置 AppSecret，不返回密文本身。 */
  secretConfigured: boolean
}

/** fetchTenantBasicSettings 读取当前租户基础设置。 */
export function fetchTenantBasicSettings(signal?: AbortSignal) {
  return requestAdminJSON<TenantBasicSettings>(
    '/api/admin/tenant/settings/basic',
    { signal },
  )
}

/** updateTenantBasicSettings 更新当前租户名称和图片库图标。 */
export function updateTenantBasicSettings(input: TenantBasicSettings) {
  return requestAdminJSON<null>('/api/admin/tenant/settings/basic', {
    method: 'PUT',
    data: {
      name: input.name,
      iconImageId: input.icon?.id ? Number(input.icon.id) : null,
    },
  })
}

/** fetchPlatformBasicSettings 读取平台品牌名称和图标。 */
export function fetchPlatformBasicSettings(signal?: AbortSignal) {
  return requestAdminJSON<PlatformBasicSettings>(
    '/api/admin/platform/settings/basic',
    { signal },
  )
}

/** updatePlatformBasicSettings 更新平台品牌名称和图片库图标。 */
export function updatePlatformBasicSettings(input: PlatformBasicSettings) {
  return requestAdminJSON<null>('/api/admin/platform/settings/basic', {
    method: 'PUT',
    data: {
      name: input.name,
      iconImageId: input.icon?.id ? Number(input.icon.id) : null,
    },
  })
}

/** fetchMiniappSettings 读取仅超级管理员可见的微信配置状态。 */
export function fetchMiniappSettings(signal?: AbortSignal) {
  return requestAdminJSON<MiniappSettings>(
    '/api/admin/platform/settings/miniapp',
    { signal },
  )
}

/** updateMiniappSettings 保存全平台唯一微信小程序 AppID。 */
export function updateMiniappSettings(appId: string) {
  return requestAdminJSON<null>('/api/admin/platform/settings/miniapp', {
    method: 'PUT',
    data: { appId },
  })
}
