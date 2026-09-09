"use client"

import { useState, type FormEvent } from "react"
import {
  ArrowLeftIcon,
  BadgeCheckIcon,
  EyeIcon,
  EyeOffIcon,
  KeyRoundIcon,
  Loader2Icon,
  LockKeyholeIcon,
  LogInIcon,
  MailIcon,
  UserPlusIcon,
  UserRoundIcon,
  WrenchIcon,
} from "lucide-react"
import { toast } from "sonner"

import { useAuth } from "@/components/auth-provider"
import { MobileLanguageSwitch } from "@/components/remote-helpdesk/mobile-language-switch"
import { useI18n } from "@/i18n/provider"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  loginWithPassword,
  registerCustomerAccount,
  sendAuthVerificationCode,
  verifyCustomerRegistration,
  type CustomerRegistrationContext,
  type CustomerRegistrationMethod,
} from "@/lib/api/auth"
import { SelectField, UnderlineTabs, type RailopsTabItem } from "@railops/ui"
import { cn } from "@/lib/utils"

type MobileCustomerLoginProps = {
  onBack?: () => void
  initialMode?: "account" | "register"
  initialServiceCode?: string
  initialInviteCode?: string
}

const customerEntryLoginI18nPrefix = "customerEntryExtract.login."
type MobileCustomerLoginT = ReturnType<typeof useI18n>
const ml = (t: MobileCustomerLoginT, key: string, values?: Record<string, string | number>) => t(`${customerEntryLoginI18nPrefix}${key}`, values)

function ContextSummary({ context }: { context: CustomerRegistrationContext }) {
  const t = useI18n()
  return (
    <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3 text-rhd-sm leading-5">
      <div className="flex items-center gap-2 font-semibold text-emerald-700">
        <BadgeCheckIcon className="size-3.5" />
        {ml(t, "authConfirmed")}
      </div>
      <dl className="mt-3 grid gap-2 text-zinc-500">
        {context.tenant ? <div className="flex justify-between gap-4"><dt>{ml(t, "enterprise")}</dt><dd className="truncate text-right font-medium text-zinc-900">{context.tenant.name}</dd></div> : null}
        {context.method === "visitor" && context.email ? <div className="flex justify-between gap-4"><dt>{ml(t, "emailLabel")}</dt><dd className="truncate text-right font-medium text-zinc-900">{context.email}</dd></div> : null}
        {context.customerOrg ? <div className="flex justify-between gap-4"><dt>{ml(t, "customerOrg")}</dt><dd className="truncate text-right font-medium text-zinc-900">{context.customerOrg.name}</dd></div> : null}
        {context.product ? <div className="flex justify-between gap-4"><dt>{ml(t, "product")}</dt><dd className="truncate text-right font-medium text-zinc-900">{context.product.name}</dd></div> : null}
        {context.device ? <div className="flex justify-between gap-4"><dt>{ml(t, "device")}</dt><dd className="truncate text-right font-mono font-medium text-zinc-900">{context.device.deviceNo}</dd></div> : null}
      </dl>
    </div>
  )
}

export function MobileCustomerLogin({
  onBack,
  initialMode = "account",
  initialServiceCode = "",
  initialInviteCode = "",
}: MobileCustomerLoginProps) {
  const { refreshProfile } = useAuth()
  const t = useI18n()
  const initialMethod: CustomerRegistrationMethod = initialServiceCode ? "service_code" : "invite"
  const [mode, setMode] = useState<"account" | "register">(
    initialServiceCode || initialInviteCode ? "register" : initialMode,
  )
  const mobileModeTabs: RailopsTabItem[] = [
    { value: "account", label: ml(t, "tabLogin") },
    { value: "register", label: ml(t, "tabRegister") },
  ]
  const registrationMethodOptions = [
    { value: "invite", label: ml(t, "inviteMethod") },
    { value: "service_code", label: ml(t, "bindDeviceMethod") },
    { value: "visitor", label: ml(t, "visitorMethod") },
  ]
  const [method, setMethod] = useState<CustomerRegistrationMethod>(initialMethod)
  const [credential, setCredential] = useState(initialServiceCode || initialInviteCode)
  const [registrationContext, setRegistrationContext] = useState<CustomerRegistrationContext | null>(null)
  const [passwordVisible, setPasswordVisible] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [verifying, setVerifying] = useState(false)
  const [sendingVerificationCode, setSendingVerificationCode] = useState(false)
  const [error, setError] = useState("")

  function changeMode(nextMode: "account" | "register") {
    setMode(nextMode)
    setError("")
  }

  function changeMethod(nextMethod: CustomerRegistrationMethod) {
    setMethod(nextMethod)
    setCredential(nextMethod === "service_code" ? initialServiceCode : nextMethod === "invite" ? initialInviteCode : "")
    setRegistrationContext(null)
    setError("")
  }

  async function submitLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting) return

    const form = new FormData(event.currentTarget)
    const username = form.get("username")?.toString().trim() ?? ""
    const password = form.get("password")?.toString() ?? ""
    setSubmitting(true)
    setError("")
    try {
      const session = await loginWithPassword({ username, password, domainType: "customer" })
      if (session.domainType !== "customer") {
        throw new Error(ml(t, "wrongAccountError"))
      }
      await refreshProfile()
    } catch (value) {
      setError(value instanceof Error && value.message ? value.message : ml(t, "loginFailedError"))
    } finally {
      setSubmitting(false)
    }
  }

  async function verifyCredential(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const value = credential.trim()
    if (!value || verifying) return
    setVerifying(true)
    setError("")
    try {
      setRegistrationContext(await verifyCustomerRegistration(method, value))
    } catch (failure) {
      setRegistrationContext(null)
      setError(failure instanceof Error && failure.message
        ? failure.message
        : method === "invite" ? ml(t, "inviteCodeInvalidError") : method === "visitor" ? ml(t, "visitorEmailInvalidError") : ml(t, "serviceCodeInvalidError"))
    } finally {
      setVerifying(false)
    }
  }

  async function sendCustomerRegisterCode(email: string) {
    if (!email.trim() || sendingVerificationCode) return
    setSendingVerificationCode(true)
    setError("")
    try {
      await sendAuthVerificationCode("customer_register", email.trim())
      toast.success(ml(t, "verificationCodeSent"))
    } catch (failure) {
      setError(failure instanceof Error && failure.message ? failure.message : ml(t, "verificationCodeSendFailed"))
    } finally {
      setSendingVerificationCode(false)
    }
  }

  async function submitRegistration(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!registrationContext || submitting) return
    const form = new FormData(event.currentTarget)
    const username = form.get("username")?.toString().trim() ?? ""
    const email = form.get("email")?.toString().trim() ?? ""
    const password = form.get("password")?.toString() ?? ""
    const confirmPassword = form.get("confirmPassword")?.toString() ?? ""
    const verificationCode = form.get("verificationCode")?.toString().trim() ?? ""
    if (password !== confirmPassword) {
      setError(ml(t, "passwordMismatchError"))
      return
    }
    if (password.length < 8) {
      setError(ml(t, "passwordTooShortError"))
      return
    }
    if (method === "visitor" && !verificationCode) {
      setError(ml(t, "verificationCodeRequired"))
      return
    }

    setSubmitting(true)
    setError("")
    try {
      const session = await registerCustomerAccount({
        method,
        credential: credential.trim(),
        username,
        displayName: registrationContext.displayName || username,
        email,
        password,
        verificationCode,
      })
      if (session.domainType !== "customer") {
        throw new Error(ml(t, "sessionAbnormalError"))
      }
      await refreshProfile()
    } catch (failure) {
      setError(failure instanceof Error && failure.message ? failure.message : ml(t, "registerFailedError"))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main className="rhd-mobile-touch-surface rhd-railops-mobile-auth min-h-svh bg-[#f3f6fb] text-[#172033] lg:grid lg:grid-cols-[minmax(320px,42vw)_minmax(360px,1fr)]">
      <section className="relative z-20 h-[148px] bg-[#172033] sm:h-[176px] lg:h-svh lg:min-h-[560px]">
        <div className="absolute inset-0 bg-[url('/images/login-service-operations.jpg')] bg-cover bg-center" aria-hidden="true" />
        <div className="absolute inset-0 bg-[#0f172a]/68" aria-hidden="true" />
        <div className="relative mx-auto flex h-full w-full max-w-sm flex-col justify-between px-5 pb-5 pt-[max(12px,env(safe-area-inset-top))] text-white sm:max-w-md sm:px-6 lg:max-w-none lg:px-10 lg:pb-10 lg:pt-8">
          <div className="flex items-start justify-between gap-3">
            {onBack ? (
              <Button type="button" variant="ghost" size="icon" className="-ml-2 size-8 rounded-md text-white hover:bg-white/15 hover:text-white" onClick={onBack} aria-label={ml(t, "back")} title={ml(t, "back")}>
                <ArrowLeftIcon className="size-4" />
              </Button>
            ) : <span className="size-8" aria-hidden="true" />}
            <MobileLanguageSwitch />
          </div>
          <div>
            <p className="text-rhd-sm font-medium text-white/72">RemoteHelpDesk</p>
            <h1 className="mt-1 text-rhd-4xl font-semibold leading-7">{ml(t, "title")}</h1>
          </div>
        </div>
      </section>

      <div className="mx-auto w-full max-w-sm px-5 pt-4 pb-[max(32px,env(safe-area-inset-bottom))] sm:max-w-md sm:px-6 lg:flex lg:min-h-svh lg:max-w-none lg:items-center lg:justify-center lg:px-10 lg:py-10">
        <div className="w-full rounded-xl border border-[#dbe6f3] bg-white p-5 shadow-[0_12px_28px_rgba(15,23,42,0.08)] sm:p-6 lg:max-w-[420px]">
          <div className="mb-4">
            <UnderlineTabs
              ariaLabel={ml(t, "entryAria")}
              items={mobileModeTabs}
              value={mode}
              onChange={(value) => changeMode(value as "account" | "register")}
            />
          </div>
          {mode === "account" ? (
            <>
              <h2 className="sr-only">{ml(t, "accountModeTitle")}</h2>
              <form className="space-y-3.5" onSubmit={submitLogin}>
                <div className="space-y-1.5">
                  <Label htmlFor="mobile-customer-username" className="text-xs font-medium text-[#657084]">{ml(t, "accountLabel")}</Label>
                  <div className="relative">
                    <UserRoundIcon className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[#8b96a8]" />
                    <Input id="mobile-customer-username" name="username" className="h-10 rounded-md border-[#dce2ea] bg-white pl-10 text-rhd-md text-[#172033] shadow-none placeholder:text-[#9aa3b2] focus-visible:bg-white" placeholder={ml(t, "accountPlaceholder")} autoCapitalize="none" autoComplete="username" autoCorrect="off" enterKeyHint="next" required autoFocus />
                  </div>
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="mobile-customer-password" className="text-xs font-medium text-[#657084]">{ml(t, "passwordLabel")}</Label>
                  <div className="relative">
                    <LockKeyholeIcon className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[#8b96a8]" />
                    <Input id="mobile-customer-password" name="password" type={passwordVisible ? "text" : "password"} className="h-10 rounded-md border-[#dce2ea] bg-white pl-10 pr-10 text-rhd-md text-[#172033] shadow-none placeholder:text-[#9aa3b2] focus-visible:bg-white" placeholder={ml(t, "passwordPlaceholder")} autoComplete="current-password" enterKeyHint="go" required />
                    <button type="button" className="absolute right-3 top-1/2 -translate-y-1/2 text-[#7b8494]" onClick={() => setPasswordVisible((visible) => !visible)} aria-label={passwordVisible ? ml(t, "hidePassword") : ml(t, "showPassword")} title={passwordVisible ? ml(t, "hidePassword") : ml(t, "showPassword")}>
                      {passwordVisible ? <EyeIcon className="size-4" /> : <EyeOffIcon className="size-4" />}
                    </button>
                  </div>
                </div>
                {error ? <p className="rounded-md bg-red-50 px-3 py-2 text-rhd-sm leading-5 text-red-700" role="alert">{error}</p> : null}
                <Button type="submit" className="h-10 w-full rounded-md bg-[#2563eb] text-rhd-md font-semibold text-white shadow-none hover:bg-[#1d4ed8]" disabled={submitting}>
                  {submitting ? <Loader2Icon className="size-4 animate-spin" /> : <LogInIcon className="size-4" />}
                  {submitting ? ml(t, "loggingIn") : ml(t, "login")}
                </Button>
              </form>
            </>
          ) : registrationContext ? (
            <form className="space-y-3.5" onSubmit={submitRegistration}>
              <div>
                <h2 className="text-rhd-3xl font-semibold leading-6 text-[#111827]">{ml(t, "registerModeTitle")}</h2>
              </div>
              <ContextSummary context={registrationContext} />
              <div className="space-y-1.5"><Label htmlFor="mobile-registration-username" className="text-xs text-[#657084]">{ml(t, "accountLabel")}</Label><Input id="mobile-registration-username" name="username" className="h-10 rounded-md border-[#dce2ea] bg-white text-rhd-md text-[#172033] shadow-none" autoCapitalize="none" autoComplete="username" required autoFocus /></div>
              <div className="space-y-1.5"><Label htmlFor="mobile-registration-email" className="text-xs text-[#657084]">{ml(t, "emailLabel")}</Label><Input id="mobile-registration-email" name="email" type="email" defaultValue={registrationContext.email || ""} readOnly={Boolean(registrationContext.email)} className="h-10 rounded-md border-[#dce2ea] bg-white text-rhd-md text-[#172033] shadow-none" autoComplete="email" required /></div>
              {method === "visitor" ? (
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_auto]">
                  <div className="space-y-1.5">
                    <Label htmlFor="mobile-registration-verification-code" className="text-xs text-[#657084]">{ml(t, "verificationCodeLabel")}</Label>
                    <Input id="mobile-registration-verification-code" name="verificationCode" className="h-10 rounded-md border-[#dce2ea] bg-white text-rhd-md text-[#172033] shadow-none" placeholder={ml(t, "verificationCodePlaceholder")} autoComplete="one-time-code" required />
                  </div>
                  <Button type="button" variant="outline" className="h-10 w-full self-end rounded-md border-[#dce2ea] bg-white px-3 text-rhd-sm font-semibold text-[#2563eb] sm:w-auto" disabled={sendingVerificationCode} onClick={() => void sendCustomerRegisterCode(registrationContext.email ?? "")}>
                    {sendingVerificationCode ? <Loader2Icon className="size-4 animate-spin" /> : <MailIcon className="size-4" />}
                    {ml(t, "sendVerificationCode")}
                  </Button>
                </div>
              ) : null}
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div className="space-y-1.5"><Label htmlFor="mobile-registration-password" className="text-xs text-[#657084]">{ml(t, "passwordLabel")}</Label><Input id="mobile-registration-password" name="password" type="password" className="h-10 rounded-md border-[#dce2ea] bg-white text-rhd-md text-[#172033] shadow-none" autoComplete="new-password" minLength={8} required /></div>
                <div className="space-y-1.5"><Label htmlFor="mobile-registration-confirm" className="text-xs text-[#657084]">{ml(t, "confirmPasswordLabel")}</Label><Input id="mobile-registration-confirm" name="confirmPassword" type="password" className="h-10 rounded-md border-[#dce2ea] bg-white text-rhd-md text-[#172033] shadow-none" autoComplete="new-password" minLength={8} required /></div>
              </div>
              {error ? <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-rhd-sm leading-5 text-red-700" role="alert">{error}</p> : null}
              <div className="flex gap-2">
                <Button type="button" variant="outline" size="icon" className="size-10 rounded-md border-[#dce2ea] bg-white text-[#657084]" onClick={() => { setRegistrationContext(null); setError("") }} aria-label={ml(t, "backToEditAuth")} title={ml(t, "backToEditAuth")}><ArrowLeftIcon className="size-4" /></Button>
                <Button type="submit" className="h-10 flex-1 rounded-md bg-[#2563eb] text-rhd-md font-semibold text-white hover:bg-[#1d4ed8]" disabled={submitting}>{submitting ? <Loader2Icon className="size-4 animate-spin" /> : <UserPlusIcon className="size-4" />}{submitting ? ml(t, "registering") : ml(t, "registerAndEnter")}</Button>
              </div>
            </form>
          ) : (
            <div className="space-y-4">
              <div>
                <h2 className="text-rhd-3xl font-semibold leading-6 text-[#111827]">{ml(t, "registerModeTitle")}</h2>
              </div>
              <div className="space-y-1.5">
                <Label id="mobile-registration-method-label" className="text-xs text-[#657084]">{ml(t, "registrationMethodLabel")}</Label>
                <SelectField
                  style={{ marginBottom: 0 }}
                  selectProps={{
                    "aria-labelledby": "mobile-registration-method-label",
                    value: method,
                    onChange: (value) => changeMethod(value as CustomerRegistrationMethod),
                    options: registrationMethodOptions,
                    size: "large",
                    style: { width: "100%" },
                  }}
                />
              </div>
              <form className="space-y-3.5" onSubmit={verifyCredential}>
                <div className="space-y-2">
                  <Label htmlFor="mobile-registration-credential" className="text-xs text-[#657084]">{method === "invite" ? ml(t, "inviteCodeLabel") : method === "visitor" ? ml(t, "emailLabel") : ml(t, "serviceCodeLabel")}</Label>
                  <div className="relative">
                    {method === "invite" ? <KeyRoundIcon className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[#8b96a8]" /> : method === "visitor" ? <MailIcon className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[#8b96a8]" /> : <WrenchIcon className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[#8b96a8]" />}
                    <Input id="mobile-registration-credential" type={method === "visitor" ? "email" : "text"} value={credential} onChange={(event) => setCredential(event.target.value)} className={cn("h-10 rounded-md border-[#dce2ea] bg-white pl-10 text-rhd-md text-[#172033] shadow-none placeholder:text-[#9aa3b2]", method === "service_code" && "font-mono uppercase")} placeholder={method === "invite" ? ml(t, "inviteCodePlaceholder") : method === "visitor" ? ml(t, "visitorEmailPlaceholder") : ml(t, "serviceCodePlaceholder")} autoCapitalize={method === "service_code" ? "characters" : "none"} autoComplete={method === "visitor" ? "email" : "off"} autoCorrect="off" required autoFocus />
                  </div>
                </div>
                {error ? <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-rhd-sm leading-5 text-red-700" role="alert">{error}</p> : null}
                <Button type="submit" className="h-10 w-full rounded-md bg-[#2563eb] text-rhd-md font-semibold text-white hover:bg-[#1d4ed8]" disabled={verifying || !credential.trim()}>{verifying ? <Loader2Icon className="size-4 animate-spin" /> : <BadgeCheckIcon className="size-4" />}{verifying ? ml(t, "verifying") : ml(t, "verifyAndContinue")}</Button>
              </form>
            </div>
          )}
        </div>
      </div>
    </main>
  )
}
