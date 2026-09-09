"use client"

import { useSearchParams } from "next/navigation"

import { AgentWorkbenchPage } from "./[agentId]/workbench-page"
import { EnterpriseAiServiceCenter } from "./ai-service-center"
import { EnterpriseAIFeatureGuard } from "@/components/enterprise/ai-feature-guard"
import { normalizeEnterpriseDetailId } from "@/lib/enterprise-detail-route"

export default function EnterpriseAiPage() {
  const searchParams = useSearchParams()
  const agentId = normalizeEnterpriseDetailId(searchParams.get("agentId"))
  return (
    <EnterpriseAIFeatureGuard title="AI">
      {agentId > 0
        ? <AgentWorkbenchPage agentId={agentId} />
        : <EnterpriseAiServiceCenter />}
    </EnterpriseAIFeatureGuard>
  )
}
