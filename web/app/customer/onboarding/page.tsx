"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { Building2Icon, Loader2Icon, ScanLineIcon, ShieldCheckIcon } from "lucide-react"
import { toast } from "sonner"

import { RailopsButton } from "@railops/ui"
import { Input } from "antd"

import { useAuth } from "@/components/auth-provider"
import { bindCustomerDevice } from "@/lib/api/customer-devices"
import { useI18n } from "@/i18n/provider"

export default function CustomerOnboardingPage() {
  const router = useRouter()
  const { ready, session, refreshProfile } = useAuth()
  const t = useI18n()
  const [serviceCode, setServiceCode] = useState("")
  const [binding, setBinding] = useState(false)

  useEffect(() => {
    if (ready && session?.featureFlags?.device === false) {
      router.replace("/customer/chat")
    }
  }, [ready, router, session?.featureFlags?.device])

  if (ready && session?.featureFlags?.device === false) {
    return null
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!serviceCode.trim() || binding) return
    setBinding(true)
    try {
      const result = await bindCustomerDevice({
        serviceCode: serviceCode.trim(),
      })
      await refreshProfile()
      toast.success(t("customerOnboarding.pageSuccess"))
      router.replace(`/customer/devices?deviceId=${result.deviceId}&bound=1`)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("customerOnboarding.pageError"))
    } finally {
      setBinding(false)
    }
  }

  return (
    <div className="mx-auto w-full max-w-5xl px-5 py-10 lg:px-10 lg:py-14">
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_300px]">
        <section className="overflow-hidden rounded-md border border-border bg-card">
          <div className="border-b border-border px-6 py-6 lg:px-8">
            <div className="flex items-center gap-2 text-sm font-semibold text-primary">
              <ShieldCheckIcon className="size-4" />
              {t("customerOnboarding.pageStatus")}
            </div>
            <h1 className="mt-3 text-3xl font-semibold text-foreground">{t("customerOnboarding.pageTitle")}</h1>
          </div>

          <div className="p-6 lg:p-8">
            <form className="space-y-6" onSubmit={submit}>
              <div className="space-y-2">
                <label htmlFor="onboarding-service-code" className="text-sm font-medium text-foreground">{t("customerOnboarding.serviceCodeLabel")}</label>
                <div className="relative">
                  <ScanLineIcon className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    id="onboarding-service-code"
                    value={serviceCode}
                    onChange={(event) => setServiceCode(event.target.value)}
                    className="h-11 pl-9 uppercase"
                    placeholder={t("customerOnboarding.serviceCodePlaceholder")}
                    autoFocus
                    required
                  />
                </div>
              </div>
              <RailopsButton variant="primary" htmlType="submit" size="large" className="h-11" disabled={binding || !serviceCode.trim()}>
                {binding ? <Loader2Icon className="animate-spin" /> : <Building2Icon />}
                {binding ? t("customerOnboarding.pageSubmitting") : t("customerOnboarding.pageSubmit")}
              </RailopsButton>
            </form>
          </div>
        </section>

        <aside className="rounded-md border border-border bg-muted/30 p-6 lg:p-7">
          <div className="text-sm font-semibold text-foreground">{t("customerOnboarding.currentAccount")}</div>
          <div className="mt-2 text-sm text-muted-foreground">{session?.user?.nickname || session?.user?.username || t("customerOnboarding.accountFallback")}</div>
          <div className="mt-6 text-sm font-semibold text-foreground">{t("customerOnboarding.rulesTitle")}</div>
          <ol className="mt-3 space-y-3 text-sm leading-6 text-muted-foreground">
            <li>1. {t("customerOnboarding.rule1")}</li>
            <li>2. {t("customerOnboarding.rule2")}</li>
            <li>3. {t("customerOnboarding.rule3")}</li>
          </ol>
        </aside>
      </div>
    </div>
  )
}
