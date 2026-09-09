"use client"

import Link from "next/link"
import { ShieldCheckIcon, Trash2Icon } from "lucide-react"

import { AppLogoMark } from "@/components/brand-logo"
import { Button } from "@/components/ui/button"
import { useI18n } from "@/i18n/provider"

export default function AccountDeletionPage() {
  const t = useI18n()
  const privacyContactEmail = process.env.NEXT_PUBLIC_PRIVACY_CONTACT_EMAIL?.trim() || ""

  return (
    <main className="min-h-svh bg-[var(--railops-layout-background)] px-5 py-6 text-[var(--railops-text)] md:px-10">
      <div className="mx-auto w-full max-w-3xl">
        <header className="flex items-center gap-2 font-medium"><AppLogoMark alt="RemoteHelpDesk" className="size-8" imageClassName="p-0.5" priority /><span>RemoteHelpDesk</span></header>
        <article className="mt-5 rounded-lg bg-[var(--railops-surface)] px-5 py-6 shadow-[var(--railops-card-shadow)] md:px-6">
          <div className="flex items-start gap-3"><span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-[var(--railops-error-bg)] text-[var(--railops-error)]"><Trash2Icon className="size-5" /></span><div><h1 className="text-rhd-4xl font-semibold leading-[30px]">{t("legal.accountDeletion.title")}</h1><p className="mt-2 text-xs leading-6 text-[var(--railops-text-secondary)]">{t("legal.accountDeletion.description")}</p></div></div>
          <ol className="mt-6 space-y-3 text-xs leading-6">
            <li><strong>{t("legal.accountDeletion.stepLogin")}</strong> {t("legal.accountDeletion.stepLoginBody")}</li>
            <li><strong>{t("legal.accountDeletion.stepOpenProfile")}</strong> {t("legal.accountDeletion.stepOpenProfileBody")}</li>
            <li><strong>{t("legal.accountDeletion.stepConfirm")}</strong> {t("legal.accountDeletion.stepConfirmBody")}</li>
            <li><strong>{t("legal.accountDeletion.stepReceipt")}</strong> {t("legal.accountDeletion.stepReceiptBody")}</li>
          </ol>
          <div className="mt-6 border-y border-[var(--railops-border-light)] py-4 text-xs leading-6 text-[var(--railops-text-secondary)]"><p>{t("legal.accountDeletion.retentionNote")}</p></div>
          <div className="mt-6 flex flex-wrap items-center gap-3"><Button render={<Link href="/mobile?state=login" />}><ShieldCheckIcon className="size-4" />{t("legal.accountDeletion.start")}</Button><Button variant="outline" render={<Link href="/legal/privacy" />}>{t("legal.accountDeletion.viewPrivacy")}</Button></div>
          {privacyContactEmail ? <p className="mt-5 text-xs text-[var(--railops-text-secondary)]">{t("legal.accountDeletion.contact", { email: privacyContactEmail })}</p> : null}
        </article>
      </div>
    </main>
  )
}
