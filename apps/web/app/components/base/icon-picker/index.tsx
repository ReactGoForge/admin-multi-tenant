import {
  CloseOutlined,
  DownOutlined,
  PlusOutlined,
  SearchOutlined,
} from '@ant-design/icons'
import { Button, Empty, Input, Popover, theme } from 'antd'
import { useMemo, useState } from 'react'

import { getMenuIcon, menuIconNames } from '@/navigation/route-meta'

/** IconPicker 配置。 */
export interface IconPickerProps {
  /** 当前选中的 Ant Design 图标名称。 */
  value?: string
  /** 选择或清空图标时触发。 */
  onChange?: (value?: string) => void
  /** 是否禁用打开和清空操作，默认 false。 */
  disabled?: boolean
  /** 是否显示清空入口，默认 true。 */
  allowClear?: boolean
}

const ICON_BATCH_SIZE = 96
const LOAD_MORE_THRESHOLD = 48

/** IconPicker 通过分批渲染的紧凑网格选择 Ant Design 图标。 */
export function IconPicker({
  value,
  onChange,
  disabled = false,
  allowClear = true,
}: IconPickerProps) {
  // 弹层、搜索词和分批渲染数量共同控制图标面板当前可见内容。
  const { token } = theme.useToken()
  const [open, setOpen] = useState(false)
  const [keyword, setKeyword] = useState('')
  const [visibleCount, setVisibleCount] = useState(ICON_BATCH_SIZE)
  // 按不区分大小写的名称关键字过滤 Ant Design 图标注册表。
  const filteredIcons = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase()
    return normalizedKeyword
      ? menuIconNames.filter(name =>
          name.toLowerCase().includes(normalizedKeyword),
        )
      : menuIconNames
  }, [keyword])
  const visibleIcons = filteredIcons.slice(0, visibleCount)

  /** close 关闭图标弹层并清空搜索词，下一次打开恢复完整列表。 */
  const close = () => {
    setOpen(false)
    setKeyword('')
  }

  const content = (
    <div className="w-80">
      <div className="flex items-center gap-2">
        <Input
          allowClear
          onChange={(event) => {
            setKeyword(event.target.value)
            setVisibleCount(ICON_BATCH_SIZE)
          }}
          placeholder="搜索图标"
          prefix={<SearchOutlined />}
          value={keyword}
        />
        {allowClear
          ? (
              <Button
                aria-label="清空图标"
                disabled={!value}
                icon={<CloseOutlined />}
                onClick={() => onChange?.(undefined)}
                type="text"
              />
            )
          : null}
      </div>
      {filteredIcons.length
        ? (
            <div
              className="mt-3 grid max-h-64 grid-cols-8 gap-1 overflow-y-auto pr-1"
              onScroll={(event) => {
                const target = event.currentTarget
                const nearBottom
                  = target.scrollHeight - target.scrollTop - target.clientHeight
                    <= LOAD_MORE_THRESHOLD
                if (nearBottom && visibleCount < filteredIcons.length) {
                  setVisibleCount(current =>
                    Math.min(current + ICON_BATCH_SIZE, filteredIcons.length),
                  )
                }
              }}
            >
              {visibleIcons.map((name) => {
                const selected = name === value
                return (
                  <button
                    aria-label={name}
                    aria-pressed={selected}
                    className="flex aspect-square items-center justify-center rounded-md border bg-white text-lg transition-colors hover:!border-blue-400 hover:!bg-blue-50 hover:!text-blue-600 focus-visible:!border-blue-400 focus-visible:!text-blue-600 focus-visible:outline-none"
                    key={name}
                    onClick={() => {
                      onChange?.(name)
                      close()
                    }}
                    style={
                      selected
                        ? {
                            background: token.colorPrimaryBg,
                            borderColor: token.colorPrimary,
                            color: token.colorPrimary,
                          }
                        : { borderColor: token.colorBorder }
                    }
                    type="button"
                  >
                    {getMenuIcon(name)}
                  </button>
                )
              })}
            </div>
          )
        : (
            <Empty
              className="my-6"
              description="没有匹配的图标"
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            />
          )}
    </div>
  )

  return (
    <Popover
      content={content}
      onOpenChange={(nextOpen) => {
        if (disabled)
          return
        if (nextOpen) {
          setVisibleCount(ICON_BATCH_SIZE)
          setOpen(true)
        }
        else {
          close()
        }
      }}
      open={open}
      placement="bottomLeft"
      trigger="click"
    >
      <Button
        aria-expanded={open}
        className="gap-1 px-2"
        disabled={disabled}
        role="combobox"
        style={{
          minWidth: token.controlHeightLG,
        }}
        size="small"
      >
        <span className="flex w-4 items-center justify-center text-base">
          {value
            ? (
                getMenuIcon(value)
              )
            : (
                <PlusOutlined className="text-slate-400" />
              )}
        </span>
        <DownOutlined className="text-[10px] text-slate-400" />
      </Button>
    </Popover>
  )
}
