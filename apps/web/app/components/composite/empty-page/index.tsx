import { Card, Result, Typography } from 'antd'
import { useLocation } from 'react-router'

import { PageContainer } from '@/components/base/page-container'

/** EmptyPage 配置。 */
export interface EmptyPageProps {
  /** 空页面主标题。 */
  title: string
}

/** EmptyPage 展示尚未实现功能的统一占位内容和当前路由。 */
export function EmptyPage({ title }: EmptyPageProps) {
  const location = useLocation()

  return (
    <PageContainer>
      <Card variant="borderless" size="medium">
        <Result
          extra={(
            <Typography.Text code>
              当前路由：
              {location.pathname}
            </Typography.Text>
          )}
          status="info"
          subTitle="功能开发中，当前阶段仅保留菜单、路由和页面入口。"
          title={title}
        />
      </Card>
    </PageContainer>
  )
}
