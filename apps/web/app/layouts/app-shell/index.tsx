import type { MenuProps } from 'antd'
import type { ReactNode } from 'react'
import type { WorkspaceType } from '@/types/auth'
import {
  ArrowLeftOutlined,
  DownOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { Avatar, Button, Dropdown, Layout, Menu, Tooltip, Watermark } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { getFirstAuthorizedPath, hasPermission } from '@/auth/permission'
import { useAuthContext } from '@/auth/use-auth-context'
import { DICTIONARY_CODE, useDictionary } from '@/contexts/dictionary'
import {
  getMenuAncestorKeys,
  getMenuNodeById,
  getRouteMetaByPath,
  getWorkspaceMenuItems,
} from '@/navigation'
import { useAuthStore } from '@/stores/auth'
import { useBrandStore } from '@/stores/brand'
import styles from './index.module.scss'
import { WorkspaceTabs } from './workspace-tabs'

/** 后台主布局需要渲染的页面内容和工作空间范围。 */
interface AppShellProps {
  /** 当前匹配路由渲染的页面内容。 */
  children: ReactNode
  /** 布局当前处于平台端或租户端，决定品牌、菜单和返回能力。 */
  workspace: WorkspaceType
}

/** 渲染后台系统主布局，包含侧边栏、顶栏、用户菜单和内容区。 */
export function AppShell({ children, workspace }: AppShellProps) {
  // 路由与会话上下文：由当前路径、菜单和权限共同派生标题、选中项和可见导航。
  const navigate = useNavigate()
  const location = useLocation()
  const { currentUser, leaveTenant, logout } = useAuthStore()
  const authContext = useAuthContext()
  const { getLabel } = useDictionary()
  const menus = currentUser?.menus ?? []
  const currentRouteMeta = getRouteMetaByPath(location.pathname, menus)
  const currentSection
    = currentRouteMeta.section
      ?? getLabel(DICTIONARY_CODE.workspaceScope, workspace)
  const menuItems = authContext
    ? getWorkspaceMenuItems(menus, authContext)
    : []
  const roleNames = currentUser?.roles.map(role => role.name).join('、')
  const tenantName = currentUser?.tenantName
  const tenantIconUrl = currentUser?.tenantIconUrl
  const publicBrand = useBrandStore()
  const platformName = currentUser?.platformName ?? publicBrand.name
  const platformIconUrl = currentUser?.platformIconUrl ?? publicBrand.iconUrl
  const brandName
    = workspace === 'tenant' ? (tenantName ?? platformName) : platformName
  const brandIconUrl
    = workspace === 'tenant'
      ? (tenantIconUrl ?? platformIconUrl)
      : platformIconUrl
  const managed = currentUser?.mode === 'managed'
  const currentMenuAncestorKeys = useMemo(
    () => getMenuAncestorKeys(location.pathname, menus),
    [location.pathname, menus],
  )
  const availableTabs = useMemo(() => {
    const menuTabs = menus.flatMap((menuNode) => {
      if (
        menuNode.scope !== workspace
        || menuNode.type !== 'menu'
        || menuNode.status !== 'enabled'
        || !menuNode.visible
        || !menuNode.path
        || !menuNode.permissionCode
        || !hasPermission(authContext, menuNode.permissionCode)
      ) {
        return []
      }
      return [{ path: menuNode.path, title: menuNode.name }]
    })
    return [...menuTabs, { path: `/${workspace}/profile`, title: '个人信息' }]
  }, [authContext, menus, workspace])
  const fallbackPath = getFirstAuthorizedPath(authContext, menus)

  // 响应式侧边栏状态：
  // - desktopSiderCollapsed：桌面端是否折叠为窄栏。
  // - mobileSiderOpen：移动端完整侧栏抽屉是否打开，避免触发折叠菜单浮层。
  const [desktopSiderCollapsed, setDesktopSiderCollapsed] = useState(false)
  const [isMobile, setIsMobile] = useState(false)
  const [mobileSiderOpen, setMobileSiderOpen] = useState(false)
  // 导航目录状态：刷新时从当前路由祖先恢复，后续仍允许用户手动展开或收起目录。
  const [openMenuKeys, setOpenMenuKeys] = useState(currentMenuAncestorKeys)

  useEffect(() => {
    setOpenMenuKeys(currentKeys => [
      ...new Set([...currentKeys, ...currentMenuAncestorKeys]),
    ])
  }, [currentMenuAncestorKeys])

  // 用户菜单提供当前工作空间的个人信息入口和退出能力。
  const userMenuItems: MenuProps['items'] = [
    {
      key: 'profile',
      label: '个人信息',
      icon: <UserOutlined />,
    },
    { type: 'divider' },
    { key: 'logout', label: '退出登录', icon: <LogoutOutlined /> },
  ]

  return (
    <Layout className={`${styles.appShell} min-h-screen`} hasSider>
      {isMobile && mobileSiderOpen
        ? (
            <button
              aria-label="关闭导航"
              className={styles.appSiderBackdrop}
              onClick={() => setMobileSiderOpen(false)}
              type="button"
            />
          )
        : null}
      <Layout.Sider
        breakpoint="lg"
        collapsed={!isMobile && desktopSiderCollapsed}
        collapsedWidth={72}
        collapsible
        className={`${styles.appSider} ${
          isMobile && !mobileSiderOpen ? styles.isMobileHidden : ''
        }`}
        onBreakpoint={(broken) => {
          setIsMobile(broken)
          setMobileSiderOpen(false)
        }}
        trigger={null}
        width={228}
      >
        <div
          className={`${styles.appBrand} ${
            !isMobile && desktopSiderCollapsed ? styles.isCollapsed : ''
          }`}
        >
          <div
            className={`${styles.appBrandMark} ${
              brandIconUrl ? styles.hasImage : ''
            }`}
          >
            {brandIconUrl
              ? (
                  <img alt={`${brandName}图标`} src={brandIconUrl} />
                )
              : (
                  brandName.trim().charAt(0) || '云'
                )}
          </div>
          <div className={`${styles.appBrandCopy} min-w-0`}>
            <div className={styles.appBrandTitle}>{brandName}</div>
            <div className={styles.appBrandSubtitle}>
              {getLabel(DICTIONARY_CODE.workspaceScope, workspace)}
            </div>
          </div>
        </div>
        <Menu
          className={`${styles.appMenu} border-0`}
          inlineCollapsed={!isMobile && desktopSiderCollapsed}
          items={menuItems}
          mode="inline"
          onClick={({ key }) => {
            const menuNode = getMenuNodeById(key, menus)
            if (menuNode?.path) {
              navigate(menuNode.path)
              if (isMobile) {
                setMobileSiderOpen(false)
              }
            }
          }}
          onOpenChange={keys => setOpenMenuKeys(keys)}
          openKeys={openMenuKeys}
          selectedKeys={currentRouteMeta.key ? [currentRouteMeta.key] : []}
          theme="light"
        />
      </Layout.Sider>
      <Layout className={styles.appMainLayout}>
        <Layout.Header className={styles.appHeader}>
          <div className={styles.appHeaderMain}>
            <Tooltip
              title={
                isMobile
                  ? mobileSiderOpen
                    ? '关闭导航'
                    : '展开导航'
                  : desktopSiderCollapsed
                    ? '展开导航'
                    : '收起导航'
              }
            >
              <Button
                aria-label={
                  isMobile
                    ? mobileSiderOpen
                      ? '关闭导航'
                      : '展开导航'
                    : desktopSiderCollapsed
                      ? '展开导航'
                      : '收起导航'
                }
                className={styles.appIconButton}
                icon={
                  (isMobile ? !mobileSiderOpen : desktopSiderCollapsed)
                    ? (
                        <MenuUnfoldOutlined />
                      )
                    : (
                        <MenuFoldOutlined />
                      )
                }
                onClick={() => {
                  if (isMobile) {
                    setMobileSiderOpen(value => !value)
                  }
                  else {
                    setDesktopSiderCollapsed(value => !value)
                  }
                }}
                shape="circle"
                type="text"
              />
            </Tooltip>
            <div className={`${styles.appHeaderPage} min-w-0`}>
              <div className={styles.appHeaderKicker}>{currentSection}</div>
              <div className={styles.appHeaderTitle}>
                {currentRouteMeta.title}
              </div>
            </div>
          </div>
          <div className={styles.appHeaderActions}>
            {managed
              ? (
                  <Button
                    icon={<ArrowLeftOutlined />}
                    onClick={async () => {
                      try {
                        await leaveTenant()
                        navigate('/platform', { replace: true })
                      }
                      catch {
                        logout()
                        navigate('/login', { replace: true })
                      }
                    }}
                    type="primary"
                  >
                    平台代管中 · 返回平台端
                  </Button>
                )
              : null}
            <Dropdown
              menu={{
                items: userMenuItems,
                onClick: ({ key }) => {
                  if (key === 'profile') {
                    navigate(`/${workspace}/profile`)
                  }
                  else if (key === 'logout') {
                    logout()
                    navigate('/login', { replace: true })
                  }
                },
              }}
              placement="bottomRight"
              trigger={['click']}
            >
              <Button className={styles.userTrigger} type="text">
                <div className="flex items-center gap-2">
                  <Avatar
                    className={styles.userAvatar}
                    size={32}
                    src={currentUser?.avatarUrl}
                  >
                    {currentUser?.avatarText ?? '云'}
                  </Avatar>
                  <div className="hidden min-w-0 text-left sm:block">
                    <div className="max-w-28 truncate text-sm font-medium text-slate-900">
                      {currentUser?.name ?? '管理员'}
                    </div>
                    <div className="text-xs text-slate-500">
                      {roleNames || '暂无启用角色'}
                    </div>
                  </div>
                  <DownOutlined
                    className={`${styles.userChevron} hidden sm:block`}
                  />
                </div>
              </Button>
            </Dropdown>
          </div>
        </Layout.Header>
        {currentUser
          ? (
              <WorkspaceTabs
                activePath={location.pathname}
                availableTabs={availableTabs}
                employeeId={currentUser.employeeId}
                fallbackPath={fallbackPath}
                tenantId={currentUser.tenantId}
                workspace={workspace}
              />
            )
          : null}
        <Layout.Content className={styles.appContent}>
          {currentUser
            ? (
                <Watermark
                  className={styles.appWatermark}
                  content={[currentUser.name, currentUser.loginAccount]}
                  font={{
                    color: 'rgba(23, 32, 51, 0.08)',
                    fontSize: 14,
                    fontWeight: 500,
                  }}
                  gap={[120, 120]}
                  height={72}
                  width={180}
                  zIndex={1}
                >
                  {children}
                </Watermark>
              )
            : (
                children
              )}
        </Layout.Content>
      </Layout>
    </Layout>
  )
}
