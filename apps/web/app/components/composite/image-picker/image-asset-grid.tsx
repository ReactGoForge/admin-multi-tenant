import type { MenuProps } from 'antd'
import type { ImageSource } from './types'
import type { ImageAsset } from '@/services/media'

import type { WorkspaceType } from '@/types/auth'
import {
  CheckCircleFilled,
  EyeOutlined,
  MoreOutlined,
} from '@ant-design/icons'
import { Button, Dropdown, Empty, Image, Pagination, Spin } from 'antd'
import styles from './index.module.scss'

/** 图片资源网格、分页、选择和管理操作配置。 */
interface ImageAssetGridProps {
  /** 当前页图片资源。 */
  assets: ImageAsset[]
  /** 图片列表加载状态。 */
  loading: boolean
  /** 当前读取的平台或租户图片来源。 */
  source: ImageSource
  /** 当前登录工作空间。 */
  workspace: WorkspaceType
  /** 平台跨租户管理时选中的租户 ID。 */
  tenantId?: string
  /** 当前页码。 */
  page: number
  /** 每页图片数量。 */
  pageSize: number
  /** 满足筛选条件的图片总数。 */
  total: number
  /** 页码变化时触发。 */
  onPageChange: (page: number) => void
  /** 是否处于批量管理模式。 */
  bulkMode: boolean
  /** 批量管理模式选中的图片 ID。 */
  managedSelectedIDs: Set<string>
  /** 最终表单选择的图片 ID。 */
  selectedIDs: Set<string>
  /** 多选数量是否已经达到上限。 */
  selectionFull: boolean
  /** 当前图片来源是否允许执行管理操作。 */
  manageable: boolean
  /** 当前图片来源是否允许作为最终选择值。 */
  canSelect: boolean
  /** 是否启用业务图片选择；管理模式仅在批量操作时启用勾选。 */
  selectionEnabled: boolean
  /** 是否展示单张图片管理菜单。 */
  canManageAssets: boolean
  /** 当前打开预览的图片 ID。 */
  previewAssetID?: string
  /** 打开或关闭图片预览时触发。 */
  onPreview: (assetId?: string) => void
  /** 为指定图片生成管理菜单。 */
  onAssetMenu: (asset: ImageAsset) => MenuProps
  /** 切换批量管理选择时触发。 */
  onToggleManaged: (asset: ImageAsset) => void
  /** 切换最终表单选择时触发。 */
  onToggleSelection: (asset: ImageAsset) => void
}

/** ImageAssetGrid 渲染图片列表、选择状态、预览入口和分页。 */
export function ImageAssetGrid({
  assets,
  loading,
  source,
  workspace,
  tenantId,
  page,
  pageSize,
  total,
  onPageChange,
  bulkMode,
  managedSelectedIDs,
  selectedIDs,
  selectionFull,
  manageable,
  canSelect,
  selectionEnabled,
  canManageAssets,
  previewAssetID,
  onPreview,
  onAssetMenu,
  onToggleManaged,
  onToggleSelection,
}: ImageAssetGridProps) {
  return (
    <section className={styles.imagePanel}>
      <div className={styles.imageViewport}>
        <Spin spinning={loading}>
          {assets.length
            ? (
                <div className={styles.grid}>
                  {assets.map((asset) => {
                    const selectable = bulkMode || selectionEnabled
                    const assetSelected = bulkMode
                      ? managedSelectedIDs.has(asset.id)
                      : selectionEnabled && selectedIDs.has(asset.id)
                    const selectionDisabled = bulkMode
                      ? !manageable
                      : !canSelect || (selectionFull && !assetSelected)
                    return (
                      <div
                        className={`${styles.asset} ${assetSelected ? styles.assetSelected : ''} ${!selectionDisabled ? styles.assetSelectable : ''}`}
                        key={asset.id}
                      >
                        {selectable
                          ? (
                              <button
                                aria-label={`${assetSelected ? '取消选择' : '选择图片'}${asset.originalName}`}
                                aria-pressed={assetSelected}
                                className={styles.assetSelect}
                                disabled={selectionDisabled}
                                onClick={() =>
                                  bulkMode
                                    ? onToggleManaged(asset)
                                    : onToggleSelection(asset)}
                                type="button"
                              />
                            )
                          : null}
                        <div className={styles.assetPreview}>
                          <Image
                            alt={asset.originalName}
                            className={styles.assetImage}
                            loading="lazy"
                            preview={{
                              open: previewAssetID === asset.id,
                              onOpenChange: nextOpen =>
                                onPreview(nextOpen ? asset.id : undefined),
                            }}
                            src={asset.previewUrl}
                          />
                        </div>
                        <Button
                          aria-label={`预览图片${asset.originalName}`}
                          className={styles.previewButton}
                          icon={<EyeOutlined />}
                          onClick={() => onPreview(asset.id)}
                          size="small"
                          type="text"
                        />
                        {selectable && assetSelected
                          ? (
                              <CheckCircleFilled className={styles.selectedIcon} />
                            )
                          : null}
                        <span className={styles.assetBody}>
                          <span className={styles.assetInfo}>
                            <span
                              className={styles.assetName}
                              title={asset.originalName}
                            >
                              {asset.originalName}
                            </span>
                            <span className={styles.assetMeta}>
                              {asset.categoryName ?? '未分类'}
                              {asset.tenantName ? ` · ${asset.tenantName}` : ''}
                            </span>
                          </span>
                        </span>
                        {canManageAssets
                          ? (
                              <Dropdown menu={onAssetMenu(asset)} trigger={['click']}>
                                <Button
                                  aria-label={`更多图片操作${asset.originalName}`}
                                  className={styles.assetMore}
                                  icon={<MoreOutlined />}
                                  size="small"
                                  type="text"
                                />
                              </Dropdown>
                            )
                          : null}
                      </div>
                    )
                  })}
                </div>
              )
            : (
                <Empty
                  description={
                    source === 'tenant' && workspace === 'platform' && !tenantId
                      ? '请先选择租户'
                      : '暂无图片'
                  }
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                />
              )}
        </Spin>
      </div>
      <div className={styles.pagination}>
        <Pagination
          current={page}
          hideOnSinglePage
          onChange={onPageChange}
          pageSize={pageSize}
          showSizeChanger={false}
          total={total}
        />
      </div>
    </section>
  )
}
