import { AgentWorkbenchPage } from "./workbench-page"
import { EnterpriseAIFeatureGuard } from "@/components/enterprise/ai-feature-guard"
import { notFound } from "next/navigation"
import { isStaticExportRouteParam, STATIC_EXPORT_PARAM } from "@/lib/static-export-route"

export function generateStaticParams() {
  return [{ agentId: STATIC_EXPORT_PARAM }]
}

type PageProps = {
  params: Promise<{ agentId: string }>
}

export default async function Page({ params }: PageProps) {
  const { agentId } = await params
  if (isStaticExportRouteParam(agentId)) {
    notFound()
  }
  return (
    <EnterpriseAIFeatureGuard title="AI">
      <AgentWorkbenchPage />
    </EnterpriseAIFeatureGuard>
  )
}
