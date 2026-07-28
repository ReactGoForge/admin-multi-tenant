import type { TabsProps } from 'antd'
import type { WorkspaceType } from '@/types/auth'
import { Dropdown, Tabs } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import styles from './index.module.scss'

/** 工作空间标签页保存的最小路由信息。 */
export interface WorkspaceTab {
  /** 标签页对应的站内绝对路径。 */
  path: string
  /** 标签页展示标题。 */
  title: string
}

interface WorkspaceTabsProps {
  /** 当前平台端或租户端工作空间。 */
  workspace: WorkspaceType
  /** 当前员工标识，用于隔离不同登录身份的访问记录。 */
  employeeId: string
  /** 当前租户标识，用于隔离同一平台员工代管不同租户时的访问记录。 */
  tenantId: string | null
  /** 当前账号关闭全部标签后应返回的第一个授权页面。 */
  fallbackPath: string
  /** 当前正在展示的路由路径。 */
  activePath: string
  /** 当前身份允许访问并可加入标签栏的页面。 */
  availableTabs: WorkspaceTab[]
}

/**
 * 渲染工作空间访问标签栏，并在当前会话中保存已打开页面。
 * 支持标签关闭和右键批量关闭，标签溢出时使用 Ant Design 原生折叠菜单。
 */
export function WorkspaceTabs({
  workspace,
  employeeId,
  tenantId,
  fallbackPath,
  activePath,
  availableTabs,
}: WorkspaceTabsProps) {
  const navigate = useNavigate()
  const homePath = `/${workspace}`
  const homeTab = availableTabs.find(
    workspaceTab => workspaceTab.path === homePath,
  )
  const workspaceStorageScope
    = workspace === 'tenant' ? `tenant:${tenantId}` : workspace
  const storageKey = `admin-multi-tenant:workspace-tabs:${employeeId}:${workspaceStorageScope}`
  const allowedTabMap = useMemo(
    () =>
      new Map(
        availableTabs.map(workspaceTab => [
          workspaceTab.path,
          workspaceTab,
        ]),
      ),
    [availableTabs],
  )

  // 标签访问状态：workspaceTabs 保存当前会话已访问页面，restored 控制读取完成前不覆盖浏览器存储。
  const [workspaceTabs, setWorkspaceTabs] = useState<WorkspaceTab[]>(
    homeTab ? [homeTab] : [],
  )
  const [restored, setRestored] = useState(false)

  // 首次进入工作空间时恢复仍有权限访问的标签，并保留已授权的当前页面与首页。
  useEffect(() => {
    let storedTabs: WorkspaceTab[] = []
    try {
      const storedValue = sessionStorage.getItem(storageKey)
      const parsedValue = storedValue ? JSON.parse(storedValue) : []
      if (Array.isArray(parsedValue)) {
        storedTabs = parsedValue.flatMap((storedTab) => {
          if (!storedTab || typeof storedTab.path !== 'string')
            return []
          const allowedTab = allowedTabMap.get(storedTab.path)
          return allowedTab ? [allowedTab] : []
        })
      }
    }
    catch {
      storedTabs = []
    }

    const restoredTabs = new Map<string, WorkspaceTab>()
    if (homeTab)
      restoredTabs.set(homeTab.path, homeTab)
    for (const storedTab of storedTabs) {
      restoredTabs.set(storedTab.path, storedTab)
    }
    const activeTab = allowedTabMap.get(activePath)
    if (activeTab)
      restoredTabs.set(activeTab.path, activeTab)
    setWorkspaceTabs([...restoredTabs.values()])
    setRestored(true)
  }, [activePath, allowedTabMap, homeTab, storageKey])

  // 路由切换时把新页面加入标签栏；相同页面只同步最新标题，不改变既有顺序。
  useEffect(() => {
    if (!restored)
      return
    const activeTab = allowedTabMap.get(activePath)
    if (!activeTab)
      return
    setWorkspaceTabs((currentTabs) => {
      const activeIndex = currentTabs.findIndex(
        workspaceTab => workspaceTab.path === activePath,
      )
      if (activeIndex < 0)
        return [...currentTabs, activeTab]
      if (currentTabs[activeIndex].title === activeTab.title)
        return currentTabs
      return currentTabs.map((workspaceTab, index) =>
        index === activeIndex ? activeTab : workspaceTab,
      )
    })
  }, [activePath, allowedTabMap, restored])

  // 标签集合变化后写入 sessionStorage，刷新页面仍保留当前浏览会话。
  useEffect(() => {
    if (!restored)
      return
    sessionStorage.setItem(storageKey, JSON.stringify(workspaceTabs))
  }, [restored, storageKey, workspaceTabs])

  const activeTabIndex = workspaceTabs.findIndex(
    workspaceTab => workspaceTab.path === activePath,
  )

  /** 关闭指定标签集合，并在活动页被关闭时切换到相邻的可用标签。 */
  const closeWorkspaceTabs = (paths: string[]) => {
    const closePaths = new Set(
      paths.filter(path => !homeTab || path !== homeTab.path),
    )
    if (!closePaths.size)
      return
    let nextTabs = workspaceTabs.filter(
      workspaceTab => !closePaths.has(workspaceTab.path),
    )
    if (!nextTabs.length) {
      const fallbackTab = allowedTabMap.get(fallbackPath)
      if (fallbackTab)
        nextTabs = [fallbackTab]
    }
    setWorkspaceTabs(nextTabs)
    if (closePaths.has(activePath)) {
      const fallbackIndex = Math.max(
        0,
        Math.min(activeTabIndex - 1, nextTabs.length - 1),
      )
      navigate(nextTabs[fallbackIndex]?.path ?? fallbackPath)
    }
  }

  const tabItems: TabsProps['items'] = workspaceTabs.map(
    (workspaceTab, tabIndex) => ({
      key: workspaceTab.path,
      closable: !homeTab || workspaceTab.path !== homeTab.path,
      label: (
        <Dropdown
          menu={{
            items: [
              {
                key: 'current',
                label: '关闭当前标签',
                disabled: workspaceTab.path === homeTab?.path,
              },
              { key: 'others', label: '关闭其他标签' },
              {
                key: 'left',
                label: '关闭左侧标签',
                disabled: tabIndex <= 1,
              },
              {
                key: 'right',
                label: '关闭右侧标签',
                disabled: tabIndex === workspaceTabs.length - 1,
              },
              { type: 'divider' },
              { key: 'all', label: '关闭全部标签' },
            ],
            onClick: ({ key }) => {
              if (key === 'current') {
                closeWorkspaceTabs([workspaceTab.path])
              }
              else if (key === 'others') {
                closeWorkspaceTabs(
                  workspaceTabs
                    .filter(tab => tab.path !== workspaceTab.path)
                    .map(tab => tab.path),
                )
              }
              else if (key === 'left') {
                closeWorkspaceTabs(
                  workspaceTabs.slice(0, tabIndex).map(tab => tab.path),
                )
              }
              else if (key === 'right') {
                closeWorkspaceTabs(
                  workspaceTabs.slice(tabIndex + 1).map(tab => tab.path),
                )
              }
              else if (key === 'all') {
                closeWorkspaceTabs(
                  workspaceTabs.map(tab => tab.path),
                )
              }
            },
          }}
          trigger={['contextMenu']}
        >
          <span className={styles.workspaceTabLabel}>{workspaceTab.title}</span>
        </Dropdown>
      ),
    }),
  )

  return (
    <div className={styles.workspaceTabs}>
      <Tabs
        activeKey={activePath}
        hideAdd
        items={tabItems}
        onChange={path => navigate(path)}
        onEdit={(targetKey, action) => {
          if (action === 'remove' && typeof targetKey === 'string') {
            closeWorkspaceTabs([targetKey])
          }
        }}
        size="small"
        type="editable-card"
      />
    </div>
  )
}
