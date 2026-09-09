import { EnterpriseWorkflowDetailPage } from "./template-page"
import { EnterpriseAIFeatureGuard } from "@/components/enterprise/ai-feature-guard"
import { notFound } from "next/navigation"
import { isStaticExportRouteParam, STATIC_EXPORT_PARAM } from "@/lib/static-export-route"

export function generateStaticParams() {
  return [{ workflowId: STATIC_EXPORT_PARAM }]
}

type PageProps = {
  params: Promise<{ workflowId: string }>
}

export default async function Page({ params }: PageProps) {
  const { workflowId } = await params
  if (isStaticExportRouteParam(workflowId)) {
    notFound()
  }
  return (
    <EnterpriseAIFeatureGuard title="AI Workflow">
      <EnterpriseWorkflowDetailPage />
    </EnterpriseAIFeatureGuard>
  )
}
