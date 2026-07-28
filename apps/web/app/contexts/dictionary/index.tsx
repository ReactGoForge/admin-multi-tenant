import type { ReactNode } from 'react'
import type { DictionaryOption, DictionaryOptionMap } from '@/services/dictionaries'

import {
  createContext,

  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react'
import {

  fetchDictionaryOptions,
} from '@/services/dictionaries'
import { isSilentRequestError } from '@/services/http'
import { useAuthStore } from '@/stores/auth'

/** 前端业务使用的稳定字典编码，值与后端字典定义保持一致。 */
export const DICTIONARY_CODE = {
  entityStatus: 'entity_status',
  menuNodeType: 'menu_node_type',
  roleType: 'role_type',
  workspaceScope: 'workspace_scope',
} as const

/** 字典接口不可用或尚未登录时使用的最小安全选项。 */
const fallbackOptions: DictionaryOptionMap = {
  [DICTIONARY_CODE.entityStatus]: [
    { label: '启用', value: 'enabled', sort: 10 },
    { label: '禁用', value: 'disabled', sort: 20 },
  ],
  [DICTIONARY_CODE.roleType]: [
    { label: '内置角色', value: 'system', sort: 10 },
    { label: '自定义角色', value: 'custom', sort: 20 },
  ],
  [DICTIONARY_CODE.menuNodeType]: [
    { label: '目录', value: 'directory', sort: 10 },
    { label: '菜单', value: 'menu', sort: 20 },
    { label: '操作权限', value: 'permission', sort: 30 },
  ],
  [DICTIONARY_CODE.workspaceScope]: [
    { label: '平台端', value: 'platform', sort: 10 },
    { label: '租户端', value: 'tenant', sort: 20 },
  ],
}

interface DictionaryContextValue {
  /** 根据字典编码和值读取展示文案，未匹配时返回传入的兜底文案。 */
  getLabel: (code: string, value: string, fallback?: string) => string
  /** 根据字典编码读取排序后的可选项；不存在时返回空数组。 */
  getOptions: (code: string) => DictionaryOption[]
  /** 在已登录状态下重新请求全部字典选项。 */
  refresh: () => Promise<void>
}

const DictionaryContext = createContext<DictionaryContextValue | null>(null)

/** DictionaryProvider 登录后统一加载字典，并在接口异常时保留内置安全选项。 */
export function DictionaryProvider({
  children,
}: {
  /** 需要共享字典查询能力的应用子树。 */
  children: ReactNode
}) {
  const isAuthenticated = useAuthStore(state => state.isAuthenticated)

  // 字典数据状态：始终以本地安全选项为基础，登录后再用服务端配置覆盖同名编码。
  const [options, setOptions] = useState<DictionaryOptionMap>(fallbackOptions)

  /**
   * 刷新全局字典选项。
   * 未登录时直接恢复本地选项；请求失败时仅处理普通错误，主动取消等静默错误保持当前数据。
   */
  const refresh = useCallback(async () => {
    if (!useAuthStore.getState().isAuthenticated) {
      setOptions(fallbackOptions)
      return
    }
    try {
      const result = await fetchDictionaryOptions()
      setOptions({ ...fallbackOptions, ...result })
    }
    catch (error) {
      if (!isSilentRequestError(error))
        setOptions(fallbackOptions)
    }
  }, [])

  // 登录状态变化时同步字典来源：登录后请求服务端，退出后立即清除会话字典并恢复默认值。
  useEffect(() => {
    if (isAuthenticated)
      void refresh()
    else setOptions(fallbackOptions)
  }, [isAuthenticated, refresh])

  // 将当前字典映射封装为稳定的查询方法，避免消费组件了解字典存储结构。
  const value = useMemo<DictionaryContextValue>(
    () => ({
      getLabel: (code, itemValue, fallback = itemValue) =>
        options[code]?.find(item => item.value === itemValue)?.label
        ?? fallback,
      getOptions: code => options[code] ?? [],
      refresh,
    }),
    [options, refresh],
  )

  return (
    <DictionaryContext.Provider value={value}>
      {children}
    </DictionaryContext.Provider>
  )
}

/** useDictionary 读取全局字典选项、文案和刷新能力。 */
export function useDictionary() {
  const context = useContext(DictionaryContext)
  if (!context)
    throw new Error('useDictionary 必须在 DictionaryProvider 内使用')
  return context
}
