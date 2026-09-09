"use client"

import { EnterpriseAIFeatureGuard } from "@/components/enterprise/ai-feature-guard"
import { EnterpriseModelsPageContent } from "@/app/enterprise/models/page"

export default function EnterpriseUsagePage() {
  return (
    <EnterpriseAIFeatureGuard title="Usage">
      <EnterpriseModelsPageContent />
    </EnterpriseAIFeatureGuard>
  )
}
