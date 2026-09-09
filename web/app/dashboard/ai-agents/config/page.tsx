"use client"

import { useSearchParams } from "next/navigation"

import { RouteAlias } from "@/components/layout/route-alias"
import { useI18n } from "@/i18n/provider"
import { buildEnterpriseAIPath } from "@/lib/enterprise-detail-route"

export default function DashboardAIAgentConfigPage() {
  const t = useI18n()
  const searchParams = useSearchParams()
  const agentId = Number(searchParams.get("agentId"))

  return <RouteAlias to={buildEnterpriseAIPath(agentId)} title={t("nav.aiAgents")} />
}
