import type { ImageValue } from '@/services/media'

/** 批量图片操作最大并发数，避免同时发起过多请求。 */
const bulkConcurrency = 3

/** 批量图片操作结果，失败项保留给用户继续处理。 */
export interface BulkResult {
  /** 操作成功的图片 ID，用于从最终选择中移除已删除项。 */
  succeededIDs: Set<string>
  /** 操作失败的图片摘要，保留在批量选择中供用户继续处理。 */
  failedItems: ImageValue[]
}

/** runBulkOperation 以固定并发数执行批量图片操作并汇总部分失败。 */
export async function runBulkOperation(
  items: ImageValue[],
  operation: (item: ImageValue) => Promise<unknown>,
): Promise<BulkResult> {
  let nextIndex = 0
  const succeededIDs = new Set<string>()
  const failedItems: ImageValue[] = []
  /** worker 从共享索引中持续领取图片，组成固定数量的并发执行器。 */
  const worker = async () => {
    while (nextIndex < items.length) {
      const item = items[nextIndex]
      nextIndex += 1
      try {
        await operation(item)
        succeededIDs.add(item.id)
      }
      catch {
        failedItems.push(item)
      }
    }
  }
  await Promise.all(
    Array.from({ length: Math.min(bulkConcurrency, items.length) }, () =>
      worker()),
  )
  return { succeededIDs, failedItems }
}
