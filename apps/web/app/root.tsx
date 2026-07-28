import type { Route } from './+types/root'
import { QueryClientProvider } from '@tanstack/react-query'
import { App as AntdApp, ConfigProvider } from 'antd'
import { useEffect } from 'react'
import {
  Links,
  Meta,
  Outlet,
  Scripts,
  ScrollRestoration,
  useLocation,
} from 'react-router'
import { antdConfig } from '@/config/antd'
import { DictionaryProvider } from '@/contexts/dictionary'
import { getRouteMetaByPath } from '@/navigation'
import ErrorPage from '@/pages/error'
import { appQueryClient } from '@/query/query-client'
import { useAuthStore } from '@/stores/auth'

import { useBrandStore } from '@/stores/brand'
import 'antd/dist/reset.css'
import './app.css'

/** 声明页面级外部资源，目前用于加载字体资源。 */
export const links: Route.LinksFunction = () => [
  { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
  {
    rel: 'preconnect',
    href: 'https://fonts.gstatic.com',
    crossOrigin: 'anonymous',
  },
  {
    rel: 'stylesheet',
    href: 'https://fonts.googleapis.com/css2?family=Inter:ital,opsz,wght@0,14..32,100..900;1,14..32,100..900&display=swap',
  },
]

/** 渲染 React Router 文档外壳，统一注入 head、脚本和滚动恢复。 */
export function Layout({
  children,
}: {
  /** React Router 当前生成的完整应用内容。 */
  children: React.ReactNode
}) {
  return (
    <html lang="zh-CN">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <Meta />
        <Links />
      </head>
      <body>
        {children}
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  )
}

/** 注入 antd 全局配置、中文语言包和主题 token，不承载具体页面 UI。 */
export default function App() {
  const location = useLocation()
  const currentUser = useAuthStore(state => state.currentUser)
  const menus = currentUser?.menus ?? []
  const { name, refresh } = useBrandStore()
  // 应用首次挂载时加载公共平台品牌；请求失败由 Store 保留默认品牌且仍结束加载状态。
  useEffect(() => {
    const controller = new AbortController()
    void refresh(controller.signal)
    return () => controller.abort()
  }, [refresh])
  // 当前路径、授权菜单或平台名称变化时同步浏览器标题，未登记路由使用导航工具的默认标题。
  useEffect(() => {
    const pageTitle
      = location.pathname === '/login'
        ? '登录'
        : getRouteMetaByPath(location.pathname, menus).title
    document.title = pageTitle ? `${pageTitle} - ${name}` : name
  }, [location.pathname, menus, name])
  return (
    <QueryClientProvider client={appQueryClient}>
      <ConfigProvider {...antdConfig}>
        <AntdApp>
          <DictionaryProvider>
            <Outlet />
          </DictionaryProvider>
        </AntdApp>
      </ConfigProvider>
    </QueryClientProvider>
  )
}

/** React Router 约定错误边界：这里只转发到独立错误页，不写页面 UI。 */
export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  return <ErrorPage error={error} />
}
