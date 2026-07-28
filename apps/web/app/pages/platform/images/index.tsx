import { Card } from 'antd'

import { PageContainer } from '@/components/base/page-container'
import { ImageLibrary } from '@/components/composite/image-picker'

/** PlatformImagesPage 集中管理平台图库及指定租户图库。 */
export default function PlatformImagesPage() {
  return (
    <PageContainer>
      <Card size="medium">
        <ImageLibrary mode="manage" workspace="platform" />
      </Card>
    </PageContainer>
  )
}
