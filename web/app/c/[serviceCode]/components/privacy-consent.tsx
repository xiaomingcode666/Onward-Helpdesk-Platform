"use client"

import { useState, useCallback } from "react"
import {
  ShieldCheckIcon,
  InfoIcon,
  CookieIcon,
  BarChart3Icon,
  MegaphoneIcon,
  Loader2Icon,
  ChevronDownIcon,
  ChevronUpIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Switch } from "@/components/ui/switch"
import { Separator } from "@/components/ui/separator"
import { useI18n } from "@/i18n/provider"

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface CookieConsent {
  required: boolean
  analytics: boolean
  marketing: boolean
}

interface PrivacyConsentProps {
  /** Called when user agrees and continues */
  onAgree?: (consent: CookieConsent) => void | Promise<void>
  /** Override the consent defaults */
  defaultConsent?: Partial<CookieConsent>
  error?: string
}

// ---------------------------------------------------------------------------
// Switch row component
// ---------------------------------------------------------------------------

interface SwitchRowProps {
  icon: React.ReactNode
  label: string
  description: string
  checked: boolean
  disabled?: boolean
  onToggle: (checked: boolean) => void
}

function SwitchRow({ icon, label, description, checked, disabled, onToggle }: SwitchRowProps) {
  return (
    <div className="flex items-start gap-3 py-3">
      <div className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted">
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-2">
          <p className="text-sm font-medium">{label}</p>
          <Switch
            checked={checked}
            disabled={disabled}
            onCheckedChange={onToggle}
            aria-label={label}
          />
        </div>
        <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Collapsible section
// ---------------------------------------------------------------------------

function CollapsibleSection({
  title,
  children,
  defaultOpen = false,
}: {
  title: string
  children: React.ReactNode
  defaultOpen?: boolean
}) {
  const [open, setOpen] = useState(defaultOpen)

  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center justify-between py-2 text-sm font-medium text-muted-foreground"
      >
        {title}
        {open ? (
          <ChevronUpIcon className="size-4" />
        ) : (
          <ChevronDownIcon className="size-4" />
        )}
      </button>
      {open && <div className="pb-2 text-xs leading-relaxed text-muted-foreground">{children}</div>}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export function PrivacyConsent({ onAgree, defaultConsent, error }: PrivacyConsentProps) {
  const t = useI18n()

  const [consent, setConsent] = useState<CookieConsent>({
    required: true,
    analytics: defaultConsent?.analytics ?? false,
    marketing: defaultConsent?.marketing ?? false,
  })

  const [showFullPolicy, setShowFullPolicy] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const handleToggle = useCallback(
    (key: keyof CookieConsent) => (checked: boolean) => {
      if (key === "required") return // always on
      setConsent((prev) => ({ ...prev, [key]: checked }))
    },
    []
  )

  const handleAgree = useCallback(async () => {
    if (!onAgree || submitting) return
    setSubmitting(true)
    try {
      await onAgree(consent)
    } finally {
      setSubmitting(false)
    }
  }, [consent, onAgree, submitting])

  return (
    <div className="flex flex-col">
      {/* Header */}
      <div className="flex items-center gap-3 border-b px-4 py-4">
        <div className="flex size-10 items-center justify-center rounded-full bg-primary/10">
          <ShieldCheckIcon className="size-5 text-primary" />
        </div>
        <div>
          <h2 className="text-base font-medium">
            {t("privacyConsent.title")}
          </h2>
        </div>
      </div>

      {/* Policy summary */}
      <div className="space-y-3 px-4 py-4">
        <Card>
          <CardContent className="space-y-2 p-4 text-sm leading-relaxed">
            <p>
              {t("privacyConsent.policySummary")}
            </p>
            <button
              type="button"
              onClick={() => setShowFullPolicy((o) => !o)}
              className="flex items-center gap-1 text-xs text-primary"
            >
              <InfoIcon className="size-3" />
              {showFullPolicy
                ? t("privacyConsent.hidePolicy")
                : t("privacyConsent.viewPolicy")}
            </button>
          </CardContent>
        </Card>

        {/* Full policy (collapsible) */}
        {showFullPolicy && (
          <Card>
            <CardContent className="space-y-2 p-4 text-xs leading-relaxed text-muted-foreground">
              <p>
                <strong>{t("privacyConsent.dataCollected")}</strong>
              </p>
              <ul className="list-inside list-disc space-y-1">
                <li>{t("privacyConsent.dataDevice")}</li>
                <li>{t("privacyConsent.dataUsageRecord")}</li>
                <li>{t("privacyConsent.dataCommunication")}</li>
              </ul>
              <Separator />
              <p>
                <strong>{t("privacyConsent.dataUsageTitle")}</strong>
              </p>
              <ul className="list-inside list-disc space-y-1">
                <li>{t("privacyConsent.usageDiagnosis")}</li>
                <li>{t("privacyConsent.usageImprove")}</li>
                <li>{t("privacyConsent.usageCompliance")}</li>
              </ul>
              <Separator />
              <p>
                <strong>{t("privacyConsent.thirdParty")}</strong>
                {t("privacyConsent.thirdPartyDesc")}
              </p>
            </CardContent>
          </Card>
        )}

        <Separator />

        {/* Cookie / Consent options */}
        <div>
          <h3 className="flex items-center gap-1.5 text-sm font-medium">
            <CookieIcon className="size-4 text-primary" />
            {t("privacyConsent.cookiePreferences")}
          </h3>

          <div className="divide-y">
            <SwitchRow
              icon={<ShieldCheckIcon className="size-4 text-primary" />}
              label={t("privacyConsent.cookieRequired")}
              description={t("privacyConsent.cookieRequiredDesc")}
              checked={consent.required}
              disabled
              onToggle={() => {}}
            />

            <SwitchRow
              icon={<BarChart3Icon className="size-4 text-primary" />}
              label={t("privacyConsent.cookieAnalytics")}
              description={t("privacyConsent.cookieAnalyticsDesc")}
              checked={consent.analytics}
              onToggle={handleToggle("analytics")}
            />

            <SwitchRow
              icon={<MegaphoneIcon className="size-4 text-primary" />}
              label={t("privacyConsent.cookieMarketing")}
              description={t("privacyConsent.cookieMarketingDesc")}
              checked={consent.marketing}
              onToggle={handleToggle("marketing")}
            />
          </div>
        </div>

        <Separator />

        {/* Data usage description */}
        <CollapsibleSection
          title={t("privacyConsent.dataUsageDetail")}
          defaultOpen={false}
        >
          <div className="space-y-2">
            <p>
              <strong>{t("privacyConsent.dataRetention")}</strong>
              {t("privacyConsent.dataRetentionDesc")}
            </p>
            <p>
              <strong>{t("privacyConsent.dataAccess")}</strong>
              {t("privacyConsent.dataAccessDesc")}
            </p>
            <p>
              <strong>{t("privacyConsent.contact")}</strong>
              {t("privacyConsent.contactDesc")}
            </p>
          </div>
        </CollapsibleSection>
      </div>

      {/* Agree button */}
      <div className="border-t px-4 py-4">
        {error ? (
          <p className="mb-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
            {error}
          </p>
        ) : null}
        <Button
          type="button"
          size="lg"
          className="w-full"
          onClick={handleAgree}
          disabled={submitting}
        >
          {submitting ? <Loader2Icon className="mr-2 size-4 animate-spin" /> : <ShieldCheckIcon className="mr-2 size-4" />}
          {submitting ? t("privacyConsent.saving") : t("privacyConsent.agreeAndContinue")}
        </Button>
        <p className="mt-2 text-center text-rhd-2xs text-muted-foreground">
          {t("privacyConsent.agreeHint")}
        </p>
      </div>
    </div>
  )
}
