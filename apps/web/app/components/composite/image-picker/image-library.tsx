import type { MenuProps, UploadProps } from 'antd'
import type { ReactElement } from 'react'
import type { BulkResult } from './bulk-operation'
import type { ImageLibraryProps, ImageSource } from './types'
import type { ImageAsset, ImageCategory, ImageValue, TenantOption } from '@/services/media'
import {
  DeleteOutlined,
  EditOutlined,
  FolderOutlined,
} from '@ant-design/icons'
import { App, Input, Select, Upload } from 'antd'
import {

  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { getErrorMessage } from '@/services/errors'
import {
  createImageCategory,
  deleteImage,
  deleteImageCategory,
  fetchImageCategories,
  fetchImages,
  fetchImageTenantOptions,

  updateImageCategory,
  updateImageCategoryName,
  updateImageName,
  uploadImage,
} from '@/services/media'
import { useAuthStore } from '@/stores/auth'
import { runBulkOperation } from './bulk-operation'
import { ImageAssetGrid } from './image-asset-grid'
import { ImageCategoryTree } from './image-category-tree'
import { ImagePickerToolbar } from './image-picker-toolbar'
import styles from './index.module.scss'

/** 图片库每页固定展示数量，与服务端分页请求保持一致。 */
const imagePageSize = 10

/** ImageLibrary 统一承载图片浏览、选择、上传、分类和批量管理能力。 */
export function ImageLibrary(props: ImageLibraryProps): ReactElement {
  // 组件模式和页面能力：workspace 决定接口范围，select 模式允许选择当前可见来源。
  const { workspace } = props
  const active = props.active ?? true
  const { message, modal } = App.useApp()
  const currentUser = useAuthStore(state => state.currentUser)

  // 图片库弹窗和数据来源状态：
  // - source/tenantId：控制平台或租户图库来源及平台跨租户目标。
  // - tenants/categories/categoryId：租户筛选选项、当前来源分类及所选分类。
  // - name/assets/imagePage/imageTotal/imageListLoading：图片搜索词、当前页资源、分页和列表加载状态。
  const [source, setSource] = useState<ImageSource>(
    workspace === 'tenant' ? 'tenant' : 'platform',
  )
  const [tenantId, setTenantId] = useState<string>()
  const [tenants, setTenants] = useState<TenantOption[]>([])
  const [categoryId, setCategoryId] = useState<string>()
  const [categories, setCategories] = useState<ImageCategory[]>([])
  const [name, setName] = useState('')
  const [assets, setAssets] = useState<ImageAsset[]>([])
  const [imagePage, setImagePage] = useState(1)
  const [imageTotal, setImageTotal] = useState(0)
  const [imageListLoading, setImageListLoading] = useState(false)

  // 选择、批量管理和预览状态：
  // - managedSelected/bulkMode/bulkOperating：独立维护批量操作选择和执行状态。
  // - previewAssetID：当前使用 Ant Design Image 预览的图片 ID。
  // - uploading/uploadingRef：展示上传状态并同步阻止重复触发同一批文件。
  const [bulkMode, setBulkMode] = useState(false)
  const [managedSelected, setManagedSelected] = useState<ImageValue[]>([])
  const [bulkOperating, setBulkOperating] = useState(false)
  const [previewAssetID, setPreviewAssetID] = useState<string>()
  const [uploading, setUploading] = useState(false)
  const uploadingRef = useRef(false)
  /** changeBulkMode 同步图库内部批量状态，并通知选择弹窗调整确认按钮。 */
  const changeBulkMode = (nextBulkMode: boolean) => {
    setBulkMode(nextBulkMode)
    if (props.mode === 'select')
      props.onBulkModeChange?.(nextBulkMode)
  }

  // 根据当前登录权限、工作空间和图片来源派生查看、选择、上传及管理能力。
  const permissionPrefix = workspace === 'platform' ? 'platform' : 'tenant'
  const hasPermission = useCallback(
    (code: string) =>
      Boolean(
        currentUser?.isSuperAdmin
        || currentUser?.permissions.includes(
          `${permissionPrefix}:image:${code}`,
        ),
      ),
    [currentUser, permissionPrefix],
  )
  const selected = props.mode === 'select' ? props.value : []
  const selectionRef = useRef(selected)
  const selectionChangeRef = useRef<
    ((value: ImageValue[]) => void) | undefined
  >(props.mode === 'select' ? props.onChange : undefined)
  selectionRef.current = selected
  selectionChangeRef.current
    = props.mode === 'select' ? props.onChange : undefined
  const setSelected = useCallback(
    (updater: (current: ImageValue[]) => ImageValue[]) => {
      const nextSelection = updater(selectionRef.current)
      selectionRef.current = nextSelection
      selectionChangeRef.current?.(nextSelection)
    },
    [],
  )
  const multiple = props.mode === 'select' && props.multiple === true
  const selectionLimit
    = props.mode === 'select' && props.multiple && props.maxCount !== undefined
      ? Math.max(0, Math.floor(props.maxCount))
      : undefined
  const ownSource = workspace === 'platform' || source === 'tenant'
  const manageable
    = ownSource
      && (source === 'platform' || Boolean(tenantId) || workspace === 'tenant')
  const canSelect = props.mode === 'select'
  const ownerTenantId
    = source === 'tenant' && workspace === 'platform' ? tenantId : undefined
  const tenantSharedSource = workspace === 'tenant' && source === 'platform'
  const selectedTreeKey
    = categoryId === undefined
      ? 'all'
      : categoryId === '0'
        ? 'uncategorized'
        : `category:${categoryId}`

  /** loadCategories 加载当前图片来源的分类，并在租户读取平台图库时定位共享分类。 */
  const loadCategories = useCallback(async () => {
    if (source === 'tenant' && workspace === 'platform' && !tenantId) {
      setCategories([])
      return
    }
    try {
      const nextCategories = await fetchImageCategories(workspace, {
        source,
        tenantId,
      })
      setCategories(nextCategories)
      if (tenantSharedSource) {
        setCategoryId(nextCategories.find(item => item.isShared)?.id)
      }
    }
    catch (error) {
      void message.error(getErrorMessage(error, '图片分类加载失败'))
    }
  }, [message, source, tenantId, tenantSharedSource, workspace])

  /**
   * loadImages 按当前来源、租户、分类、名称和页码加载图片。
   * 结果页越界时回退最后一页，并同步刷新选择项中的图片名称和预览地址。
   */
  const loadImages = useCallback(async () => {
    if (
      !active
      || (source === 'tenant' && workspace === 'platform' && !tenantId)
    ) {
      setAssets([])
      setImageTotal(0)
      return
    }
    setImageListLoading(true)
    try {
      const result = await fetchImages(workspace, {
        source,
        tenantId,
        categoryId,
        name,
        page: imagePage,
        pageSize: imagePageSize,
      })
      const lastPage = Math.max(1, Math.ceil(result.total / imagePageSize))
      if (imagePage > lastPage) {
        setImagePage(lastPage)
        return
      }
      setAssets(result.items)
      const refreshedValues = new Map(
        result.items.map(item => [item.id, item] as const),
      )
      setSelected(current =>
        current.map((item) => {
          const refreshed = refreshedValues.get(item.id)
          return refreshed
            ? {
                id: refreshed.id,
                originalName: refreshed.originalName,
                previewUrl: refreshed.previewUrl,
              }
            : item
        }),
      )
      setManagedSelected(current =>
        current.map((item) => {
          const refreshed = refreshedValues.get(item.id)
          return refreshed
            ? {
                id: refreshed.id,
                originalName: refreshed.originalName,
                previewUrl: refreshed.previewUrl,
              }
            : item
        }),
      )
      setImageTotal(result.total)
    }
    catch (error) {
      void message.error(getErrorMessage(error, '图片加载失败'))
    }
    finally {
      setImageListLoading(false)
    }
  }, [
    active,
    categoryId,
    imagePage,
    message,
    name,
    setSelected,
    source,
    tenantId,
    workspace,
  ])

  // 平台打开图库时加载租户选项；切换到租户图库且未选租户时默认第一个租户。
  useEffect(() => {
    if (!active || workspace !== 'platform')
      return
    void fetchImageTenantOptions()
      .then((items) => {
        setTenants(items)
        if (source === 'tenant' && !tenantId)
          setTenantId(items[0]?.id)
      })
      .catch(
        error =>
          void message.error(getErrorMessage(error, '租户选项加载失败')),
      )
  }, [active, message, source, tenantId, workspace])

  // 弹窗打开或图片来源变化后刷新分类。
  useEffect(() => {
    if (!active)
      return
    void loadCategories()
  }, [active, loadCategories])

  // loadImages 依赖中的筛选或分页状态变化时刷新图片列表。
  useEffect(() => {
    void loadImages()
  }, [loadImages])

  // 平台和租户工作空间使用不同顺序和文案展示图片来源。
  const sourceOptions = useMemo<
    Array<{
      /** 图片来源展示文案。 */
      label: string
      /** 图片来源稳定值。 */
      value: ImageSource
    }>
  >(
    () =>
      workspace === 'platform'
        ? [
            { label: '平台图库', value: 'platform' },
            { label: '租户图库', value: 'tenant' },
          ]
        : [
            { label: '本租户', value: 'tenant' },
            { label: '平台图库', value: 'platform' },
          ],
    [workspace],
  )

  /** uploadFiles 最多并发上传三张图片，完成后刷新列表和分类并汇总失败文件。 */
  const uploadFiles = async (files: File[]) => {
    if (!files.length || uploadingRef.current)
      return
    uploadingRef.current = true
    setUploading(true)
    let nextIndex = 0
    let successCount = 0
    const failedNames: string[] = []
    /** worker 从共享索引中持续领取文件，实现不超过三个并发上传任务。 */
    const worker = async () => {
      while (nextIndex < files.length) {
        const file = files[nextIndex]
        nextIndex += 1
        try {
          await uploadImage(workspace, file, {
            tenantId: ownerTenantId,
            categoryId: categoryId === '0' ? undefined : categoryId,
          })
          successCount += 1
        }
        catch {
          failedNames.push(file.name)
        }
      }
    }
    try {
      await Promise.all(
        Array.from({ length: Math.min(3, files.length) }, () => worker()),
      )
      await Promise.all([loadImages(), loadCategories()])
      if (!failedNames.length) {
        void message.success(`已上传 ${successCount} 张图片`)
      }
      else {
        const names = failedNames.slice(0, 3).join('、')
        const suffix
          = failedNames.length > 3 ? `等 ${failedNames.length} 个文件` : ''
        const detail = `${names}${suffix}`
        if (successCount) {
          void message.warning(
            `成功 ${successCount} 张，失败 ${failedNames.length} 张：${detail}`,
          )
        }
        else {
          void message.error(`图片上传失败：${detail}`)
        }
      }
    }
    finally {
      uploadingRef.current = false
      setUploading(false)
    }
  }

  /** handleBeforeUpload 拦截 Ant Design 默认上传，只由首个文件触发整批自定义上传。 */
  const handleBeforeUpload: UploadProps['beforeUpload'] = (file, fileList) => {
    if (file.uid === fileList[0]?.uid)
      void uploadFiles(fileList)
    return Upload.LIST_IGNORE
  }

  /** handleDelete 二次确认删除单张图片，并同步清理临时选择和刷新列表。 */
  const handleDelete = (asset: ImageAsset) => {
    modal.confirm({
      title: '删除图片',
      content: `确定删除“${asset.originalName}”吗？被品牌设置引用的图片无法删除。`,
      okText: '删除',
      okButtonProps: { danger: true },
      onOk: async () => {
        await deleteImage(workspace, asset.id, ownerTenantId)
        setSelected(current =>
          current.filter(item => item.id !== asset.id),
        )
        void message.success('图片已删除')
        await loadImages()
      },
    })
  }

  /** finishBulkOperation 保留失败项供用户重试，刷新列表并汇总批量操作结果。 */
  const finishBulkOperation = async (
    result: BulkResult,
    successLabel: string,
    removeFromSelection: boolean,
  ) => {
    setManagedSelected(result.failedItems)
    if (removeFromSelection) {
      setSelected(current =>
        current.filter(item => !result.succeededIDs.has(item.id)),
      )
    }
    await loadImages()
    const successCount = result.succeededIDs.size
    if (!result.failedItems.length) {
      void message.success(`${successLabel} ${successCount} 张图片`)
      return
    }
    const failedNames = result.failedItems
      .slice(0, 3)
      .map(item => item.originalName)
      .join('、')
    const suffix
      = result.failedItems.length > 3 ? `等 ${result.failedItems.length} 张` : ''
    void message.warning(
      `成功 ${successCount} 张，失败 ${result.failedItems.length} 张：${failedNames}${suffix}`,
    )
  }

  /** handleBatchCategory 选择目标分类后并发更新已勾选图片，并保留失败项。 */
  const handleBatchCategory = () => {
    let nextCategoryId: string | null = null
    modal.confirm({
      title: `批量分类（已选 ${managedSelected.length} 张）`,
      icon: <FolderOutlined />,
      content: (
        <Select
          allowClear
          className="mt-3 w-full"
          options={categories.map(item => ({
            label: item.name,
            value: item.id,
          }))}
          onChange={(next) => {
            nextCategoryId = next ?? null
          }}
          placeholder="未分类"
        />
      ),
      okText: '确认分类',
      onOk: async () => {
        setBulkOperating(true)
        try {
          const result = await runBulkOperation(managedSelected, item =>
            updateImageCategory(
              workspace,
              item.id,
              nextCategoryId,
              ownerTenantId,
            ))
          await finishBulkOperation(result, '已分类', false)
        }
        finally {
          setBulkOperating(false)
        }
      },
    })
  }

  /** handleBatchDelete 确认后批量删除图片，被品牌引用等失败项继续保留。 */
  const handleBatchDelete = () => {
    modal.confirm({
      title: `批量删除（已选 ${managedSelected.length} 张）`,
      content: '确定删除这些图片吗？被品牌设置引用的图片会保留并显示为失败。',
      okButtonProps: { danger: true },
      okText: '确认删除',
      onOk: async () => {
        setBulkOperating(true)
        try {
          const result = await runBulkOperation(managedSelected, item =>
            deleteImage(workspace, item.id, ownerTenantId))
          await finishBulkOperation(result, '已删除', true)
        }
        finally {
          setBulkOperating(false)
        }
      },
    })
  }

  /** handleRename 校验新名称后更新图片，并同步当前临时选择中的展示名称。 */
  const handleRename = (asset: ImageAsset) => {
    let nextName = asset.originalName
    modal.confirm({
      title: '重命名图片',
      content: (
        <Input
          defaultValue={asset.originalName}
          maxLength={255}
          onChange={(event) => {
            nextName = event.target.value
          }}
          placeholder="请输入图片名称"
        />
      ),
      okText: '保存',
      onOk: async () => {
        const normalizedName = nextName.trim()
        if (!normalizedName || Array.from(normalizedName).length > 255) {
          void message.warning('图片名称长度应为 1 至 255 个字符')
          throw new Error('图片名称不合法')
        }
        await updateImageName(
          workspace,
          asset.id,
          normalizedName,
          ownerTenantId,
        )
        setSelected(current =>
          current.map(item =>
            item.id === asset.id
              ? { ...item, originalName: normalizedName }
              : item,
          ),
        )
        void message.success('图片名称已更新')
        await loadImages()
      },
    })
  }

  /** assetMenu 根据来源和权限生成图片重命名、分类及删除菜单。 */
  const assetMenu = (asset: ImageAsset): MenuProps => ({
    items: [
      {
        key: 'rename',
        label: '重命名',
        icon: <EditOutlined />,
        disabled: !manageable || !hasPermission('edit'),
      },
      {
        key: 'category',
        label: '修改分类',
        icon: <FolderOutlined />,
        disabled: !manageable || !hasPermission('edit'),
      },
      {
        key: 'delete',
        label: '删除图片',
        icon: <DeleteOutlined />,
        danger: true,
        disabled: !manageable || !hasPermission('delete'),
      },
    ],
    onClick: ({ key }) => {
      if (key === 'rename')
        handleRename(asset)
      if (key === 'delete')
        handleDelete(asset)
      if (key === 'category') {
        let nextCategoryId = asset.categoryId
        modal.confirm({
          title: '修改图片分类',
          icon: <EditOutlined />,
          content: (
            <Select
              allowClear
              className="mt-3 w-full"
              defaultValue={asset.categoryId ?? undefined}
              options={categories.map(item => ({
                label: item.name,
                value: item.id,
              }))}
              onChange={(next) => {
                nextCategoryId = next ?? null
              }}
              placeholder="未分类"
            />
          ),
          onOk: async () => {
            await updateImageCategory(
              workspace,
              asset.id,
              nextCategoryId,
              ownerTenantId,
            )
            void message.success('图片分类已更新')
            await loadImages()
          },
        })
      }
    },
  })

  /** handleCreateCategory 校验分类名称后在当前图片所有者范围新增分类。 */
  const handleCreateCategory = () => {
    let nextName = ''
    modal.confirm({
      title: '新增分类',
      content: (
        <Input
          maxLength={40}
          onChange={(event) => {
            nextName = event.target.value
          }}
          placeholder="输入分类名称"
        />
      ),
      okText: '新增',
      onOk: async () => {
        if (!nextName.trim())
          throw new Error('请输入分类名称')
        await createImageCategory(workspace, nextName.trim(), ownerTenantId)
        await loadCategories()
        void message.success('分类已新增')
      },
    })
  }

  /** handleRenameCategory 修改当前所有者范围内的分类名称并刷新分类树。 */
  const handleRenameCategory = (category: ImageCategory) => {
    let nextName = category.name
    modal.confirm({
      title: '重命名分类',
      content: (
        <Input
          defaultValue={category.name}
          maxLength={40}
          onChange={(event) => {
            nextName = event.target.value
          }}
        />
      ),
      onOk: async () => {
        if (!nextName.trim())
          throw new Error('请输入分类名称')
        await updateImageCategoryName(
          workspace,
          category.id,
          nextName.trim(),
          ownerTenantId,
        )
        await loadCategories()
        void message.success('分类已重命名')
      },
    })
  }

  /** handleDeleteCategory 删除分类；当前分类被删除时切回全部图片并刷新列表。 */
  const handleDeleteCategory = (category: ImageCategory) => {
    modal.confirm({
      title: '删除分类',
      content: '分类内图片将自动转入未分类。',
      okButtonProps: { danger: true },
      okText: '删除',
      onOk: async () => {
        const deletingCurrent = categoryId === category.id
        await deleteImageCategory(workspace, category.id, ownerTenantId)
        if (deletingCurrent)
          setCategoryId(undefined)
        await loadCategories()
        if (!deletingCurrent)
          await loadImages()
        void message.success('分类已删除')
      },
    })
  }

  /** categoryMenu 生成分类重命名和删除操作菜单。 */
  const categoryMenu = (category: ImageCategory): MenuProps => ({
    items: [
      { key: 'rename', label: '重命名', icon: <EditOutlined /> },
      {
        key: 'delete',
        label: '删除分类',
        icon: <DeleteOutlined />,
        danger: true,
      },
    ],
    onClick: ({ key }) => {
      if (key === 'rename')
        handleRenameCategory(category)
      if (key === 'delete')
        handleDeleteCategory(category)
    },
  })

  // 将选择数组转换为 ID 集合，供图片网格快速判断选中和数量上限状态。
  const selectedIDs = new Set(selected.map(item => item.id))
  const managedSelectedIDs = new Set(managedSelected.map(item => item.id))
  const selectionFull
    = multiple
      && selectionLimit !== undefined
      && selected.length >= selectionLimit
  /** toggleSelection 按单选、多选和最大数量规则切换准备提交的图片。 */
  const toggleSelection = (asset: ImageAsset) => {
    if (!canSelect)
      return
    if (selectedIDs.has(asset.id)) {
      setSelected(current => current.filter(item => item.id !== asset.id))
      return
    }
    if (selectionFull) {
      void message.warning(`最多选择 ${selectionLimit} 张图片`)
      return
    }
    const nextValue: ImageValue = {
      id: asset.id,
      originalName: asset.originalName,
      previewUrl: asset.previewUrl,
    }
    setSelected(current =>
      multiple ? [...current, nextValue] : [nextValue],
    )
  }
  /** toggleManagedSelection 独立切换批量管理选择，不影响最终表单选择值。 */
  const toggleManagedSelection = (asset: ImageAsset) => {
    if (managedSelectedIDs.has(asset.id)) {
      setManagedSelected(current =>
        current.filter(item => item.id !== asset.id),
      )
      return
    }
    setManagedSelected(current => [
      ...current,
      {
        id: asset.id,
        originalName: asset.originalName,
        previewUrl: asset.previewUrl,
      },
    ])
  }

  // 分类和批量操作能力同时受图片所有者范围及具体编辑、删除权限约束。
  const canManageCategories = manageable && hasPermission('edit')
  const canBulkManage
    = manageable && (hasPermission('edit') || hasPermission('delete'))

  return (
    <div>
      <ImagePickerToolbar
        bulkMode={bulkMode}
        bulkOperating={bulkOperating}
        canBatchCategory={hasPermission('edit')}
        canBatchDelete={hasPermission('delete')}
        canBulkManage={canBulkManage}
        canUpload={manageable && hasPermission('upload')}
        onBatchCategory={handleBatchCategory}
        onBatchDelete={handleBatchDelete}
        onBeforeUpload={handleBeforeUpload}
        onEnterBulk={() => changeBulkMode(true)}
        onExitBulk={() => {
          changeBulkMode(false)
          setManagedSelected([])
        }}
        onSearch={(nextName) => {
          setName(nextName)
          setImagePage(1)
        }}
        onSourceChange={(nextSource) => {
          setSource(nextSource)
          setTenantId(undefined)
          setCategoryId(undefined)
          setImagePage(1)
          changeBulkMode(false)
          setManagedSelected([])
          if (props.mode === 'select' && !multiple)
            setSelected(() => [])
        }}
        onTenantChange={(nextTenantId) => {
          setTenantId(nextTenantId)
          setCategoryId(undefined)
          setImagePage(1)
          changeBulkMode(false)
          setManagedSelected([])
          if (props.mode === 'select' && !multiple)
            setSelected(() => [])
        }}
        selectedCount={managedSelected.length}
        source={source}
        sourceOptions={sourceOptions}
        tenantId={tenantId}
        tenants={tenants}
        uploading={uploading}
        workspace={workspace}
      />

      <div className={styles.library}>
        <ImageCategoryTree
          canManage={canManageCategories}
          categories={categories}
          hideBuiltInCategories={tenantSharedSource}
          onCreate={handleCreateCategory}
          onManage={categoryMenu}
          onSelect={(key) => {
            setCategoryId(
              key === 'all'
                ? undefined
                : key === 'uncategorized'
                  ? '0'
                  : key.replace('category:', ''),
            )
            setImagePage(1)
            if (props.mode === 'select' && !multiple && !bulkMode)
              setSelected(() => [])
          }}
          selectedKey={selectedTreeKey}
        />
        <ImageAssetGrid
          assets={assets}
          bulkMode={bulkMode}
          canManageAssets={
            manageable && (hasPermission('edit') || hasPermission('delete'))
          }
          canSelect={canSelect}
          loading={imageListLoading}
          manageable={manageable}
          managedSelectedIDs={managedSelectedIDs}
          onAssetMenu={assetMenu}
          onPageChange={setImagePage}
          onPreview={setPreviewAssetID}
          onToggleManaged={toggleManagedSelection}
          onToggleSelection={toggleSelection}
          page={imagePage}
          pageSize={imagePageSize}
          previewAssetID={previewAssetID}
          selectionEnabled={props.mode === 'select'}
          selectedIDs={selectedIDs}
          selectionFull={selectionFull}
          source={source}
          tenantId={tenantId}
          total={imageTotal}
          workspace={workspace}
        />
      </div>
    </div>
  )
}
