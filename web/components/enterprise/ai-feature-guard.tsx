"use client"

import type { ReactNode } from "react"

import { LoadingState, PageShell } from "@railops/ui"

import { useAuth } from "@/components/auth-provider"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { ForbiddenState } from "@/components/shared/error-states"
import { useI18n } from "@/i18n/provider"

export function EnterpriseAIFeatureGuard({
  children,
  title,
}: {
  children: ReactNode
  title: string
}) {
  const t = useI18n()
  const breadcrumb = useRouteBreadcrumbItems()
  const { ready, session } = useAuth()

  if (!ready) {
    return (
      <PageShell title={title} breadcrumb={breadcrumb}>
        <LoadingState description={t("common.loadingData")} />
      </PageShell>
    )
  }

  if (session?.featureFlags?.ai === false) {
    return (
      <PageShell title={title} breadcrumb={breadcrumb}>
        <ForbiddenState
          description={t("enterpriseDiagnosis.aiDisabledDescription")}
          action={{ label: t("common.backHome"), href: "/enterprise" }}
        />
      </PageShell>
    )
  }

  return <>{children}</>
}
