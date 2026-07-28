import type { ElementType } from 'react'
import * as AntDesignIcons from '@ant-design/icons'
import { createElement } from 'react'

/** Ant Design 可作为菜单图标的组件命名后缀。 */
const menuIconPattern = /(?:Outlined|Filled|TwoTone)$/

/** 图标选择器可展示的 Ant Design 图标名称，按名称稳定排序。 */
export const menuIconNames = Object.keys(AntDesignIcons)
  .filter(name => menuIconPattern.test(name))
  .sort((left, right) => left.localeCompare(right))

/**
 * 根据后端保存的图标组件名称创建菜单图标元素。
 * 名称为空、格式不合法或组件不存在时返回 undefined，由菜单使用默认布局。
 */
export function getMenuIcon(icon?: string) {
  if (!icon || !menuIconPattern.test(icon))
    return undefined
  const IconComponent = AntDesignIcons[icon as keyof typeof AntDesignIcons] as
    | ElementType
    | undefined
  return IconComponent ? createElement(IconComponent) : undefined
}
