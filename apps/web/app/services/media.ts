import type { WorkspaceType } from '@/types/auth'
import { requestAdminJSON, requestJSON } from '@/services/http'

/** 表单保存和图片预览所需的最小图片信息。 */
export interface ImageValue {
  /** 图片资源唯一标识。 */
  id: string
  /** 图片当前展示名称。 */
  originalName: string
  /** 可供浏览器预览的访问地址。 */
  previewUrl: string
}

/** 图片库列表展示和管理所需的完整图片资源。 */
export type ImageAsset = ImageValue & {
  /** 图片所属租户；平台共享图片为 null。 */
  tenantId: string | null
  /** 图片所属租户名称；平台共享图片为 null。 */
  tenantName: string | null
  /** 图片所属分类；未分类时为 null。 */
  categoryId: string | null
  /** 图片分类名称；未分类时为 null。 */
  categoryName: string | null
  /** 图片文件 MIME 类型。 */
  mimeType: string
  /** 图片文件字节数。 */
  sizeBytes: number
  /** 图片上传时间。 */
  createdAt: string
}

/** 图片库的分类选项。 */
export interface ImageCategory {
  /** 分类唯一标识。 */
  id: string
  /** 分类所属租户；平台共享分类为 null。 */
  tenantId: string | null
  /** 分类显示名称。 */
  name: string
  /** 是否为平台侧所有工作空间可见的共享分类。 */
  isShared: boolean
}

/** 图片资源分页查询结果。 */
export interface ImagePage {
  /** 当前页图片资源。 */
  items: ImageAsset[]
  /** 当前页码。 */
  page: number
  /** 每页记录数。 */
  pageSize: number
  /** 符合条件的图片总数。 */
  total: number
}

/** 图片管理筛选器使用的租户选项。 */
export interface TenantOption {
  /** 租户唯一标识。 */
  id: string
  /** 租户显示名称。 */
  name: string
}
/** 登录页和页面标题使用的平台公共品牌。 */
export interface PlatformBrand {
  /** 平台品牌名称。 */
  name: string
  /** 平台品牌图标地址；未配置时为 null。 */
  iconUrl: string | null
}

/** 图片库分页和所有者范围查询参数。 */
interface ImageQuery {
  /** 查询平台共享图片或租户私有图片。 */
  source: 'platform' | 'tenant'
  /** 租户图片来源对应的租户标识。 */
  tenantId?: string
  /** 限定图片分类。 */
  categoryId?: string
  /** 按图片名称模糊搜索。 */
  name?: string
  /** 请求页码。 */
  page?: number
  /** 每页记录数。 */
  pageSize?: number
}

/** 根据工作空间生成图片后台接口基础路径。 */
function adminBase(workspace: WorkspaceType) {
  return `/api/admin/${workspace}`
}

/** 将有实际值的查询参数追加到接口路径。 */
function withQuery<T extends object>(path: string, values: T) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined && value !== '')
      query.set(key, String(value))
  }
  const suffix = query.toString()
  return suffix ? `${path}?${suffix}` : path
}

/** fetchPlatformBrand 读取登录页和页面标题使用的公共平台品牌。 */
export function fetchPlatformBrand(signal?: AbortSignal) {
  return requestJSON<PlatformBrand>('/api/public/platform-brand', { signal })
}

/** fetchImages 按当前工作空间和可信来源分页读取图片。 */
export function fetchImages(workspace: WorkspaceType, query: ImageQuery) {
  return requestAdminJSON<ImagePage>(
    withQuery(`${adminBase(workspace)}/images`, query),
  )
}

/** uploadImage 上传一张图片，并由服务端确定平台或租户所有者范围。 */
export function uploadImage(
  workspace: WorkspaceType,
  file: File,
  options: {
    /** 平台跨租户上传时指定的租户所有者。 */
    tenantId?: string
    /** 上传后直接归属的图片分类。 */
    categoryId?: string
  },
) {
  const data = new FormData()
  data.append('file', file)
  if (options.tenantId)
    data.append('tenantId', options.tenantId)
  if (options.categoryId)
    data.append('categoryId', options.categoryId)
  return requestAdminJSON<ImageAsset>(`${adminBase(workspace)}/images`, {
    method: 'POST',
    data,
  })
}

/** updateImageCategory 修改当前所有者范围内图片的分类。 */
export function updateImageCategory(
  workspace: WorkspaceType,
  imageId: string,
  categoryId: string | null,
  tenantId?: string,
) {
  return requestAdminJSON<null>(
    withQuery(`${adminBase(workspace)}/images/${imageId}`, {
      source: tenantId ? 'tenant' : 'platform',
      tenantId,
    }),
    {
      method: 'PATCH',
      data: { categoryId: categoryId ? Number(categoryId) : null },
    },
  )
}

/** updateImageName 修改当前所有者范围内图片的展示名称。 */
export function updateImageName(
  workspace: WorkspaceType,
  imageId: string,
  originalName: string,
  tenantId?: string,
) {
  return requestAdminJSON<null>(
    withQuery(`${adminBase(workspace)}/images/${imageId}`, {
      source: tenantId ? 'tenant' : 'platform',
      tenantId,
    }),
    {
      method: 'PATCH',
      data: { originalName },
    },
  )
}

/** deleteImage 删除未被品牌设置引用的图片。 */
export function deleteImage(
  workspace: WorkspaceType,
  imageId: string,
  tenantId?: string,
) {
  return requestAdminJSON<null>(
    withQuery(`${adminBase(workspace)}/images/${imageId}`, {
      source: tenantId ? 'tenant' : 'platform',
      tenantId,
    }),
    { method: 'DELETE' },
  )
}

/** fetchImageCategories 读取平台共享或指定租户的分类。 */
export function fetchImageCategories(
  workspace: WorkspaceType,
  options: {
    /** 查询平台共享分类或租户分类。 */
    source: 'platform' | 'tenant'
    /** 租户分类所属租户。 */
    tenantId?: string
  },
) {
  const tenantId = options.source === 'tenant' ? options.tenantId : undefined
  return requestAdminJSON<ImageCategory[]>(
    withQuery(`${adminBase(workspace)}/image-categories`, {
      source: options.source,
      tenantId,
    }),
  )
}

/** createImageCategory 在当前选择的图片所有者下新增分类。 */
export function createImageCategory(
  workspace: WorkspaceType,
  name: string,
  tenantId?: string,
) {
  return requestAdminJSON<ImageCategory>(
    `${adminBase(workspace)}/image-categories`,
    {
      method: 'POST',
      data: { name, tenantId: tenantId ? Number(tenantId) : undefined },
    },
  )
}

/** updateImageCategoryName 修改当前所有者下的分类名称。 */
export function updateImageCategoryName(
  workspace: WorkspaceType,
  categoryId: string,
  name: string,
  tenantId?: string,
) {
  return requestAdminJSON<null>(
    withQuery(`${adminBase(workspace)}/image-categories/${categoryId}`, {
      tenantId,
    }),
    {
      method: 'PATCH',
      data: { name },
    },
  )
}

/** deleteImageCategory 删除分类，分类内图片由后端转入未分类。 */
export function deleteImageCategory(
  workspace: WorkspaceType,
  categoryId: string,
  tenantId?: string,
) {
  return requestAdminJSON<null>(
    withQuery(`${adminBase(workspace)}/image-categories/${categoryId}`, {
      tenantId,
    }),
    { method: 'DELETE' },
  )
}

/** fetchImageTenantOptions 读取平台跨租户图片管理筛选项。 */
export function fetchImageTenantOptions() {
  return requestAdminJSON<TenantOption[]>(
    '/api/admin/platform/images/tenant-options',
  )
}
