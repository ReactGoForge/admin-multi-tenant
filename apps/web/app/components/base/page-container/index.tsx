import type { ReactNode } from 'react'
import styles from './index.module.scss'

/** PageContainer 配置。 */
export interface PageContainerProps {
  /** 页面主体内容。 */
  children: ReactNode
}

/** 统一页面内容外层宽度和间距。 */
export function PageContainer({ children }: PageContainerProps) {
  return <div className={styles.pageContainer}>{children}</div>
}
