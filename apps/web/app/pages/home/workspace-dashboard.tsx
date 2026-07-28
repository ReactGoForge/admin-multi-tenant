import type { WorkspaceType } from '@/types/auth'
import {
  ApartmentOutlined,
  AppstoreOutlined,
  ArrowRightOutlined,
  KeyOutlined,
  SafetyCertificateOutlined,
  TeamOutlined,
} from '@ant-design/icons'
import { Avatar, Button, Card, Col, Row, Tag, Typography } from 'antd'
import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import { hasPermission } from '@/auth/permission'
import { useAuthContext } from '@/auth/use-auth-context'
import { getMenuIcon } from '@/navigation/route-meta'
import { useAuthStore } from '@/stores/auth'
import styles from './workspace-dashboard.module.scss'

interface WorkspaceDashboardProps {
  /** 首页所属的平台端或租户端工作空间。 */
  workspace: WorkspaceType
}

/**
 * 渲染平台端或租户端工作台首页。
 * 页面只汇总当前会话已有的角色、权限和授权菜单，并提供现有业务页面的快捷入口。
 */
export function WorkspaceDashboard({ workspace }: WorkspaceDashboardProps) {
  const navigate = useNavigate()
  const authContext = useAuthContext()
  const currentUser = useAuthStore(state => state.currentUser)
  const workspaceMenus = currentUser?.menus ?? []
  const authorizedPages = useMemo(
    () =>
      workspaceMenus
        .filter(
          menuNode =>
            menuNode.scope === workspace
            && menuNode.type === 'menu'
            && menuNode.status === 'enabled'
            && menuNode.visible
            && menuNode.path !== `/${workspace}`
            && menuNode.path
            && menuNode.permissionCode
            && hasPermission(authContext, menuNode.permissionCode),
        )
        .sort((left, right) => left.sort - right.sort),
    [authContext, workspace, workspaceMenus],
  )
  const workspaceDirectories = useMemo(() => {
    const directoryIds = new Set(
      authorizedPages.flatMap(menuNode =>
        menuNode.parentId ? [menuNode.parentId] : [],
      ),
    )
    return workspaceMenus.filter(menuNode => directoryIds.has(menuNode.id))
  }, [authorizedPages, workspaceMenus])

  const isPlatform = workspace === 'platform'
  const workspaceName = isPlatform
    ? (currentUser?.platformName ?? '平台管理端')
    : (currentUser?.tenantName ?? '租户管理端')
  const roleNames = currentUser?.roles.map(role => role.name) ?? []
  const dashboardSummary = [
    {
      label: '可用业务页面',
      value: authorizedPages.length,
      description: '按当前账号实时权限统计',
      icon: <AppstoreOutlined />,
      tone: 'blue',
    },
    {
      label: '业务模块',
      value: workspaceDirectories.length,
      description: isPlatform ? '平台运营与系统管理' : '企业协作与系统管理',
      icon: isPlatform ? <ApartmentOutlined /> : <TeamOutlined />,
      tone: 'cyan',
    },
    {
      label: '当前角色',
      value: roleNames.length,
      description: roleNames.join('、') || '暂无启用角色',
      icon: <SafetyCertificateOutlined />,
      tone: 'green',
    },
    {
      label: '授权能力',
      value: currentUser?.isSuperAdmin
        ? '全部'
        : (currentUser?.permissions.length ?? 0),
      description: currentUser?.isSuperAdmin
        ? '平台超级管理员动态放行'
        : '由当前启用角色共同授权',
      icon: <KeyOutlined />,
      tone: 'amber',
    },
  ]

  return (
    <div className={styles.dashboard}>
      <section className={styles.hero}>
        <div className={styles.heroContent}>
          <Tag className={styles.workspaceTag} variant="filled">
            {isPlatform ? 'PLATFORM CONSOLE' : 'TENANT WORKSPACE'}
          </Tag>
          <Typography.Title className={styles.heroTitle} level={2}>
            {currentUser?.name ?? '管理员'}
            ，欢迎回到
            {workspaceName}
          </Typography.Title>
          <Typography.Paragraph className={styles.heroDescription}>
            {isPlatform
              ? '从这里统一进入租户运营、用户管理、权限配置与系统运行记录。'
              : '从这里快速进入企业成员、角色权限、用户与操作记录管理。'}
          </Typography.Paragraph>
        </div>
        <div className={styles.heroIdentity}>
          <Avatar className={styles.heroAvatar} size={54}>
            {currentUser?.avatarText ?? '云'}
          </Avatar>
          <div>
            <div className={styles.identityName}>{currentUser?.name}</div>
            <div className={styles.identityMeta}>
              {currentUser?.mode === 'managed'
                ? '平台代管访问'
                : isPlatform
                  ? '平台工作空间'
                  : '租户工作空间'}
            </div>
          </div>
        </div>
      </section>

      <Row gutter={[16, 16]}>
        {dashboardSummary.map(summaryItem => (
          <Col key={summaryItem.label} lg={6} sm={12} xs={24}>
            <Card className={styles.summaryCard} variant="borderless">
              <div className={styles.summaryHeader}>
                <span className={styles.summaryLabel}>{summaryItem.label}</span>
                <span
                  className={`${styles.summaryIcon} ${styles[summaryItem.tone]}`}
                >
                  {summaryItem.icon}
                </span>
              </div>
              <div className={styles.summaryValue}>{summaryItem.value}</div>
              <div className={styles.summaryDescription}>
                {summaryItem.description}
              </div>
            </Card>
          </Col>
        ))}
      </Row>

      <Row gutter={[16, 16]}>
        <Col lg={16} xs={24}>
          <Card
            className={styles.contentCard}
            title="快捷入口"
            variant="borderless"
          >
            {authorizedPages.length
              ? (
                  <div className={styles.quickGrid}>
                    {authorizedPages.slice(0, 8).map((menuNode) => {
                      const parentName = workspaceMenus.find(
                        parentNode => parentNode.id === menuNode.parentId,
                      )?.name
                      return (
                        <button
                          className={styles.quickItem}
                          key={menuNode.id}
                          onClick={() => navigate(menuNode.path as string)}
                          type="button"
                        >
                          <span className={styles.quickIcon}>
                            {getMenuIcon(menuNode.icon) ?? <AppstoreOutlined />}
                          </span>
                          <span className={styles.quickCopy}>
                            <strong>{menuNode.name}</strong>
                            <small>{parentName ?? '业务管理'}</small>
                          </span>
                          <ArrowRightOutlined className={styles.quickArrow} />
                        </button>
                      )
                    })}
                  </div>
                )
              : (
                  <div className={styles.noAccess}>当前账号暂无可访问的业务页面</div>
                )}
          </Card>
        </Col>
        <Col lg={8} xs={24}>
          <Card
            className={styles.contentCard}
            title="当前访问身份"
            variant="borderless"
          >
            <div className={styles.accessList}>
              <div className={styles.accessItem}>
                <span>工作空间</span>
                <Tag color="blue">{isPlatform ? '平台端' : '租户端'}</Tag>
              </div>
              <div className={styles.accessItem}>
                <span>访问方式</span>
                <strong>
                  {currentUser?.mode === 'managed' ? '平台代管' : '正常登录'}
                </strong>
              </div>
              <div className={styles.accessItem}>
                <span>生效角色</span>
                <strong>{roleNames.length ? `${roleNames.length} 个` : '暂无'}</strong>
              </div>
              <div className={styles.accessItem}>
                <span>权限状态</span>
                <Tag color="green">实时生效</Tag>
              </div>
            </div>
            {authorizedPages[0]?.path
              ? (
                  <Button
                    block
                    className={styles.primaryEntry}
                    onClick={() => navigate(authorizedPages[0].path as string)}
                    type="primary"
                  >
                    进入
                    {authorizedPages[0].name}
                  </Button>
                )
              : null}
          </Card>
        </Col>
      </Row>
    </div>
  )
}
