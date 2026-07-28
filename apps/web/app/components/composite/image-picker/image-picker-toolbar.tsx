import type { UploadProps } from 'antd'
import type { ImageSource } from './types'
import type { TenantOption } from '@/services/media'

import type { WorkspaceType } from '@/types/auth'
import {
  DeleteOutlined,
  FolderOutlined,
  UploadOutlined,
} from '@ant-design/icons'
import { Button, Input, Segmented, Space, Typography, Upload } from 'antd'
import { TenantSelect } from '@/components/base/selects'
import styles from './index.module.scss'

/** 图片选择器工具栏配置。 */
interface ImagePickerToolbarProps {
  /** 当前登录工作空间。 */
  workspace: WorkspaceType
  /** 当前平台或租户图片来源。 */
  source: ImageSource
  /** 工具栏可切换的图片来源选项。 */
  sourceOptions: Array<{
    /** 图片来源展示文案。 */
    label: string
    /** 图片来源稳定值。 */
    value: ImageSource
  }>
  /** 图片来源变化时触发。 */
  onSourceChange: (source: ImageSource) => void
  /** 平台跨租户图片管理使用的租户选项。 */
  tenants: TenantOption[]
  /** 当前选中的租户 ID。 */
  tenantId?: string
  /** 平台目标租户变化时触发。 */
  onTenantChange: (tenantId?: string) => void
  /** 用户提交文件名搜索时触发。 */
  onSearch: (name: string) => void
  /** 是否允许上传图片。 */
  canUpload: boolean
  /** 图片上传进行中状态。 */
  uploading: boolean
  /** Ant Design Upload 选择文件前的自定义处理。 */
  onBeforeUpload: UploadProps['beforeUpload']
  /** 是否展示批量管理入口。 */
  canBulkManage: boolean
  /** 是否已经进入批量管理模式。 */
  bulkMode: boolean
  /** 批量管理当前选中的图片数量。 */
  selectedCount: number
  /** 批量操作执行中状态。 */
  bulkOperating: boolean
  /** 是否允许批量修改分类。 */
  canBatchCategory: boolean
  /** 是否允许批量删除。 */
  canBatchDelete: boolean
  /** 点击批量分类时触发。 */
  onBatchCategory: () => void
  /** 点击批量删除时触发。 */
  onBatchDelete: () => void
  /** 进入批量管理模式时触发。 */
  onEnterBulk: () => void
  /** 退出批量管理模式时触发。 */
  onExitBulk: () => void
}

/** ImagePickerToolbar 渲染图片来源、搜索、上传和批量操作入口。 */
export function ImagePickerToolbar({
  workspace,
  source,
  sourceOptions,
  onSourceChange,
  tenants,
  tenantId,
  onTenantChange,
  onSearch,
  canUpload,
  uploading,
  onBeforeUpload,
  canBulkManage,
  bulkMode,
  selectedCount,
  bulkOperating,
  canBatchCategory,
  canBatchDelete,
  onBatchCategory,
  onBatchDelete,
  onEnterBulk,
  onExitBulk,
}: ImagePickerToolbarProps) {
  return (
    <div className={styles.toolbar}>
      <Segmented
        options={sourceOptions}
        value={source}
        onChange={next => onSourceChange(next as ImageSource)}
      />
      {workspace === 'platform' && source === 'tenant'
        ? (
            <TenantSelect
              className="min-w-48"
              onChange={onTenantChange}
              options={tenants}
              placeholder="请选择租户"
              style={{ width: 192 }}
              value={tenantId}
            />
          )
        : null}
      <Input.Search
        allowClear
        className="max-w-56"
        onSearch={next => onSearch(next.trim())}
        placeholder="按文件名搜索"
      />
      {canUpload
        ? (
            <Upload
              accept="image/png,image/jpeg,image/webp"
              beforeUpload={onBeforeUpload}
              disabled={uploading}
              multiple
              showUploadList={false}
            >
              <Button icon={<UploadOutlined />} loading={uploading}>
                上传图片
              </Button>
            </Upload>
          )
        : null}
      {canBulkManage
        ? (
            <div className={styles.bulkActions}>
              {bulkMode
                ? (
                    <Space size={8}>
                      <Typography.Text type="secondary">
                        已选
                        {' '}
                        {selectedCount}
                        {' '}
                        张
                      </Typography.Text>
                      {canBatchCategory
                        ? (
                            <Button
                              disabled={!selectedCount || bulkOperating}
                              icon={<FolderOutlined />}
                              onClick={onBatchCategory}
                            >
                              批量分类
                            </Button>
                          )
                        : null}
                      {canBatchDelete
                        ? (
                            <Button
                              danger
                              disabled={!selectedCount || bulkOperating}
                              icon={<DeleteOutlined />}
                              onClick={onBatchDelete}
                            >
                              批量删除
                            </Button>
                          )
                        : null}
                      <Button disabled={bulkOperating} onClick={onExitBulk}>
                        退出批量
                      </Button>
                    </Space>
                  )
                : (
                    <Button onClick={onEnterBulk}>批量管理</Button>
                  )}
            </div>
          )
        : null}
    </div>
  )
}
