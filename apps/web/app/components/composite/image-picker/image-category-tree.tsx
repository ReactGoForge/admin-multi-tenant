import type { MenuProps, TreeDataNode } from 'antd'
import type { ImageCategory } from '@/services/media'
import { MoreOutlined, PlusOutlined } from '@ant-design/icons'

import { Button, Dropdown, Tree, Typography } from 'antd'
import styles from './index.module.scss'

/** 图片分类树配置。 */
interface ImageCategoryTreeProps {
  /** 当前图片来源可见的分类。 */
  categories: ImageCategory[]
  /** 当前选中的内置分类键或业务分类键。 */
  selectedKey: string
  /** 是否隐藏“全部图片”和“未分类”等内置入口。 */
  hideBuiltInCategories: boolean
  /** 是否允许新增、重命名和删除分类。 */
  canManage: boolean
  /** 点击新增分类按钮时触发。 */
  onCreate: () => void
  /** 为指定分类生成管理菜单。 */
  onManage: (category: ImageCategory) => MenuProps
  /** 用户选择分类时返回稳定树节点键。 */
  onSelect: (key: string) => void
}

/** ImageCategoryTree 渲染图片分类导航和分类管理入口。 */
export function ImageCategoryTree({
  categories,
  selectedKey,
  hideBuiltInCategories,
  canManage,
  onCreate,
  onManage,
  onSelect,
}: ImageCategoryTreeProps) {
  // 将内置分类入口和业务分类合并为 Ant Design Tree 数据。
  const treeData: TreeDataNode[] = [
    ...(hideBuiltInCategories
      ? []
      : [
          { key: 'all', title: '全部图片' },
          { key: 'uncategorized', title: '未分类' },
        ]),
    ...categories.map(category => ({
      key: `category:${category.id}`,
      title: (
        <span className={styles.treeNodeTitle}>
          <span className={styles.treeNodeName}>{category.name}</span>
          {canManage && !category.isShared
            ? (
                <Dropdown menu={onManage(category)} trigger={['click']}>
                  <Button
                    aria-label={`管理分类${category.name}`}
                    icon={<MoreOutlined />}
                    onClick={event => event.stopPropagation()}
                    size="small"
                    type="text"
                  />
                </Dropdown>
              )
            : null}
        </span>
      ),
    })),
  ]

  return (
    <aside className={styles.categoryPanel}>
      <div className={styles.categoryHeader}>
        <Typography.Text strong>图片分类</Typography.Text>
        {canManage
          ? (
              <Button
                aria-label="新增分类"
                icon={<PlusOutlined />}
                onClick={onCreate}
                size="small"
                type="text"
              />
            )
          : null}
      </div>
      <Tree
        blockNode
        onSelect={keys => onSelect(String(keys[0] ?? 'all'))}
        selectedKeys={[selectedKey]}
        treeData={treeData}
      />
    </aside>
  )
}
