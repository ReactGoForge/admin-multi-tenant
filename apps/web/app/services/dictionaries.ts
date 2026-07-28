import type { EntityStatus } from '@/types/rbac'
import { requestAdminJSON } from '@/services/http'

/** 下拉框和标签展示使用的启用字典项。 */
export interface DictionaryOption {
  /** 面向用户展示的字典项名称。 */
  label: string
  /** 业务数据实际保存的稳定值。 */
  value: string
  /** 同一字典内的显示顺序。 */
  sort: number
}

/** 按字典编码索引的可选项集合。 */
export type DictionaryOptionMap = Record<string, DictionaryOption[]>

/** 字典管理页面使用的完整字典项。 */
export type DictionaryItem = DictionaryOption & {
  /** 字典项唯一标识。 */
  id: string
  /** 字典项当前是否可供业务选择。 */
  status: EntityStatus
}

/** 字典管理页面使用的字典类型及其明细项。 */
export interface DictionaryType {
  /** 字典类型唯一标识。 */
  id: string
  /** 程序读取字典时使用的稳定编码。 */
  code: string
  /** 字典类型显示名称。 */
  name: string
  /** 字典用途备注；未填写时为 null。 */
  remark: string | null
  /** 字典类型在管理页面的显示顺序。 */
  sort: number
  /** 字典类型当前是否启用。 */
  status: EntityStatus
  /** 是否为系统内置字典，内置字典受到删除等操作限制。 */
  isSystem: boolean
  /** 当前字典包含的全部管理项。 */
  items: DictionaryItem[]
}

/** 新增或修改字典类型时提交的字段。 */
export interface DictionaryTypeMutation {
  /** 字典稳定编码。 */
  code: string
  /** 字典显示名称。 */
  name: string
  /** 可选的业务用途备注。 */
  remark?: string
  /** 字典类型显示顺序。 */
  sort: number
  /** 字典类型启停状态。 */
  status: EntityStatus
}

/** 新增或修改字典项时提交的字段。 */
export interface DictionaryItemMutation {
  /** 字典项展示名称。 */
  label: string
  /** 字典项稳定业务值。 */
  value: string
  /** 字典项显示顺序。 */
  sort: number
  /** 字典项启停状态。 */
  status: EntityStatus
}

/** fetchDictionaryOptions 读取当前后台账号可使用的全局启用字典。 */
export function fetchDictionaryOptions(signal?: AbortSignal) {
  return requestAdminJSON<DictionaryOptionMap>(
    '/api/admin/dictionary-options',
    {
      signal,
    },
  )
}

/** fetchPlatformDictionaries 读取字典管理所需的完整字段和字典项。 */
export function fetchPlatformDictionaries(signal?: AbortSignal) {
  return requestAdminJSON<DictionaryType[]>(
    '/api/admin/platform/dictionaries',
    {
      signal,
    },
  )
}

/** createPlatformDictionary 新增自定义字典字段。 */
export function createPlatformDictionary(input: DictionaryTypeMutation) {
  return requestAdminJSON<null>('/api/admin/platform/dictionaries', {
    method: 'POST',
    data: input,
  })
}

/** updatePlatformDictionary 更新指定字典字段。 */
export function updatePlatformDictionary(
  dictionaryId: string,
  input: DictionaryTypeMutation,
) {
  return requestAdminJSON<null>(
    `/api/admin/platform/dictionaries/${dictionaryId}`,
    { method: 'PATCH', data: input },
  )
}

/** deletePlatformDictionary 删除无字典项的自定义字典字段。 */
export function deletePlatformDictionary(dictionaryId: string) {
  return requestAdminJSON<null>(
    `/api/admin/platform/dictionaries/${dictionaryId}`,
    { method: 'DELETE' },
  )
}

/** createPlatformDictionaryItem 为自定义字典字段新增字典项。 */
export function createPlatformDictionaryItem(
  dictionaryId: string,
  input: DictionaryItemMutation,
) {
  return requestAdminJSON<null>(
    `/api/admin/platform/dictionaries/${dictionaryId}/items`,
    { method: 'POST', data: input },
  )
}

/** updatePlatformDictionaryItem 更新指定字典项。 */
export function updatePlatformDictionaryItem(
  dictionaryId: string,
  itemId: string,
  input: DictionaryItemMutation,
) {
  return requestAdminJSON<null>(
    `/api/admin/platform/dictionaries/${dictionaryId}/items/${itemId}`,
    { method: 'PATCH', data: input },
  )
}

/** deletePlatformDictionaryItem 删除自定义字典字段下的字典项。 */
export function deletePlatformDictionaryItem(
  dictionaryId: string,
  itemId: string,
) {
  return requestAdminJSON<null>(
    `/api/admin/platform/dictionaries/${dictionaryId}/items/${itemId}`,
    { method: 'DELETE' },
  )
}
