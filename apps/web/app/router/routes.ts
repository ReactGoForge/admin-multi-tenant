import type { RouteConfig } from '@react-router/dev/routes'
import { index, route } from '@react-router/dev/routes'

/** 定义当前前端静态路由，后续动态路由能力也统一收口到 router 目录。 */
const routes = [
  index('router/modules/home.tsx'),
  route('login', 'router/modules/login.tsx'),
  route('403', 'pages/error/forbidden.tsx'),
  route('404', 'pages/error/not-found.tsx'),
  route('platform', 'router/modules/platform-layout.tsx', [
    index('router/modules/platform-index.tsx'),
    route('profile', 'pages/platform/profile/index.tsx'),
    route('tenants', 'pages/platform/tenants/index.tsx'),
    route('images', 'pages/platform/images/index.tsx'),
    route('system/employees', 'pages/platform/system/employees/index.tsx'),
    route('system/roles', 'pages/platform/system/roles/index.tsx'),
    route('system/menus', 'pages/platform/system/menus/index.tsx'),
    route('system/departments', 'pages/platform/system/departments/index.tsx'),
    route('system/basic', 'pages/platform/system/basic/index.tsx'),
    route('system/miniapp', 'pages/platform/system/miniapp/index.tsx'),
    route('system/fields', 'pages/platform/system/fields/index.tsx'),
    route('users', 'pages/platform/users/index.tsx'),
    route('logs/system', 'pages/platform/logs/system/index.tsx'),
    route('logs/operations', 'pages/platform/logs/operations/index.tsx'),
    route('logs/login', 'pages/platform/logs/login/index.tsx'),
  ]),
  route('tenant', 'router/modules/tenant-layout.tsx', [
    index('router/modules/tenant-index.tsx'),
    route('profile', 'pages/tenant/profile/index.tsx'),
    route('images', 'pages/tenant/images/index.tsx'),
    route('system/employees', 'pages/tenant/system/employees/index.tsx'),
    route('system/roles', 'pages/tenant/system/roles/index.tsx'),
    route('system/menus', 'pages/tenant/system/menus/index.tsx'),
    route('system/departments', 'pages/tenant/system/departments/index.tsx'),
    route('system/basic', 'pages/tenant/system/basic/index.tsx'),
    route('users', 'pages/tenant/users/index.tsx'),
    route('logs/operations', 'pages/tenant/logs/operations/index.tsx'),
    route('logs/login', 'pages/tenant/logs/login/index.tsx'),
  ]),
  route('*', 'router/modules/not-found.tsx'),
] satisfies RouteConfig

export default routes
