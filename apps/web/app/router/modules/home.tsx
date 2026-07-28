import type { Route } from './+types/home'
import HomePage from '@/pages/home'

/** 定义工作台路由的浏览器标题和页面描述。 */
export function meta(_: Route.MetaArgs) {
  return [
    { title: 'ReactGoForge Admin 工作台' },
    { name: 'description', content: 'ReactGoForge Admin 管理控制台' },
  ]
}

export default HomePage
