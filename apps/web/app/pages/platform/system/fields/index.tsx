import type { ColumnsType } from 'antd/es/table'
import type { FormFieldConfig } from '@/components/composite/schema-form'
import type { DictionaryItem, DictionaryItemMutation, DictionaryType, DictionaryTypeMutation } from '@/services/dictionaries'
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons'

import { Alert, App, Button, Card, Empty, Form, Space, Table, Tag } from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { ConfirmDelete } from '@/components/base/confirm-delete'
import { PageContainer } from '@/components/base/page-container'
import { StatusSwitch } from '@/components/base/status-switch'
import { FormDrawer } from '@/components/composite/form-drawer'
import {

  SchemaForm,
} from '@/components/composite/schema-form'
import { DICTIONARY_CODE, useDictionary } from '@/contexts/dictionary'
import {
  createPlatformDictionary,
  createPlatformDictionaryItem,
  deletePlatformDictionary,
  deletePlatformDictionaryItem,

  fetchPlatformDictionaries,
  updatePlatformDictionary,
  updatePlatformDictionaryItem,
} from '@/services/dictionaries'
import { getErrorMessage } from '@/services/errors'
import { isSilentRequestError } from '@/services/http'
import styles from './index.module.scss'

/** PlatformFieldsPage 以左侧字典字段和右侧字典内容维护全平台统一字典。 */
export default function PlatformFieldsPage() {
  // 页面反馈和字典上下文：refreshOptions 在管理操作成功后同步刷新全局消费字典。
  const { message } = App.useApp()
  const { getLabel, getOptions, refresh: refreshOptions } = useDictionary()

  // 字典字段和字典项表单：分别控制两个独立的新增、编辑抽屉。
  const [typeForm] = Form.useForm<DictionaryTypeMutation>()
  const [itemForm] = Form.useForm<DictionaryItemMutation>()

  // 字典数据和选择状态：
  // - dictionaryTypes：接口返回的全部字典字段及其字典项。
  // - selectedDictionaryTypeID/selectedType：左侧当前选中的字段 ID 及对应实体。
  // - editingType/editingItem：两个抽屉当前编辑的目标；空值表示新增。
  // - dictionaryLoading/dictionaryMutationLoading：整页数据加载和写操作提交状态。
  const [dictionaryTypes, setDictionaryTypes] = useState<DictionaryType[]>([])
  const [selectedDictionaryTypeID, setSelectedDictionaryTypeID]
    = useState<string>()
  const [editingType, setEditingType] = useState<DictionaryType | null>()
  const [editingItem, setEditingItem] = useState<DictionaryItem | null>()
  const [typeDrawerOpen, setTypeDrawerOpen] = useState(false)
  const [itemDrawerOpen, setItemDrawerOpen] = useState(false)
  const [dictionaryLoading, setDictionaryLoading] = useState(true)
  const [dictionaryMutationLoading, setDictionaryMutationLoading]
    = useState(false)

  const selectedType
    = dictionaryTypes.find(item => item.id === selectedDictionaryTypeID)
      ?? null
  const statusOptions = getOptions(DICTIONARY_CODE.entityStatus)

  /**
   * loadDictionaryTypes 重新加载完整字典数据，并尽量保留指定或当前选中的字段。
   * 当原字段已被删除时自动选中返回列表中的第一个字段。
   */
  const loadDictionaryTypes = useCallback(
    async (preferredID?: string, signal?: AbortSignal) => {
      setDictionaryLoading(true)
      try {
        const result = await fetchPlatformDictionaries(signal)
        setDictionaryTypes(result)
        setSelectedDictionaryTypeID((current) => {
          const candidate = preferredID ?? current
          return result.some(item => item.id === candidate)
            ? candidate
            : result[0]?.id
        })
      }
      catch (error) {
        if (!isSilentRequestError(error)) {
          void message.error(getErrorMessage(error, '字典数据加载失败'))
        }
      }
      finally {
        if (!signal?.aborted)
          setDictionaryLoading(false)
      }
    },
    [message],
  )

  useEffect(() => {
    const controller = new AbortController()
    void loadDictionaryTypes(undefined, controller.signal)
    return () => controller.abort()
  }, [loadDictionaryTypes])

  /**
   * runDictionaryMutation 执行字典写操作，并同时刷新管理数据和全局字典上下文。
   * @returns 操作成功返回 true，供抽屉提交逻辑决定是否关闭。
   */
  const runDictionaryMutation = async (
    action: () => Promise<unknown>,
    success: string,
  ) => {
    setDictionaryMutationLoading(true)
    try {
      await action()
      await Promise.all([
        loadDictionaryTypes(selectedDictionaryTypeID),
        refreshOptions(),
      ])
      void message.success(success)
      return true
    }
    catch (error) {
      if (!isSilentRequestError(error)) {
        void message.error(getErrorMessage(error))
      }
      return false
    }
    finally {
      setDictionaryMutationLoading(false)
    }
  }

  /** openDictionaryTypeDrawer 重置并回填字典字段表单，然后打开新增或编辑抽屉。 */
  const openDictionaryTypeDrawer = (dictionaryType?: DictionaryType) => {
    setEditingType(dictionaryType ?? null)
    typeForm.resetFields()
    typeForm.setFieldsValue(
      dictionaryType
        ? {
            code: dictionaryType.code,
            name: dictionaryType.name,
            remark: dictionaryType.remark ?? undefined,
            sort: dictionaryType.sort,
            status: dictionaryType.status,
          }
        : { sort: 10, status: 'enabled' },
    )
    setTypeDrawerOpen(true)
  }

  /** openDictionaryItemDrawer 重置并回填字典项表单，然后打开新增或编辑抽屉。 */
  const openDictionaryItemDrawer = (item?: DictionaryItem) => {
    setEditingItem(item ?? null)
    itemForm.resetFields()
    itemForm.setFieldsValue(
      item
        ? {
            label: item.label,
            value: item.value,
            sort: item.sort,
            status: item.status,
          }
        : { sort: 10, status: 'enabled' },
    )
    setItemDrawerOpen(true)
  }

  // 系统字典字段禁止修改稳定编码和状态，自定义字段允许完整维护。
  const dictionaryTypeFields = useMemo<
    Array<FormFieldConfig<DictionaryTypeMutation>>
  >(
    () => [
      {
        name: 'name',
        label: '字段名称',
        rules: [{ required: true, message: '请输入字段名称' }],
        componentProps: { maxLength: 50 },
      },
      {
        name: 'code',
        label: '字段编码',
        disabled: Boolean(editingType?.isSystem),
        rules: [
          { required: true, message: '请输入字段编码' },
          {
            pattern: /^[a-z][a-z0-9_]{1,63}$/,
            message: '请使用小写字母、数字和下划线，且以字母开头',
          },
        ],
        componentProps: { maxLength: 64 },
      },
      {
        name: 'remark',
        label: '备注',
        type: 'textarea',
        componentProps: { maxLength: 200, showCount: true },
      },
      {
        name: 'sort',
        label: '排序',
        type: 'number',
        rules: [{ required: true, message: '请输入排序' }],
        componentProps: { min: 0 },
      },
      {
        name: 'status',
        label: '状态',
        type: 'select',
        disabled: Boolean(editingType?.isSystem),
        rules: [{ required: true, message: '请选择状态' }],
        options: statusOptions,
      },
    ],
    [editingType?.isSystem, statusOptions],
  )

  // 系统字典项只允许修改展示文案和排序，稳定值与状态保持受保护。
  const dictionaryItemFields = useMemo<
    Array<FormFieldConfig<DictionaryItemMutation>>
  >(
    () => [
      {
        name: 'label',
        label: '展示文案',
        rules: [{ required: true, message: '请输入展示文案' }],
        componentProps: { maxLength: 50 },
      },
      {
        name: 'value',
        label: '稳定值',
        disabled: Boolean(selectedType?.isSystem),
        rules: [{ required: true, message: '请输入稳定值' }],
        componentProps: { maxLength: 100 },
      },
      {
        name: 'sort',
        label: '排序',
        type: 'number',
        rules: [{ required: true, message: '请输入排序' }],
        componentProps: { min: 0 },
      },
      {
        name: 'status',
        label: '状态',
        type: 'select',
        disabled: Boolean(selectedType?.isSystem),
        rules: [{ required: true, message: '请选择状态' }],
        options: statusOptions,
      },
    ],
    [selectedType?.isSystem, statusOptions],
  )

  // 右侧字典项表格提供状态维护以及受系统字典保护规则控制的编辑、删除操作。
  const dictionaryItemColumns: ColumnsType<DictionaryItem> = [
    { title: '展示文案', dataIndex: 'label' },
    {
      title: '稳定值',
      dataIndex: 'value',
      render: value => (
        <code className={styles.stableValue} title={value} translate="no">
          {value}
        </code>
      ),
    },
    {
      title: '排序',
      dataIndex: 'sort',
      width: 90,
      render: sort => <span className={styles.numericCell}>{sort}</span>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 110,
      render: (status, item) =>
        selectedType?.isSystem
          ? (
              <Tag color={status === 'enabled' ? 'success' : 'default'}>
                {getLabel(DICTIONARY_CODE.entityStatus, status)}
              </Tag>
            )
          : (
              <StatusSwitch
                value={status}
                onChange={(value) => {
                  if (!selectedType)
                    return
                  void runDictionaryMutation(
                    () =>
                      updatePlatformDictionaryItem(selectedType.id, item.id, {
                        label: item.label,
                        value: item.value,
                        sort: item.sort,
                        status: value,
                      }),
                    '字典项状态已更新',
                  )
                }}
              />
            ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 140,
      render: (_, item) => (
        <Space>
          <Button onClick={() => openDictionaryItemDrawer(item)} type="link">
            编辑
          </Button>
          {selectedType?.isSystem
            ? null
            : (
                <ConfirmDelete
                  onConfirm={() => {
                    if (!selectedType)
                      return
                    void runDictionaryMutation(
                      () => deletePlatformDictionaryItem(selectedType.id, item.id),
                      '字典项已删除',
                    )
                  }}
                >
                  <Button danger type="link">
                    删除
                  </Button>
                </ConfirmDelete>
              )}
        </Space>
      ),
    },
  ]

  return (
    <PageContainer>
      <Alert
        title="自定义字典只有在业务页面按字段编码接入后才会生效；修改已接入的字段编码可能导致页面无法读取选项。"
        showIcon
        type="info"
      />
      <div className={styles.dictionaryLayout}>
        <Card
          className={styles.panelCard}
          extra={(
            <Button
              icon={<PlusOutlined />}
              onClick={() => openDictionaryTypeDrawer()}
            >
              新增字段
            </Button>
          )}
          loading={dictionaryLoading}
          size="medium"
          title="字典字段"
          variant="borderless"
        >
          {dictionaryTypes.length
            ? (
                <ul className={styles.typeList}>
                  {dictionaryTypes.map((dictionaryType) => {
                    const selected = dictionaryType.id === selectedDictionaryTypeID
                    return (
                      <li key={dictionaryType.id}>
                        <button
                          aria-pressed={selected}
                          className={`${styles.typeListButton} ${
                            selected ? styles.isSelected : ''
                          }`}
                          onClick={() =>
                            setSelectedDictionaryTypeID(dictionaryType.id)}
                          type="button"
                        >
                          <span className={styles.typeListText}>
                            <span
                              className={styles.typeListName}
                              title={dictionaryType.name}
                            >
                              {dictionaryType.name}
                            </span>
                            <code
                              className={styles.typeListCode}
                              title={dictionaryType.code}
                              translate="no"
                            >
                              {dictionaryType.code}
                            </code>
                          </span>
                          <span className={styles.typeListMeta}>
                            <Tag
                              bordered={false}
                              className={styles.statusTag}
                              color={
                                dictionaryType.status === 'enabled'
                                  ? 'success'
                                  : 'default'
                              }
                            >
                              {getLabel(
                                DICTIONARY_CODE.entityStatus,
                                dictionaryType.status,
                              )}
                            </Tag>
                            <span className={styles.itemCount}>
                              {dictionaryType.items.length}
                              {' '}
                              项
                            </span>
                          </span>
                        </button>
                      </li>
                    )
                  })}
                </ul>
              )
            : (
                <Empty
                  description="暂无字典字段"
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                />
              )}
        </Card>

        <Card
          className={styles.panelCard}
          extra={
            selectedType
              ? (
                  <Space size={8}>
                    {selectedType.isSystem
                      ? null
                      : (
                          <Button
                            icon={<PlusOutlined />}
                            onClick={() => openDictionaryItemDrawer()}
                            type="primary"
                          >
                            新增字典项
                          </Button>
                        )}
                    <Button
                      icon={<EditOutlined />}
                      onClick={() => openDictionaryTypeDrawer(selectedType)}
                    >
                      编辑字段
                    </Button>
                    {selectedType.isSystem
                      ? null
                      : (
                          <ConfirmDelete
                            description={
                              selectedType.items.length
                                ? '请先删除该字段下的全部字典项。'
                                : '删除后无法恢复，请谨慎操作。'
                            }
                            disabled={selectedType.items.length > 0}
                            onConfirm={() => {
                              void runDictionaryMutation(
                                () => deletePlatformDictionary(selectedType.id),
                                '字典字段已删除',
                              )
                            }}
                          >
                            <Button
                              danger
                              disabled={selectedType.items.length > 0}
                              icon={<DeleteOutlined />}
                              type="text"
                            >
                              删除字段
                            </Button>
                          </ConfirmDelete>
                        )}
                  </Space>
                )
              : null
          }
          loading={dictionaryLoading}
          size="medium"
          title={
            selectedType
              ? (
                  <div className={styles.detailTitle}>
                    <span className={styles.detailName} title={selectedType.name}>
                      {selectedType.name}
                    </span>
                    {selectedType.isSystem
                      ? (
                          <Tag color="blue">系统字段</Tag>
                        )
                      : null}
                    <Tag
                      bordered={false}
                      className={styles.statusTag}
                      color={
                        selectedType.status === 'enabled' ? 'success' : 'default'
                      }
                    >
                      {getLabel(DICTIONARY_CODE.entityStatus, selectedType.status)}
                    </Tag>
                  </div>
                )
              : (
                  '字典项'
                )
          }
          variant="borderless"
        >
          {selectedType
            ? (
                <>
                  <div className={styles.detailMeta}>
                    <code
                      className={styles.detailCode}
                      title={selectedType.code}
                      translate="no"
                    >
                      {selectedType.code}
                    </code>
                    <span className={styles.detailCount}>
                      {selectedType.items.length}
                      {' '}
                      个字典项
                    </span>
                    {selectedType.remark
                      ? (
                          <p className={styles.remark}>{selectedType.remark}</p>
                        )
                      : null}
                  </div>
                  <Table<DictionaryItem>
                    columns={dictionaryItemColumns}
                    dataSource={selectedType.items}
                    locale={{
                      emptyText: (
                        <Empty
                          description="暂无字典项"
                          image={Empty.PRESENTED_IMAGE_SIMPLE}
                        />
                      ),
                    }}
                    pagination={false}
                    rowKey="id"
                    size="small"
                  />
                </>
              )
            : (
                <Empty description="请选择字典字段" />
              )}
        </Card>
      </div>

      <FormDrawer
        loading={dictionaryMutationLoading}
        onClose={() => setTypeDrawerOpen(false)}
        onSubmit={() => typeForm.submit()}
        open={typeDrawerOpen}
        title={editingType ? '编辑字典字段' : '新增字典字段'}
      >
        <SchemaForm
          columns={1}
          fields={dictionaryTypeFields}
          form={typeForm}
          onFinish={async (values) => {
            const success = await runDictionaryMutation(
              () =>
                editingType
                  ? updatePlatformDictionary(editingType.id, values)
                  : createPlatformDictionary(values),
              editingType ? '字典字段已更新' : '字典字段已新增',
            )
            if (success)
              setTypeDrawerOpen(false)
          }}
          showActions={false}
        />
      </FormDrawer>

      <FormDrawer
        loading={dictionaryMutationLoading}
        onClose={() => setItemDrawerOpen(false)}
        onSubmit={() => itemForm.submit()}
        open={itemDrawerOpen}
        title={editingItem ? '编辑字典项' : '新增字典项'}
      >
        <SchemaForm
          columns={1}
          fields={dictionaryItemFields}
          form={itemForm}
          onFinish={async (values) => {
            if (!selectedType)
              return
            const success = await runDictionaryMutation(
              () =>
                editingItem
                  ? updatePlatformDictionaryItem(
                      selectedType.id,
                      editingItem.id,
                      values,
                    )
                  : createPlatformDictionaryItem(selectedType.id, values),
              editingItem ? '字典项已更新' : '字典项已新增',
            )
            if (success)
              setItemDrawerOpen(false)
          }}
          showActions={false}
        />
      </FormDrawer>
    </PageContainer>
  )
}
