import type { PlatformBrand } from '@/services/media'
import { create } from 'zustand'
import { fetchPlatformBrand } from '@/services/media'

type BrandState = PlatformBrand & {
  /** 是否已经完成过一次品牌接口加载，成功和失败都会置为 true。 */
  loaded: boolean
  /** 重新请求平台品牌；失败时保留 Store 中已有的安全默认值。 */
  refresh: (signal?: AbortSignal) => Promise<void>
}

/** useBrandStore 管理登录前后共用的平台品牌，并在接口异常时保留默认品牌。 */
export const useBrandStore = create<BrandState>(set => ({
  name: 'ReactGoForge Admin',
  iconUrl: null,
  loaded: false,
  refresh: async (signal) => {
    try {
      const brand = await fetchPlatformBrand(signal)
      set({ ...brand, loaded: true })
    }
    catch {
      if (!signal?.aborted)
        set({ loaded: true })
    }
  },
}))
