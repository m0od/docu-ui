import { Separator } from '@/components/ui/separator'
import { DocoCDForm } from '@/features/env-files/components/doco-cd-form'
import { SharedWebhookForm } from '@/features/env-files/components/shared-webhook-form'
import { ContentSection } from '../components/content-section'

// Docker Compose needs no settings here: it only needs the socket mounted.
export function SettingsApply() {
  return (
    <ContentSection
      title='Apply'
      desc='How saved env files reach the containers. Each file picks Doco-CD, Docker Compose or a webhook on its own page.'
    >
      <div className='grid gap-6'>
        <DocoCDForm />
        <Separator />
        <SharedWebhookForm />
      </div>
    </ContentSection>
  )
}
