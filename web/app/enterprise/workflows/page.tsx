import { EnterpriseAIFeatureGuard } from "@/components/enterprise/ai-feature-guard"
import { RouteAlias } from "@/components/layout/route-alias"

export default function EnterpriseWorkflowsAliasPage() {
  return (
    <EnterpriseAIFeatureGuard title="AI Workflow">
      <RouteAlias to="/enterprise/workflow" />
    </EnterpriseAIFeatureGuard>
  )
}
