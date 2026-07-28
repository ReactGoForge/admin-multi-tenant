import { Card } from 'antd'

import { PageContainer } from '@/components/base/page-container'
import { ImageLibrary } from '@/components/composite/image-picker'

/** TenantImagesPage 管理当前租户图库并只读浏览平台共享图库。 */
export default function TenantImagesPage() {
  return (
    <PageContainer>
      <Card size="medium">
        <ImageLibrary mode="manage" workspace="tenant" />
      </Card>
    </PageContainer>
  )
}
