"use client"

import { useState } from "react"
import { CheckIcon, CopyIcon, ExternalLinkIcon, KeyRoundIcon, ServerCogIcon, UserRoundIcon } from "lucide-react"

import { ContentModule, EmptyState, PageShell, RailopsButton, StatusTag } from "@railops/ui"

import { useAuth } from "@/components/auth-provider"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { useI18n } from "@/i18n/provider"

export default function EnterpriseServerConsolePage() {
  const t = useI18n()
  const { session } = useAuth()
  const [copiedCredential, setCopiedCredential] = useState<"username" | "password" | null>(null)
  const portal = session?.externalPortals?.find((item) => item.provider === "1panel")
  const embedded = portal?.embedMode === "iframe"
  const openPortal = () => {
    if (portal?.url) window.open(portal.url, "_blank", "noopener,noreferrer")
  }
  const copyCredential = async (credential: "username" | "password", value: string) => {
    try {
      await navigator.clipboard.writeText(value)
      setCopiedCredential(credential)
      window.setTimeout(() => setCopiedCredential(null), 1800)
    } catch {
      setCopiedCredential(null)
    }
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
          <div className="relative">
            <div className="mb-2 flex flex-wrap items-center gap-x-5 gap-y-2 rounded-md border border-[var(--railops-border)] bg-background/95 px-3 py-2 text-sm shadow-sm backdrop-blur">
              <span className="flex items-center gap-1.5 font-medium text-foreground">
                <KeyRoundIcon className="size-4 text-[var(--railops-primary)]" />
                1Panel 登录凭据
              </span>
              <span className="flex items-center gap-1.5 text-muted-foreground">
                <UserRoundIcon className="size-4" />
                账号 <span className="font-mono text-foreground">admin</span>
                <button
                  type="button"
                  title="复制账号"
                  aria-label="复制账号"
                  className="rounded p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  onClick={() => void copyCredential("username", "admin")}
                >
                  {copiedCredential === "username" ? <CheckIcon className="size-4 text-emerald-600" /> : <CopyIcon className="size-4" />}
                </button>
              </span>
              <span className="flex items-center gap-1.5 text-muted-foreground">
                密码 <span className="font-mono text-foreground">abcd@1234</span>
                <button
                  type="button"
                  title="复制密码"
                  aria-label="复制密码"
                  className="rounded p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  onClick={() => void copyCredential("password", "abcd@1234")}
                >
                  {copiedCredential === "password" ? <CheckIcon className="size-4 text-emerald-600" /> : <CopyIcon className="size-4" />}
                </button>
              </span>
            </div>
            <iframe
              src={portal.url}
              title={portal.name}
              className="h-[calc(100vh-244px)] min-h-[580px] w-full border-0 bg-white"
              referrerPolicy="no-referrer"
            />
          </div>
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
