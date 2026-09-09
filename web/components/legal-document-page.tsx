"use client"

import Link from "next/link"

import { AppLogoMark } from "@/components/brand-logo"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useAppLocale, useI18n } from "@/i18n/provider"
import enUSMessages from "@/messages/en-US.json"
import esESMessages from "@/messages/es-ES.json"
import zhCNMessages from "@/messages/zh-CN.json"

type LegalPageType = "terms" | "privacy"

type LegalSection = {
  title: string
  body: string
}

type LegalDocument = {
  title: string
  updatedAt: string
  relatedLabel: string
  relatedLink: string
  sections: LegalSection[]
}

const messages = {
  "zh-CN": zhCNMessages,
  "en-US": enUSMessages,
  "es-ES": esESMessages,
}

export function LegalDocumentPage({ type }: { type: LegalPageType }) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const document = messages[locale].legal[type] as LegalDocument
  const relatedHref = type === "terms" ? "/legal/privacy" : "/legal/terms"
  const legalEntityName = process.env.NEXT_PUBLIC_LEGAL_ENTITY_NAME?.trim() || t("legal.privacyOverview.entityFallback")
  const privacyContactEmail = process.env.NEXT_PUBLIC_PRIVACY_CONTACT_EMAIL?.trim() || t("legal.privacyOverview.contactFallback")
  const dataStorageRegion = process.env.NEXT_PUBLIC_DATA_STORAGE_REGION?.trim() || t("legal.privacyOverview.storageRegionFallback")

  return (
    <main className="min-h-svh bg-[var(--railops-layout-background)] px-5 py-6 text-[var(--railops-text)] md:px-10">
      <div className="mx-auto flex w-full max-w-4xl flex-col gap-4">
        <header className="flex items-center gap-4">
          <div className="flex items-center gap-2 font-medium">
            <AppLogoMark alt={t("app.brand")} className="size-8" imageClassName="p-0.5" priority />
            <span>{t("app.brand")}</span>
          </div>
        </header>

        <Card className="bg-[var(--railops-surface)]">
          <CardHeader className="gap-2 border-b border-[var(--railops-border-light)] px-5 py-5 md:px-6">
            <div className="flex flex-col gap-2">
              <CardTitle className="text-rhd-4xl font-semibold leading-[30px]">
                {document.title}
              </CardTitle>
            </div>
            <p className="text-xs text-[var(--railops-text-secondary)]">{document.updatedAt}</p>
          </CardHeader>
          <CardContent className="space-y-6 px-5 py-6 md:px-6">
            {type === "privacy" ? (
              <dl className="grid gap-4 border-y border-[var(--railops-border-light)] py-4 text-xs sm:grid-cols-3">
                <div><dt className="text-xs text-[var(--railops-text-secondary)]">{t("legal.privacyOverview.controller")}</dt><dd className="mt-1 font-medium leading-5">{legalEntityName}</dd></div>
                <div><dt className="text-xs text-[var(--railops-text-secondary)]">{t("legal.privacyOverview.contact")}</dt><dd className="mt-1 break-words font-medium leading-5">{privacyContactEmail}</dd></div>
                <div><dt className="text-xs text-[var(--railops-text-secondary)]">{t("legal.privacyOverview.storageRegion")}</dt><dd className="mt-1 font-medium leading-5">{dataStorageRegion}</dd></div>
              </dl>
            ) : null}
            {document.sections.map((section) => (
              <section key={section.title} className="space-y-2">
                <h2 className="text-base font-semibold leading-6">
                  {section.title}
                </h2>
                <p className="text-xs leading-6 text-[var(--railops-text-secondary)]">
                  {section.body}
                </p>
              </section>
            ))}
          </CardContent>
        </Card>

        <footer className="flex flex-wrap justify-end gap-x-4 gap-y-2 text-xs text-[var(--railops-text-secondary)]">
          {type === "privacy" ? <Link href="/legal/account-deletion" className="font-medium text-foreground underline-offset-4 hover:underline">{t("legal.accountDeletion.link")}</Link> : null}
          <p>
            {document.relatedLabel}{" "}
            <Link
              href={relatedHref}
              className="font-medium text-foreground underline-offset-4 hover:underline"
            >
              {document.relatedLink}
            </Link>
          </p>
        </footer>
      </div>
    </main>
  )
}
