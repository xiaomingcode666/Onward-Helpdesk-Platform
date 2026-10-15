"use client"

import { ExternalLinkIcon, ServerCogIcon } from "lucide-react"

import { ContentModule, EmptyState, PageShell, RailopsButton, StatusTag } from "@railops/ui"

import { useAuth } from "@/components/auth-provider"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { useI18n } from "@/i18n/provider"

export default function EnterpriseServerConsolePage() {
  const t = useI18n()
  const { session } = useAuth()
  const portal = session?.externalPortals?.find((item) => item.provider === "1panel")
  const embedded = portal?.embedMode === "iframe"
  const openPortal = () => {
    if (portal?.url) window.open(portal.url, "_blank", "noopener,noreferrer")
  }

  return (
    <PageShell
      title={t("serverConsole.title")}
      description={t("serverConsole.description")}
      breadcrumb={useRouteBreadcrumbItems()}
    >
      {!portal ? (
        <EmptyState
          title={t("serverConsole.missingTitle")}
          description={t("serverConsole.missingDescription")}
        />
      ) : embedded ? (
        <ContentModule
          title={portal.name}
          note={t("serverConsole.embedHint")}
          extra={<StatusTag tone="success">{t("serverConsole.iframeMode")}</StatusTag>}
        >
          <iframe
            src={portal.url}
            title={portal.name}
            className="h-[calc(100vh-244px)] min-h-[580px] w-full border-0 bg-white"
            referrerPolicy="no-referrer"
          />
        </ContentModule>
      ) : (
        <ContentModule
          title={portal.name}
          note={portal.url}
          extra={<StatusTag tone="blue">{t("serverConsole.externalMode")}</StatusTag>}
        >
          <div className="flex min-h-64 flex-col items-center justify-center gap-5 px-6 py-10 text-center">
            <span className="grid size-14 place-items-center rounded-md bg-[var(--railops-primary-bg)] text-[var(--railops-primary)]">
              <ServerCogIcon className="size-7" />
            </span>
            <p className="max-w-xl text-sm text-muted-foreground">{t("serverConsole.openHint")}</p>
            <RailopsButton variant="primary" onClick={openPortal}>
              <ExternalLinkIcon className="size-4" />
              {t("serverConsole.open", { name: portal.name })}
            </RailopsButton>
          </div>
        </ContentModule>
      )}
    </PageShell>
  )
}
