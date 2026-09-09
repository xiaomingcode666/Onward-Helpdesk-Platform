"use client"

import Image from "next/image"
import Link from "next/link"
import { useSearchParams } from "next/navigation"
import { useEffect, useState } from "react"
import {
  ArrowRightIcon,
  BadgeCheckIcon,
  BarChart3Icon,
  BrainCircuitIcon,
  Building2Icon,
  EyeIcon,
  EyeOffIcon,
  FileTextIcon,
  Globe2Icon,
  HeadphonesIcon,
  InfoIcon,
  Loader2Icon,
  LockKeyholeIcon,
  MailIcon,
  MonitorIcon,
  ScanLineIcon,
  ShieldCheckIcon,
  TriangleAlertIcon,
  SmartphoneIcon,
  UserRoundIcon,
  UsersRoundIcon,
  WrenchIcon,
  type LucideIcon,
} from "lucide-react"
import { toast } from "sonner"

import { SelectField, UnderlineTabs, type RailopsTabItem } from "@railops/ui"
import { Checkbox as AntCheckbox } from "antd"

import { useAuth } from "@/components/auth-provider"
import { AppLogoMark } from "@/components/brand-logo"
import { ProjectDialog } from "@/components/project-dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { SELECTABLE_LOCALES, normalizeSelectableLocale } from "@/i18n/config"
import {
  loginWithPassword,
  bindPortalInvitation,
  registerPortalInvitation,
  registerEnterpriseAccount,
  registerCustomerAccount,
  resetPassword,
  sendAuthVerificationCode,
  verifyPortalInvitation,
  verifyCustomerRegistration,
  type CustomerRegistrationContext,
  type CustomerRegistrationMethod,
  type PortalInvitationDomain,
} from "@/lib/api/auth"
import type { AuthSession } from "@/lib/auth"
import {
  getSessionPortal,
  resolveLoginPortal,
  resolveSessionDestination,
  type LoginPortal,
} from "@/lib/login-portals"
import { CUSTOMER_GUEST_QUESTION_LIMIT, CUSTOMER_GUEST_SERVICE_CODE } from "@/lib/customer-guest-demo"
import { useAppLocale, useI18n } from "@/i18n/provider"
import { cn } from "@/lib/utils"

type PortalPresentation = {
  icon: LucideIcon
  tabKey: string
  eyebrowKey: string
  titleKey: string
  signInKey: string
}

const PORTAL_PRESENTATIONS: Record<LoginPortal, PortalPresentation> = {
  platform: {
    icon: ShieldCheckIcon,
    tabKey: "auth.portals.platform.tab",
    eyebrowKey: "auth.portals.platform.eyebrow",
    titleKey: "auth.portals.platform.title",
    signInKey: "auth.portals.platform.signIn",
  },
  enterprise: {
    icon: Building2Icon,
    tabKey: "auth.portals.enterprise.tab",
    eyebrowKey: "auth.portals.enterprise.eyebrow",
    titleKey: "auth.portals.enterprise.title",
    signInKey: "auth.portals.enterprise.signIn",
  },
  partner: {
    icon: WrenchIcon,
    tabKey: "auth.portals.partner.tab",
    eyebrowKey: "auth.portals.partner.eyebrow",
    titleKey: "auth.portals.partner.title",
    signInKey: "auth.portals.partner.signIn",
  },
  customer: {
    icon: HeadphonesIcon,
    tabKey: "auth.portals.customer.tab",
    eyebrowKey: "auth.portals.customer.eyebrow",
    titleKey: "auth.portals.customer.title",
    signInKey: "auth.portals.customer.signIn",
  },
}

const authInputClass = "h-10 rounded-md border-[var(--railops-border)] bg-[var(--railops-surface)] text-rhd-md text-[var(--railops-text)] shadow-none placeholder:text-[var(--railops-text-tertiary)] focus-visible:border-[var(--railops-primary)] focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklab,var(--railops-primary)_15%,transparent)]"
const authIconClass = "pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-[var(--railops-text-tertiary)]"
const authLabelClass = "text-rhd-sm font-medium text-[var(--railops-text-secondary)]"
const authSecondaryButtonClass = "h-10 rounded-md border-[var(--railops-border)] bg-[var(--railops-surface)] text-rhd-sm font-medium text-[var(--railops-text)] shadow-none hover:border-[var(--railops-primary)] hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)]"
const authPrimaryButtonClass = "h-10 rounded-md bg-[var(--railops-primary)] px-3 text-rhd-md font-semibold text-white shadow-none hover:bg-[var(--railops-primary-hover)]"
const LOGIN_FONT_FAMILY = '"PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Noto Sans CJK SC", "Source Han Sans SC", Arial, sans-serif'

const LOGIN_FEATURES = [
  {
    icon: MonitorIcon,
    titleKey: "auth.loginHero.features.diagnosis.title",
    detailKey: "auth.loginHero.features.diagnosis.detail",
  },
  {
    icon: BrainCircuitIcon,
    titleKey: "auth.loginHero.features.remoteAssist.title",
    detailKey: "auth.loginHero.features.remoteAssist.detail",
  },
  {
    icon: FileTextIcon,
    titleKey: "auth.loginHero.features.knowledge.title",
    detailKey: "auth.loginHero.features.knowledge.detail",
  },
  {
    icon: UsersRoundIcon,
    titleKey: "auth.loginHero.features.expert.title",
    detailKey: "auth.loginHero.features.expert.detail",
  },
  {
    icon: BarChart3Icon,
    titleKey: "auth.loginHero.features.closedLoop.title",
    detailKey: "auth.loginHero.features.closedLoop.detail",
  },
] as const

function navigateAfterAuth(destination: string) {
  window.location.replace(destination)
}

function LoginLocaleSwitcher({
  showPlatformAdminEntry = false,
  showEnterpriseEntry = false,
  showCustomerEntry = false,
}: {
  showPlatformAdminEntry?: boolean
  showEnterpriseEntry?: boolean
  showCustomerEntry?: boolean
}) {
  const { locale, setLocale, t } = useAppLocale()

  return (
    <div className="flex shrink-0 items-center gap-2">
      {showPlatformAdminEntry ? (
        <Link
          href="/platform/login"
          className="inline-flex h-9 shrink-0 items-center justify-center gap-1.5 rounded-md border border-[var(--railops-border)] bg-[var(--railops-surface)] px-2 text-rhd-sm font-medium text-[var(--railops-text)] shadow-none transition hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklab,var(--railops-primary)_15%,transparent)] sm:px-3"
          aria-label={t("auth.adminLogin")}
          title={t("auth.adminLogin")}
        >
          <ShieldCheckIcon className="size-4" />
          <span className="hidden whitespace-nowrap sm:inline">{t("auth.adminLogin")}</span>
        </Link>
      ) : null}
      {showEnterpriseEntry ? (
        <Link
          href="/dashboard/login?portal=enterprise"
          className="inline-flex h-9 shrink-0 items-center justify-center gap-1.5 rounded-md border border-[var(--railops-border)] bg-[var(--railops-surface)] px-2 text-rhd-sm font-medium text-[var(--railops-text)] shadow-none transition hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklab,var(--railops-primary)_15%,transparent)] sm:px-3"
          aria-label={t("auth.enterpriseLogin")}
          title={t("auth.enterpriseLogin")}
        >
          <Building2Icon className="size-4" />
          <span className="hidden whitespace-nowrap sm:inline">{t("auth.enterpriseLogin")}</span>
        </Link>
      ) : null}
      {showCustomerEntry ? (
        <Link
          href="/customer/login"
          className="inline-flex h-9 shrink-0 items-center justify-center gap-1.5 rounded-md border border-[var(--railops-border)] bg-[var(--railops-surface)] px-2 text-rhd-sm font-medium text-[var(--railops-text)] shadow-none transition hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklab,var(--railops-primary)_15%,transparent)] sm:px-3"
          aria-label={t("auth.customerLogin")}
          title={t("auth.customerLogin")}
        >
          <SmartphoneIcon className="size-4" />
          <span className="hidden whitespace-nowrap sm:inline">{t("auth.customerLogin")}</span>
        </Link>
      ) : null}
      <SelectField
        style={{ marginBottom: 0 }}
        selectProps={{
          "aria-label": t("account.languageSettings"),
          prefix: <Globe2Icon className="size-4 text-[var(--railops-text)]" />,
          value: normalizeSelectableLocale(locale),
          onChange: (value) => setLocale(normalizeSelectableLocale(value)),
          options: SELECTABLE_LOCALES.map((item) => ({ value: item, label: t(`locale.${item}`) })),
          style: { width: 132 },
        }}
      />
    </div>
  )
}

type StaffLoginFormProps = {
  isPending: boolean
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void
  portal: Exclude<LoginPortal, "customer">
  onForgotPassword?: () => void
}

function StaffLoginForm({
  isPending,
  onSubmit,
  portal,
  onForgotPassword,
}: StaffLoginFormProps) {
  const t = useI18n()
  const [passwordVisible, setPasswordVisible] = useState(false)

  return (
    <form className="space-y-4" onSubmit={onSubmit}>
      <div className="space-y-2.5">
        <Label htmlFor="username" className="sr-only">
          {t("auth.username")}
        </Label>
        <div className="relative">
          <UserRoundIcon className={authIconClass} />
          <Input
            id="username"
            name="username"
            className={`${authInputClass} pl-10`}
            placeholder={t("auth.usernamePlaceholder")}
            autoComplete="username"
            required
          />
        </div>
      </div>
      <div className="space-y-2.5">
        <Label htmlFor="password" className="sr-only">
          {t("auth.password")}
        </Label>
        <div className="relative">
          <LockKeyholeIcon className={authIconClass} />
          <Input
            id="password"
            name="password"
            type={passwordVisible ? "text" : "password"}
            className={`${authInputClass} pl-10 pr-10`}
            placeholder={t("auth.passwordPlaceholder")}
            autoComplete="current-password"
            required
          />
          <button
            type="button"
            className="absolute right-3.5 top-1/2 -translate-y-1/2 text-[var(--railops-text-secondary)] transition hover:text-[var(--railops-primary-hover)]"
            onClick={() => setPasswordVisible((current) => !current)}
            aria-label={passwordVisible ? t("auth.passwordHide") : t("auth.passwordShow")}
            title={passwordVisible ? t("auth.passwordHide") : t("auth.passwordShow")}
          >
            {passwordVisible ? <EyeIcon className="size-4" /> : <EyeOffIcon className="size-4" />}
          </button>
        </div>
      </div>
      <div className="flex items-center justify-between gap-4 text-rhd-sm text-[var(--railops-text-secondary)]">
        <label className="inline-flex items-center gap-2">
          <AntCheckbox />
          <span>{t("auth.rememberAccount")}</span>
        </label>
        <button type="button" className="font-semibold text-[var(--railops-primary-hover)] transition hover:text-[var(--railops-primary)]" onClick={onForgotPassword}>
          {t("auth.forgotPassword")}
        </button>
      </div>
      <Button type="submit" size="lg" className={`${authPrimaryButtonClass} w-full`} disabled={isPending}>
        {isPending ? <Loader2Icon className="size-4 animate-spin" /> : null}
        {isPending ? t("auth.signingIn") : t("auth.loginHero.submit")}
      </Button>
    </form>
  )
}

function CustomerEntryForm({
  isPending,
  onAccountSubmit,
  onForgotPassword,
}: {
  isPending: boolean
  onAccountSubmit: (event: React.FormEvent<HTMLFormElement>) => void
  onForgotPassword: () => void
}) {
  const t = useI18n()
  const searchParams = useSearchParams()
  const { refreshProfile } = useAuth()
  const inviteFromURL = searchParams.get("invite")?.trim() ?? ""
  const [mode, setMode] = useState<"account" | "register">(inviteFromURL ? "register" : "account")
  const customerModeTabs: RailopsTabItem[] = [
    { value: "account", label: t("auth.customer.accountLogin") },
    { value: "register", label: t("auth.customer.registerAccount") },
  ]
  const [method, setMethod] = useState<CustomerRegistrationMethod>(inviteFromURL ? "invite" : "service_code")
  const [credential, setCredential] = useState(inviteFromURL)
  const [registrationContext, setRegistrationContext] = useState<CustomerRegistrationContext | null>(null)
  const [verifying, setVerifying] = useState(Boolean(inviteFromURL))
  const [registering, setRegistering] = useState(false)
  const [sendingVerificationCode, setSendingVerificationCode] = useState(false)
  const [registrationError, setRegistrationError] = useState("")
  const nextPath = searchParams.get("next")
  const [passwordVisible, setPasswordVisible] = useState(false)
  const inviteCredentialLocked = method === "invite" && Boolean(inviteFromURL)

  useEffect(() => {
    if (!inviteFromURL) return
    let cancelled = false
    setVerifying(true)
    setRegistrationError("")
    void verifyCustomerRegistration("invite", inviteFromURL)
      .then((context) => {
        if (!cancelled) setRegistrationContext(context)
      })
      .catch(() => {
        if (!cancelled) setRegistrationError(t("auth.customer.invalidInvitation"))
      })
      .finally(() => {
        if (!cancelled) setVerifying(false)
      })
    return () => {
      cancelled = true
    }
  }, [inviteFromURL, t])

  function selectRegistrationMethod(nextMethod: CustomerRegistrationMethod) {
    setMethod(nextMethod)
    setCredential(nextMethod === "invite" ? inviteFromURL : "")
    setRegistrationContext(null)
    setRegistrationError("")
  }

  async function verifyCredential(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const value = credential.trim()
    if (!value || verifying) return
    setVerifying(true)
    setRegistrationError("")
    try {
      setRegistrationContext(await verifyCustomerRegistration(method, value))
    } catch {
      setRegistrationContext(null)
      setRegistrationError(method === "invite" ? t("auth.customer.invalidInvitation") : method === "visitor" ? t("auth.customer.invalidEmail") : t("auth.customer.invalidServiceCode"))
    } finally {
      setVerifying(false)
    }
  }

  async function sendCustomerRegisterCode(email: string) {
    if (!email.trim() || sendingVerificationCode) return
    setSendingVerificationCode(true)
    setRegistrationError("")
    try {
      await sendAuthVerificationCode("customer_register", email.trim())
      toast.success(t("auth.verificationCodeSent"))
    } catch (error) {
      setRegistrationError(error instanceof Error && error.message ? error.message : t("auth.verificationCodeSendFailed"))
    } finally {
      setSendingVerificationCode(false)
    }
  }

  async function register(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!registrationContext || registering) return
    const formData = new FormData(event.currentTarget)
    const username = formData.get("username")?.toString().trim() ?? ""
    const email = formData.get("email")?.toString().trim() ?? ""
    const password = formData.get("password")?.toString() ?? ""
    const confirmPassword = formData.get("confirmPassword")?.toString() ?? ""
    const verificationCode = formData.get("verificationCode")?.toString().trim() ?? ""
    if (password !== confirmPassword) {
      setRegistrationError(t("auth.customer.passwordMismatch"))
      return
    }
    if (password.length < 8) {
      setRegistrationError(t("auth.customer.passwordLength"))
      return
    }
    if (method === "visitor" && !verificationCode) {
      setRegistrationError(t("auth.verificationCodeRequired"))
      return
    }
    setRegistering(true)
    setRegistrationError("")
    let navigating = false
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
      await refreshProfile()
      toast.success(t("auth.customer.registrationSuccess"))
      navigating = true
      navigateAfterAuth(resolveSessionDestination(session, nextPath))
    } catch (error) {
      setRegistrationError(error instanceof Error && error.message ? error.message : t("auth.customer.registrationFailed"))
    } finally {
      if (!navigating) setRegistering(false)
    }
  }

  return (
    <div className="mt-5">
      <UnderlineTabs
        ariaLabel={t("auth.customer.registrationMethod")}
        items={customerModeTabs}
        value={mode}
        onChange={(value) => setMode(value as "account" | "register")}
      />

      {mode === "account" ? (
        <form key="customer-account-login" className="mt-4 space-y-4" onSubmit={onAccountSubmit}>
          <div className="space-y-2.5">
            <Label htmlFor="customer-username" className={authLabelClass}>{t("auth.username")}</Label>
            <div className="relative"><UserRoundIcon className={authIconClass} /><Input id="customer-username" name="username" className={`${authInputClass} pl-10`} placeholder={t("auth.usernamePlaceholder")} autoComplete="username" required /></div>
          </div>
          <div className="space-y-2.5">
            <Label htmlFor="customer-password" className={authLabelClass}>{t("auth.password")}</Label>
            <div className="relative"><LockKeyholeIcon className={authIconClass} /><Input id="customer-password" name="password" type={passwordVisible ? "text" : "password"} className={`${authInputClass} pl-10 pr-10`} placeholder={t("auth.passwordPlaceholder")} autoComplete="current-password" required /><button type="button" className="absolute right-3.5 top-1/2 -translate-y-1/2 text-[var(--railops-text-secondary)] transition hover:text-[var(--railops-primary-hover)]" onClick={() => setPasswordVisible((current) => !current)} aria-label={passwordVisible ? t("auth.passwordHide") : t("auth.passwordShow")} title={passwordVisible ? t("auth.passwordHide") : t("auth.passwordShow")}>{passwordVisible ? <EyeIcon className="size-4" /> : <EyeOffIcon className="size-4" />}</button></div>
          </div>
          <div className="flex justify-end text-rhd-sm">
            <button type="button" className="font-semibold text-[var(--railops-primary-hover)] hover:text-[var(--railops-primary)]" onClick={onForgotPassword}>
              {t("auth.forgotPassword")}
            </button>
          </div>
          <Button type="submit" size="lg" className={`${authPrimaryButtonClass} w-full`} disabled={isPending}>{isPending ? <Loader2Icon className="size-4 animate-spin" /> : <ArrowRightIcon className="size-4" />}{isPending ? t("auth.signingIn") : t("auth.loginHero.submit")}</Button>
        </form>
      ) : registrationContext?.registered && method === "invite" ? (
        <InvitationBindForm
          context={registrationContext}
          credential={credential.trim()}
          domainType="customer"
          nextPath={nextPath}
          refreshProfile={refreshProfile}
          onBack={() => { setRegistrationContext(null); setRegistrationError("") }}
        />
      ) : registrationContext ? (
        <form key="customer-registration" className="mt-4 space-y-3.5" onSubmit={register}>
          <RegistrationContextSummary context={registrationContext} t={t} />
          {method === "invite" && credential.trim() ? (
            <div className="space-y-2.5">
              <Label htmlFor="registration-invite-code" className={authLabelClass}>{t("auth.customer.inviteCode")}</Label>
              <div className="relative">
                <Building2Icon className={authIconClass} />
                <Input id="registration-invite-code" className={`${authInputClass} pl-10`} value={credential.trim()} readOnly aria-readonly="true" />
              </div>
            </div>
          ) : null}
          <div className="space-y-2.5">
            <Label htmlFor="registration-username" className={authLabelClass}>{t("auth.username")}</Label>
            <div className="relative"><UserRoundIcon className={authIconClass} /><Input id="registration-username" name="username" className={`${authInputClass} pl-10`} placeholder={t("auth.customer.registrationUsernamePlaceholder")} autoComplete="username" required autoFocus /></div>
          </div>
          <div className="space-y-2.5">
            <Label htmlFor="registration-email" className={authLabelClass}>{t("auth.customer.email")}</Label>
            <div className="relative"><MailIcon className={authIconClass} /><Input id="registration-email" name="email" type="email" className={`${authInputClass} pl-10`} defaultValue={registrationContext.email ?? ""} readOnly={Boolean(registrationContext.email)} autoComplete="email" required /></div>
          </div>
          {method === "visitor" ? (
            <div className="grid gap-3 sm:grid-cols-[1fr_auto]">
              <div className="space-y-2.5">
                <Label htmlFor="registration-verification-code" className={authLabelClass}>{t("auth.verificationCode")}</Label>
                <Input id="registration-verification-code" name="verificationCode" className={authInputClass} placeholder={t("auth.verificationCodePlaceholder")} autoComplete="one-time-code" required />
              </div>
              <Button type="button" variant="outline" className={`${authSecondaryButtonClass} self-end`} disabled={sendingVerificationCode} onClick={() => void sendCustomerRegisterCode(registrationContext.email ?? "")}>
                {sendingVerificationCode ? <Loader2Icon className="size-4 animate-spin" /> : <MailIcon className="size-4" />}
                {t("auth.sendVerificationCode")}
              </Button>
            </div>
          ) : null}
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-2.5"><Label htmlFor="registration-password" className={authLabelClass}>{t("auth.password")}</Label><Input id="registration-password" name="password" type="password" className={authInputClass} placeholder={t("auth.customer.newPasswordPlaceholder")} autoComplete="new-password" minLength={8} required /></div>
            <div className="space-y-2.5"><Label htmlFor="registration-confirm-password" className={authLabelClass}>{t("auth.customer.confirmPassword")}</Label><Input id="registration-confirm-password" name="confirmPassword" type="password" className={authInputClass} placeholder={t("auth.customer.confirmPasswordPlaceholder")} autoComplete="new-password" minLength={8} required /></div>
          </div>
          {registrationError ? <RegistrationError message={registrationError} /> : null}
          <div className="flex gap-2">
            <Button type="button" variant="outline" size="lg" className={authSecondaryButtonClass} onClick={() => { setRegistrationContext(null); setRegistrationError("") }} disabled={registering} aria-label={t("auth.customer.changeCredential")}><ArrowRightIcon className="rotate-180" /></Button>
            <Button type="submit" size="lg" className={`${authPrimaryButtonClass} flex-1`} disabled={registering}>{registering ? <Loader2Icon className="size-4 animate-spin" /> : <BadgeCheckIcon className="size-4" />}{registering ? t("auth.customer.registering") : t("auth.customer.registerAndEnter")}</Button>
          </div>
        </form>
      ) : (
        <div className="mt-4 space-y-4">
          <div className="grid grid-cols-3 gap-2" role="radiogroup" aria-label={t("auth.customer.registrationMethod")}>
            <RegistrationMethodButton active={method === "invite"} icon={Building2Icon} title={t("auth.customer.inviteRegistration")} onClick={() => selectRegistrationMethod("invite")} />
            <RegistrationMethodButton active={method === "service_code"} icon={ScanLineIcon} title={t("auth.customer.deviceRegistration")} onClick={() => selectRegistrationMethod("service_code")} />
            <RegistrationMethodButton active={method === "visitor"} icon={UserRoundIcon} title={t("auth.customer.visitorRegistration")} onClick={() => selectRegistrationMethod("visitor")} />
          </div>
          <form className="space-y-4" onSubmit={verifyCredential}>
            <div className="space-y-2.5">
              <Label htmlFor="registration-credential" className={authLabelClass}>{method === "invite" ? t("auth.customer.inviteCode") : method === "visitor" ? t("auth.customer.email") : t("auth.customer.serviceCode")}</Label>
              <div className="relative">
                {method === "invite" ? <Building2Icon className={authIconClass} /> : method === "visitor" ? <MailIcon className={authIconClass} /> : <ScanLineIcon className={authIconClass} />}
                <Input id="registration-credential" type={method === "visitor" ? "email" : "text"} className={cn(authInputClass, "pl-10", method === "service_code" && "uppercase")} placeholder={method === "invite" ? t("auth.customer.inviteCodePlaceholder") : method === "visitor" ? t("auth.customer.emailPlaceholder") : t("auth.customer.serviceCodePlaceholder")} value={credential} onChange={(event) => setCredential(event.target.value)} autoComplete={method === "visitor" ? "email" : "off"} readOnly={inviteCredentialLocked} aria-readonly={inviteCredentialLocked ? "true" : undefined} required autoFocus />
              </div>
            </div>
            {registrationError ? <RegistrationError message={registrationError} /> : null}
            <Button type="submit" size="lg" className={`${authPrimaryButtonClass} w-full`} disabled={verifying || !credential.trim()}>{verifying ? <Loader2Icon className="size-4 animate-spin" /> : <BadgeCheckIcon className="size-4" />}{verifying ? t("auth.customer.verifying") : t("auth.customer.verifyAndContinue")}</Button>
          </form>
        </div>
      )}
    </div>
  )
}

function InvitationBindForm({
  context,
  credential,
  domainType,
  nextPath,
  onBack,
  refreshProfile,
}: {
  context: CustomerRegistrationContext
  credential: string
  domainType: PortalInvitationDomain
  nextPath: string | null
  onBack?: () => void
  refreshProfile: () => Promise<void>
}) {
  const t = useI18n()
  const [submitting, setSubmitting] = useState(false)
  const [passwordVisible, setPasswordVisible] = useState(false)
  const [error, setError] = useState("")
  const successKey = domainType === "partner" ? "auth.partner.bindSuccess" : "auth.customer.bindSuccess"
  const failedKey = domainType === "partner" ? "auth.partner.bindFailed" : "auth.customer.bindFailed"
  const submitKey = domainType === "partner" ? "auth.partner.bindAndEnter" : "auth.customer.bindAndEnter"

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting) return
    const formData = new FormData(event.currentTarget)
    const username = formData.get("username")?.toString().trim() ?? ""
    const password = formData.get("password")?.toString() ?? ""
    setSubmitting(true)
    setError("")
    let navigating = false
    try {
      const session = await bindPortalInvitation({ domainType, credential, username, password })
      await refreshProfile()
      toast.success(t(successKey))
      navigating = true
      navigateAfterAuth(resolveSessionDestination(session, nextPath))
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : t(failedKey))
    } finally {
      if (!navigating) setSubmitting(false)
    }
  }

  return (
    <form key={`${domainType}-invitation-bind`} className="mt-4 space-y-3.5" onSubmit={submit}>
      <RegistrationContextSummary context={context} t={t} />
      <div className="space-y-2.5">
        <Label htmlFor={`${domainType}-bind-invite-code`} className={authLabelClass}>{t("auth.customer.inviteCode")}</Label>
        <div className="relative">
          <Building2Icon className={authIconClass} />
          <Input id={`${domainType}-bind-invite-code`} className={`${authInputClass} pl-10`} value={credential} readOnly aria-readonly="true" />
        </div>
      </div>
      <div className="space-y-2.5">
        <Label htmlFor={`${domainType}-bind-email`} className={authLabelClass}>{t("auth.customer.email")}</Label>
        <div className="relative">
          <MailIcon className={authIconClass} />
          <Input id={`${domainType}-bind-email`} className={`${authInputClass} pl-10`} value={context.email ?? ""} readOnly aria-readonly="true" />
        </div>
      </div>
      <div className="space-y-2.5">
        <Label htmlFor={`${domainType}-bind-username`} className={authLabelClass}>{t("auth.username")}</Label>
        <div className="relative">
          <UserRoundIcon className={authIconClass} />
          <Input id={`${domainType}-bind-username`} name="username" className={`${authInputClass} pl-10`} placeholder={t("auth.usernamePlaceholder")} autoComplete="username" required autoFocus />
        </div>
      </div>
      <div className="space-y-2.5">
        <Label htmlFor={`${domainType}-bind-password`} className={authLabelClass}>{t("auth.password")}</Label>
        <div className="relative">
          <LockKeyholeIcon className={authIconClass} />
          <Input id={`${domainType}-bind-password`} name="password" type={passwordVisible ? "text" : "password"} className={`${authInputClass} pl-10 pr-10`} placeholder={t("auth.passwordPlaceholder")} autoComplete="current-password" required />
          <button
            type="button"
            className="absolute right-3.5 top-1/2 -translate-y-1/2 text-[var(--railops-text-secondary)] transition hover:text-[var(--railops-primary-hover)]"
            onClick={() => setPasswordVisible((current) => !current)}
            aria-label={passwordVisible ? t("auth.passwordHide") : t("auth.passwordShow")}
            title={passwordVisible ? t("auth.passwordHide") : t("auth.passwordShow")}
          >
            {passwordVisible ? <EyeIcon className="size-4" /> : <EyeOffIcon className="size-4" />}
          </button>
        </div>
      </div>
      {error ? <RegistrationError message={error} /> : null}
      <div className="flex gap-2">
        {onBack ? <Button type="button" variant="outline" size="lg" className={authSecondaryButtonClass} onClick={onBack} disabled={submitting} aria-label={t("auth.customer.changeCredential")}><ArrowRightIcon className="rotate-180" /></Button> : null}
        <Button type="submit" size="lg" className={`${authPrimaryButtonClass} flex-1`} disabled={submitting}>{submitting ? <Loader2Icon className="size-4 animate-spin" /> : <BadgeCheckIcon className="size-4" />}{submitting ? t("auth.customer.registering") : t(submitKey)}</Button>
      </div>
    </form>
  )
}

function GuestDeviceInfoDialog({
  onOpenChange,
  open,
}: {
  onOpenChange: (open: boolean) => void
  open: boolean
}) {
  const t = useI18n()
  return (
    <ProjectDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("auth.customer.guestDeviceTitle")}
      size="sm"
      contentClassName="bg-white shadow-xl"
      footer={(
        <Button type="button" render={<Link href={`/c/${CUSTOMER_GUEST_SERVICE_CODE}`} />}>
          {t("auth.customer.openGuestEntry")}
        </Button>
      )}
    >
      <div className="rounded-md border bg-muted/30 p-3">
        <p className="text-xs font-medium text-muted-foreground">{t("auth.customer.guestDeviceCode")}</p>
        <p className="mt-2 break-all font-mono text-rhd-md font-semibold text-foreground">{CUSTOMER_GUEST_SERVICE_CODE}</p>
      </div>
      <div className="grid gap-2 text-sm text-muted-foreground">
        <p>{t("auth.customer.guestDefaultDevice")}</p>
        <p>{t("auth.customer.guestQuestionLimit", { count: CUSTOMER_GUEST_QUESTION_LIMIT })}</p>
      </div>
    </ProjectDialog>
  )
}

type PortalChoiceOption = NonNullable<AuthSession["availablePortals"]>[number]
type BusinessPortalChoiceOption = PortalChoiceOption & { domainType: "enterprise" | "partner" }

function isBusinessPortalChoiceOption(option: PortalChoiceOption): option is BusinessPortalChoiceOption {
  return option.domainType === "enterprise" || option.domainType === "partner"
}

function BusinessPortalChoiceDialog({
  onOpenChange,
  onSelect,
  open,
  options,
  pending,
}: {
  onOpenChange: (open: boolean) => void
  onSelect: (domainType: "enterprise" | "partner") => void
  open: boolean
  options: PortalChoiceOption[]
  pending: boolean
}) {
  const t = useI18n()
  const businessOptions = options
    .filter(isBusinessPortalChoiceOption)
    .sort((left, right) => Number(Boolean(right.default)) - Number(Boolean(left.default)))
  return (
    <ProjectDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("auth.portalChoice.title")}
      size="sm"
      contentClassName="bg-white shadow-xl"
    >
      <div className="grid gap-3">
        {businessOptions.map((option) => {
          const partner = option.domainType === "partner"
          const Icon = partner ? WrenchIcon : Building2Icon
          return (
            <Button
              key={`${option.domainType}-${option.subjectId}`}
              type="button"
              variant={partner ? "default" : "outline"}
              className="h-auto justify-start gap-3 rounded-md p-3 text-left"
              disabled={pending}
              onClick={() => onSelect(option.domainType)}
            >
              <Icon className="size-5 shrink-0" />
              <span className="min-w-0">
                <span className="block text-rhd-md font-semibold">{partner ? t("auth.portalChoice.partner") : t("auth.portalChoice.enterprise")}</span>
                <span className="mt-1 block truncate text-xs opacity-80">{option.label}</span>
              </span>
            </Button>
          )
        })}
      </div>
    </ProjectDialog>
  )
}

function PasswordResetDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const t = useI18n()
  const [email, setEmail] = useState("")
  const [verificationCode, setVerificationCode] = useState("")
  const [password, setPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [sending, setSending] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState("")

  async function sendCode() {
    if (!email.trim() || sending) return
    setSending(true)
    setError("")
    try {
      await sendAuthVerificationCode("password_reset", email.trim())
      toast.success(t("auth.verificationCodeSent"))
    } catch (err) {
      setError(err instanceof Error ? err.message : t("auth.verificationCodeSendFailed"))
    } finally {
      setSending(false)
    }
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (password !== confirmPassword) {
      setError(t("auth.customer.passwordMismatch"))
      return
    }
    if (password.length < 8) {
      setError(t("auth.customer.passwordLength"))
      return
    }
    setSubmitting(true)
    setError("")
    try {
      await resetPassword({ email: email.trim(), verificationCode: verificationCode.trim(), password })
      toast.success(t("auth.passwordResetSuccess"))
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("auth.passwordResetFailed"))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <ProjectDialog open={open} onOpenChange={onOpenChange} title={t("auth.passwordResetTitle")} size="sm" contentClassName="bg-white shadow-xl">
      <form className="space-y-3.5" onSubmit={submit}>
        <div className="space-y-2">
          <Label htmlFor="reset-email" className={authLabelClass}>{t("auth.customer.email")}</Label>
          <Input id="reset-email" type="email" className={authInputClass} value={email} onChange={(event) => setEmail(event.target.value)} autoComplete="email" required />
        </div>
        <div className="grid gap-3 sm:grid-cols-[1fr_auto]">
          <div className="space-y-2">
            <Label htmlFor="reset-code" className={authLabelClass}>{t("auth.verificationCode")}</Label>
            <Input id="reset-code" className={authInputClass} value={verificationCode} onChange={(event) => setVerificationCode(event.target.value)} autoComplete="one-time-code" required />
          </div>
          <Button type="button" variant="outline" className={`${authSecondaryButtonClass} self-end`} onClick={() => void sendCode()} disabled={sending || !email.trim()}>
            {sending ? <Loader2Icon className="size-4 animate-spin" /> : <MailIcon className="size-4" />}
            {t("auth.sendVerificationCode")}
          </Button>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="reset-password" className={authLabelClass}>{t("auth.customer.newPasswordPlaceholder")}</Label>
            <Input id="reset-password" type="password" className={authInputClass} value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="new-password" required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="reset-confirm-password" className={authLabelClass}>{t("auth.customer.confirmPassword")}</Label>
            <Input id="reset-confirm-password" type="password" className={authInputClass} value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} autoComplete="new-password" required />
          </div>
        </div>
        {error ? <RegistrationError message={error} /> : null}
        <Button type="submit" className={`${authPrimaryButtonClass} w-full`} disabled={submitting}>
          {submitting ? <Loader2Icon className="size-4 animate-spin" /> : <BadgeCheckIcon className="size-4" />}
          {t("auth.passwordResetSubmit")}
        </Button>
      </form>
    </ProjectDialog>
  )
}

function EnterpriseRegistrationForm({
  nextPath,
  refreshProfile,
}: {
  nextPath: string | null
  refreshProfile: () => Promise<void>
}) {
  const t = useI18n()
  const [email, setEmail] = useState("")
  const [verificationCode, setVerificationCode] = useState("")
  const [sending, setSending] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [verificationError, setVerificationError] = useState("")
  const [error, setError] = useState("")

  async function sendCode() {
    const targetEmail = email.trim()
    if (!targetEmail || sending) return
    setSending(true)
    setVerificationError("")
    setError("")
    try {
      await sendAuthVerificationCode("enterprise_register", targetEmail)
      toast.success(t("auth.verificationCodeSent"))
    } catch (err) {
      setVerificationError(err instanceof Error ? err.message : t("auth.verificationCodeSendFailed"))
    } finally {
      setSending(false)
    }
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formData = new FormData(event.currentTarget)
    const password = formData.get("password")?.toString() ?? ""
    const confirmPassword = formData.get("confirmPassword")?.toString() ?? ""
    if (password !== confirmPassword) {
      setError(t("auth.customer.passwordMismatch"))
      return
    }
    if (password.length < 8) {
      setError(t("auth.customer.passwordLength"))
      return
    }
    setSubmitting(true)
    setError("")
    setVerificationError("")
    try {
      const session = await registerEnterpriseAccount({
        tenantName: formData.get("tenantName")?.toString().trim() ?? "",
        username: formData.get("username")?.toString().trim() ?? "",
        displayName: formData.get("displayName")?.toString().trim() ?? "",
        email: email.trim(),
        mobile: formData.get("mobile")?.toString().trim() ?? "",
        password,
        verificationCode: verificationCode.trim(),
      })
      await refreshProfile()
      toast.success(t("auth.enterpriseRegistrationSuccess"))
      navigateAfterAuth(resolveSessionDestination(session, nextPath))
    } catch (err) {
      setError(err instanceof Error ? err.message : t("auth.enterpriseRegistrationFailed"))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="space-y-3.5" onSubmit={submit}>
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-2 sm:col-span-2">
          <Label htmlFor="enterprise-tenant-name" className={authLabelClass}>{t("auth.enterpriseName")}</Label>
          <Input id="enterprise-tenant-name" name="tenantName" className={authInputClass} required autoFocus />
        </div>
        <div className="space-y-2">
          <Label htmlFor="enterprise-register-username" className={authLabelClass}>{t("auth.username")}</Label>
          <Input id="enterprise-register-username" name="username" className={authInputClass} required />
        </div>
        <div className="space-y-2">
          <Label htmlFor="enterprise-register-display-name" className={authLabelClass}>{t("auth.displayName")}</Label>
          <Input id="enterprise-register-display-name" name="displayName" className={authInputClass} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="enterprise-register-email" className={authLabelClass}>{t("auth.customer.email")}</Label>
          <Input id="enterprise-register-email" name="email" type="email" className={authInputClass} value={email} onChange={(event) => { setEmail(event.target.value); setVerificationError("") }} autoComplete="email" required />
        </div>
        <div className="space-y-2">
          <Label htmlFor="enterprise-register-mobile" className={authLabelClass}>{t("auth.mobile")}</Label>
          <Input id="enterprise-register-mobile" name="mobile" className={authInputClass} />
        </div>
      </div>
      <div className="grid gap-3 sm:grid-cols-[1fr_auto]">
        <div className="space-y-2">
          <Label htmlFor="enterprise-register-code" className={authLabelClass}>{t("auth.verificationCode")}</Label>
          <Input id="enterprise-register-code" name="verificationCode" className={authInputClass} value={verificationCode} onChange={(event) => setVerificationCode(event.target.value)} placeholder={t("auth.verificationCodePlaceholder")} autoComplete="one-time-code" required />
        </div>
        <Button type="button" variant="outline" className={`${authSecondaryButtonClass} self-end`} onClick={() => void sendCode()} disabled={sending || !email.trim()}>
          {sending ? <Loader2Icon className="size-4 animate-spin" /> : <MailIcon className="size-4" />}
          {t("auth.sendVerificationCode")}
        </Button>
      </div>
      {verificationError ? <RegistrationError message={verificationError} /> : null}
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="enterprise-register-password" className={authLabelClass}>{t("auth.password")}</Label>
          <Input id="enterprise-register-password" name="password" type="password" className={authInputClass} autoComplete="new-password" required />
        </div>
        <div className="space-y-2">
          <Label htmlFor="enterprise-register-confirm-password" className={authLabelClass}>{t("auth.customer.confirmPassword")}</Label>
          <Input id="enterprise-register-confirm-password" name="confirmPassword" type="password" className={authInputClass} autoComplete="new-password" required />
        </div>
      </div>
      {error ? <RegistrationError message={error} /> : null}
      <Button type="submit" className={`${authPrimaryButtonClass} w-full`} disabled={submitting}>
        {submitting ? <Loader2Icon className="size-4 animate-spin" /> : <BadgeCheckIcon className="size-4" />}
        {t("auth.enterpriseRegistrationSubmit")}
      </Button>
    </form>
  )
}

function RegistrationMethodButton({ active, icon: Icon, title, onClick }: { active: boolean; icon: LucideIcon; title: string; onClick: () => void }) {
  return <button type="button" role="radio" aria-checked={active} onClick={onClick} className={cn("flex min-w-0 items-center gap-2 rounded-md border px-2.5 py-2 text-left outline-none transition focus-visible:ring-2 focus-visible:ring-primary", active ? "border-primary bg-primary/10" : "border-border bg-card hover:border-primary/40")}><span className={cn("grid size-6 shrink-0 place-items-center rounded-md", active ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground")}><Icon className="size-3.5" /></span><span className="min-w-0 truncate text-xs font-semibold text-foreground">{title}</span></button>
}

function RegistrationContextSummary({ context, t }: { context: CustomerRegistrationContext; t: ReturnType<typeof useI18n> }) {
  return <div className="rounded-md border border-primary/20 bg-primary/5 p-3"><div className="flex items-center gap-2 text-xs font-semibold text-foreground"><BadgeCheckIcon className="size-4 text-primary" />{t("auth.customer.credentialVerified")}</div><div className="mt-2 grid gap-1 text-xs text-muted-foreground sm:grid-cols-2">{context.tenant ? <span className="truncate">{t("auth.customer.enterprise")}: {context.tenant.name}</span> : <span className="truncate">{t("auth.customer.visitorIdentity")}</span>}{context.customerOrg ? <span className="truncate">{t("auth.customer.customerOrg")}: {context.customerOrg.name}</span> : null}{context.partner ? <span className="truncate">{t("auth.partner.supplier")}: {context.partner.name}</span> : null}{context.product ? <span className="truncate">{t("auth.customer.product")}: {context.product.name}</span> : null}{context.device ? <span className="truncate">{t("auth.customer.device")}: {context.device.deviceNo}</span> : null}</div></div>
}

function RegistrationError({ message }: { message: string }) {
  return <p className="flex items-start gap-2 rounded-md border border-destructive/20 bg-destructive/10 p-3 text-xs leading-5 text-destructive"><TriangleAlertIcon className="mt-0.5 size-3.5 shrink-0" />{message}</p>
}

function LoginVisualBackdrop() {
  return (
    <div className="rhd-railops-auth-backdrop pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
      <div className="absolute inset-0 bg-[var(--railops-layout-background)]" />
      <div className="absolute left-[52%] top-[96px] hidden h-[calc(100%-168px)] w-[38vw] min-w-[380px] max-w-[620px] overflow-hidden rounded-lg border border-[var(--railops-border-light)] bg-[var(--railops-surface)] shadow-[var(--railops-modal-shadow)] md:block">
        <Image
          src="/images/login-factory-support.jpg"
          alt=""
          fill
          priority
          unoptimized
          sizes="(min-width: 1280px) 32vw, (min-width: 768px) 42vw, 0px"
          className="object-cover object-center opacity-95"
        />
      </div>
      <div
        className="absolute inset-0"
        style={{
          background:
            "linear-gradient(90deg, var(--railops-layout-background) 0%, var(--railops-layout-background) 47%, color-mix(in oklab, var(--railops-layout-background) 72%, transparent) 66%, var(--railops-layout-background) 100%)",
        }}
      />
    </div>
  )
}

function LoginBrandHeader() {
  const t = useI18n()
  return (
    <div className="flex min-w-0 items-center gap-3">
      <AppLogoMark alt={t("remoteTopbar.brandTitle")} className="size-9" imageClassName="p-0.5" priority />
      <div className="min-w-0">
        <p className="truncate text-rhd-xl font-semibold leading-tight text-[var(--railops-text)]">
          {t("remoteTopbar.brandTitle")}
        </p>
        <p className="mt-0.5 truncate text-rhd-sm font-medium text-[var(--railops-text-secondary)]">
          {t("remoteTopbar.brandSub")}
        </p>
      </div>
    </div>
  )
}

function LoginHeroContent() {
  const t = useI18n()
  return (
    <section className="rhd-railops-auth-hero flex min-w-0 flex-col justify-center pb-4 pt-4 lg:pb-6 lg:pt-0">
      <div className="max-w-[620px]">
        <h1 className="max-w-[620px] text-rhd-5xl font-semibold leading-[1.3] text-[var(--railops-text)] sm:text-rhd-5xl lg:text-rhd-5xl">
          {t("auth.loginHero.highlight")}
        </h1>
        <p className="mt-3 max-w-[540px] text-rhd-md font-medium leading-6 text-[var(--railops-text-secondary)]">
          {t("auth.loginHero.description")}
        </p>
      </div>

      <div className="mt-8 grid max-w-[540px] gap-3">
        {LOGIN_FEATURES.map((feature) => {
          const Icon = feature.icon
          return (
            <div key={feature.titleKey} className="flex items-start gap-3">
              <span className="grid size-7 shrink-0 place-items-center rounded-md border border-[var(--railops-border-light)] bg-[var(--railops-surface)] text-[var(--railops-primary-hover)]">
                <Icon className="size-4 stroke-[1.9]" />
              </span>
              <span className="min-w-0">
                <span className="block text-rhd-md font-semibold leading-5 text-[var(--railops-text)]">
                  {t(feature.titleKey)}
                </span>
                <span className="mt-0.5 block text-rhd-sm font-medium leading-5 text-[var(--railops-text-secondary)]">
                  {t(feature.detailKey)}
                </span>
              </span>
            </div>
          )
        })}
      </div>
    </section>
  )
}

function RailOpsLoginCard({
  children,
  title,
  subtitle,
  headerExtra,
}: {
  children: React.ReactNode
  title: string
  subtitle: string
  headerExtra?: React.ReactNode
}) {
  return (
    <section className="rhd-railops-auth-card w-full max-w-[420px] rounded-lg border border-[var(--railops-border-light)] bg-[var(--railops-surface)] p-5 shadow-[var(--railops-modal-shadow)] sm:p-6">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-rhd-4xl font-semibold leading-7 text-[var(--railops-text)]">{title}</h2>
          <p className="mt-1.5 text-rhd-md font-medium leading-5 text-[var(--railops-text-secondary)]">{subtitle}</p>
        </div>
        {headerExtra}
      </div>
      {children}
    </section>
  )
}

type RailOpsLoginShellProps = React.ComponentProps<"div"> & {
  showPlatformAdminEntry?: boolean
  showEnterpriseEntry?: boolean
  showCustomerEntry?: boolean
}

function RailOpsLoginShell({
  children,
  className,
  showPlatformAdminEntry = true,
  showEnterpriseEntry = false,
  showCustomerEntry = false,
  style,
  ...props
}: RailOpsLoginShellProps) {
  const t = useI18n()
  return (
    <div
      className={cn("rhd-railops-auth-shell relative min-h-svh overflow-x-hidden bg-[var(--railops-layout-background)] text-[var(--railops-text)]", className)}
      style={{ fontFamily: LOGIN_FONT_FAMILY, ...style }}
      {...props}
    >
      <LoginVisualBackdrop />
      <header className="relative z-10 flex items-start justify-between gap-4 px-5 pt-5 sm:px-8 sm:pt-6 xl:px-12 xl:pt-7">
        <LoginBrandHeader />
        <LoginLocaleSwitcher
          showPlatformAdminEntry={showPlatformAdminEntry}
          showEnterpriseEntry={showEnterpriseEntry}
          showCustomerEntry={showCustomerEntry}
        />
      </header>

      <main className="relative z-10 grid min-h-[calc(100svh-7rem)] gap-8 px-5 pb-8 sm:px-8 lg:grid-cols-[minmax(0,1fr)_420px] lg:items-center xl:px-12 xl:pb-10">
        <LoginHeroContent />
        <div className="flex min-w-0 items-center justify-center lg:justify-end">
          {children}
        </div>
      </main>

      <footer className="relative z-10 px-5 pb-4 text-center text-rhd-xs font-medium text-[var(--railops-text-tertiary)]">
        {t("auth.loginHero.copyright")}
      </footer>
    </div>
  )
}

type StaffPortalLoginLayoutProps = {
  isPending: boolean
  onForgotPassword: () => void
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void
  nextPath: string | null
  portal: Exclude<LoginPortal, "customer">
  refreshProfile: () => Promise<void>
}

function StaffPortalLoginLayout({
  isPending,
  onForgotPassword,
  onSubmit,
  nextPath,
  portal,
  refreshProfile,
}: StaffPortalLoginLayoutProps) {
  const t = useI18n()
  const [mode, setMode] = useState<"account" | "register">("account")
  const hasRegistration = portal !== "platform"
  const staffModeTabs: RailopsTabItem[] = [
    { value: "account", label: t("auth.accountLogin") },
    ...(hasRegistration ? [{ value: "register" as const, label: t("auth.applyTrial") }] : []),
  ]
  return (
    <RailOpsLoginShell
      showPlatformAdminEntry={portal !== "platform"}
      showEnterpriseEntry={portal === "platform"}
      showCustomerEntry={portal === "enterprise"}
    >
      <RailOpsLoginCard title={t("auth.loginHero.cardTitle")} subtitle={t("remoteTopbar.brandTitle")}>
        <div className="mt-5">
          <UnderlineTabs
            ariaLabel={t("auth.portalSelector")}
            items={staffModeTabs}
            value={mode}
            onChange={(value) => setMode(value as "account" | "register")}
          />

          <div className="mt-4">
            {hasRegistration && mode === "register" ? (
              <EnterpriseRegistrationForm nextPath={nextPath} refreshProfile={refreshProfile} />
            ) : (
              <StaffLoginForm key={portal} isPending={isPending} onSubmit={onSubmit} portal={portal} onForgotPassword={onForgotPassword} />
            )}
          </div>

          {hasRegistration ? (
            <div className="mt-5 border-t border-[var(--railops-border-light)] pt-4 text-center text-rhd-sm font-medium text-[var(--railops-text-secondary)]">
              {mode === "account" ? t("auth.noAccount") : t("auth.welcome")}{" "}
              <button type="button" className="font-semibold text-[var(--railops-primary-hover)] hover:text-[var(--railops-primary)]" onClick={() => setMode(mode === "account" ? "register" : "account")}>
                {mode === "account" ? t("auth.applyTrial") : t("auth.signIn")}
              </button>
            </div>
          ) : null}
        </div>
      </RailOpsLoginCard>
    </RailOpsLoginShell>
  )
}

function PartnerInvitationEntry({
  inviteCode,
  nextPath,
  refreshProfile,
}: {
  inviteCode: string
  nextPath: string | null
  refreshProfile: () => Promise<void>
}) {
  const t = useI18n()
  const [context, setContext] = useState<CustomerRegistrationContext | null>(null)
  const [verifying, setVerifying] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    let cancelled = false
    setVerifying(true)
    setError("")
    setContext(null)
    void verifyPortalInvitation("partner", inviteCode)
      .then((value) => {
        if (!cancelled) setContext(value)
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error && err.message ? err.message : t("auth.partner.invalidInvitation"))
      })
      .finally(() => {
        if (!cancelled) setVerifying(false)
      })
    return () => {
      cancelled = true
    }
  }, [inviteCode, t])

  async function register(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!context || submitting) return
    const formData = new FormData(event.currentTarget)
    const username = formData.get("username")?.toString().trim() ?? ""
    const displayName = formData.get("displayName")?.toString().trim() ?? ""
    const email = formData.get("email")?.toString().trim() ?? ""
    const password = formData.get("password")?.toString() ?? ""
    const confirmPassword = formData.get("confirmPassword")?.toString() ?? ""
    if (password !== confirmPassword) {
      setError(t("auth.customer.passwordMismatch"))
      return
    }
    if (password.length < 8) {
      setError(t("auth.customer.passwordLength"))
      return
    }
    setSubmitting(true)
    setError("")
    let navigating = false
    try {
      const session = await registerPortalInvitation({
        domainType: "partner",
        credential: inviteCode,
        username,
        displayName: displayName || context.displayName || username,
        email,
        password,
      })
      await refreshProfile()
      toast.success(t("auth.partner.registrationSuccess"))
      navigating = true
      navigateAfterAuth(resolveSessionDestination(session, nextPath))
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : t("auth.partner.registrationFailed"))
    } finally {
      if (!navigating) setSubmitting(false)
    }
  }

  return (
    <RailOpsLoginShell showPlatformAdminEntry>
      <RailOpsLoginCard title={t("auth.partner.invitationTitle")} subtitle={t("remoteTopbar.brandTitle")}>
        {verifying ? (
          <div className="mt-5 space-y-3.5">
            <div className="space-y-2.5">
              <Label htmlFor="partner-verifying-invite-code" className={authLabelClass}>{t("auth.customer.inviteCode")}</Label>
              <div className="relative">
                <Building2Icon className={authIconClass} />
                <Input id="partner-verifying-invite-code" className={`${authInputClass} pl-10`} value={inviteCode} readOnly aria-readonly="true" />
              </div>
            </div>
            <div className="flex items-center gap-2 rounded-md border border-[var(--railops-border-light)] bg-[var(--railops-surface)] p-3 text-rhd-sm font-medium text-[var(--railops-text-secondary)]">
              <Loader2Icon className="size-4 animate-spin" />
              {t("auth.customer.verifying")}
            </div>
          </div>
        ) : context?.registered ? (
          <InvitationBindForm
            context={context}
            credential={inviteCode}
            domainType="partner"
            nextPath={nextPath}
            refreshProfile={refreshProfile}
          />
        ) : context ? (
          <form key="partner-invitation-register" className="mt-4 space-y-3.5" onSubmit={register}>
            <RegistrationContextSummary context={context} t={t} />
            <div className="space-y-2.5">
              <Label htmlFor="partner-registration-invite-code" className={authLabelClass}>{t("auth.customer.inviteCode")}</Label>
              <div className="relative">
                <Building2Icon className={authIconClass} />
                <Input id="partner-registration-invite-code" className={`${authInputClass} pl-10`} value={inviteCode} readOnly aria-readonly="true" />
              </div>
            </div>
            <div className="space-y-2.5">
              <Label htmlFor="partner-registration-username" className={authLabelClass}>{t("auth.username")}</Label>
              <div className="relative"><UserRoundIcon className={authIconClass} /><Input id="partner-registration-username" name="username" className={`${authInputClass} pl-10`} placeholder={t("auth.customer.registrationUsernamePlaceholder")} autoComplete="username" required autoFocus /></div>
            </div>
            <div className="space-y-2.5">
              <Label htmlFor="partner-registration-display-name" className={authLabelClass}>{t("auth.displayName")}</Label>
              <Input id="partner-registration-display-name" name="displayName" className={authInputClass} defaultValue={context.displayName ?? ""} />
            </div>
            <div className="space-y-2.5">
              <Label htmlFor="partner-registration-email" className={authLabelClass}>{t("auth.customer.email")}</Label>
              <div className="relative"><MailIcon className={authIconClass} /><Input id="partner-registration-email" name="email" type="email" className={`${authInputClass} pl-10`} defaultValue={context.email ?? ""} readOnly aria-readonly="true" autoComplete="email" required /></div>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-2.5"><Label htmlFor="partner-registration-password" className={authLabelClass}>{t("auth.password")}</Label><Input id="partner-registration-password" name="password" type="password" className={authInputClass} placeholder={t("auth.customer.newPasswordPlaceholder")} autoComplete="new-password" minLength={8} required /></div>
              <div className="space-y-2.5"><Label htmlFor="partner-registration-confirm-password" className={authLabelClass}>{t("auth.customer.confirmPassword")}</Label><Input id="partner-registration-confirm-password" name="confirmPassword" type="password" className={authInputClass} placeholder={t("auth.customer.confirmPasswordPlaceholder")} autoComplete="new-password" minLength={8} required /></div>
            </div>
            {error ? <RegistrationError message={error} /> : null}
            <Button type="submit" size="lg" className={`${authPrimaryButtonClass} w-full`} disabled={submitting}>{submitting ? <Loader2Icon className="size-4 animate-spin" /> : <BadgeCheckIcon className="size-4" />}{submitting ? t("auth.partner.registering") : t("auth.partner.registerAndEnter")}</Button>
          </form>
        ) : (
          <div className="mt-5 space-y-3.5">
            <div className="space-y-2.5">
              <Label htmlFor="partner-invalid-invite-code" className={authLabelClass}>{t("auth.customer.inviteCode")}</Label>
              <div className="relative">
                <Building2Icon className={authIconClass} />
                <Input id="partner-invalid-invite-code" className={`${authInputClass} pl-10`} value={inviteCode} readOnly aria-readonly="true" />
              </div>
            </div>
            <RegistrationError message={error || t("auth.partner.invalidInvitation")} />
          </div>
        )}
      </RailOpsLoginCard>
    </RailOpsLoginShell>
  )
}

export function LoginForm({
  className,
  forcedPortal,
  ...props
}: React.ComponentProps<"div"> & { forcedPortal?: LoginPortal }) {
  const t = useI18n()
  const searchParams = useSearchParams()
  const { session, ready, refreshProfile } = useAuth()
  const [isPending, setIsPending] = useState(false)
  const [guestInfoOpen, setGuestInfoOpen] = useState(false)
  const [passwordResetOpen, setPasswordResetOpen] = useState(false)
  const [portalChoiceSession, setPortalChoiceSession] = useState<AuthSession | null>(null)
  const [pendingCredentials, setPendingCredentials] = useState<{ username: string; password: string } | null>(null)
  const [choicePending, setChoicePending] = useState(false)
  const nextPath = searchParams.get("next")
  const inviteFromURL = searchParams.get("invite")?.trim() ?? ""
  const requestedPortal = forcedPortal ?? resolveLoginPortal(searchParams.get("portal"), nextPath)
  const resolvedPortal = forcedPortal ? requestedPortal : requestedPortal === "platform" ? "enterprise" : requestedPortal
  const portal = resolvedPortal === "partner" ? "enterprise" : resolvedPortal
  const wxworkError = searchParams.get("wxworkError")
  const oidcError = searchParams.get("oidcError")

  useEffect(() => {
    if (ready && session && portal !== "customer" && !inviteFromURL && !isPending && !portalChoiceSession) {
      navigateAfterAuth(resolveSessionDestination(session, nextPath))
    }
  }, [inviteFromURL, isPending, nextPath, portal, portalChoiceSession, ready, session])

  useEffect(() => {
    if (wxworkError) toast.error(wxworkError)
  }, [wxworkError])

  useEffect(() => {
    if (oidcError) toast.error(oidcError)
  }, [oidcError])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const formData = new FormData(event.currentTarget)
    const username = formData.get("username")?.toString().trim() ?? ""
    const password = formData.get("password")?.toString() ?? ""

    setIsPending(true)
    let navigating = false
    try {
      const nextSession = await loginWithPassword({ username, password, domainType: portal })
      await refreshProfile()
      if (portal === "enterprise" && nextSession.requiresPortalChoice) {
        setPendingCredentials({ username, password })
        setPortalChoiceSession(nextSession)
        setIsPending(false)
        return
      }
      const sessionPortal = getSessionPortal(nextSession.domainType)
      toast.success(t("auth.loginSuccess"))
      if (sessionPortal !== portal) {
        toast.info(t("auth.portalAdjusted", { portal: t(PORTAL_PRESENTATIONS[sessionPortal].tabKey) }))
      }
      navigating = true
      navigateAfterAuth(resolveSessionDestination(nextSession, nextPath))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("auth.loginFailed"))
    } finally {
      if (!navigating) setIsPending(false)
    }
  }

  async function chooseBusinessPortal(domainType: "enterprise" | "partner") {
    if (!pendingCredentials || choicePending) return
    setChoicePending(true)
    let navigating = false
    try {
      const nextSession = await loginWithPassword({
        ...pendingCredentials,
        domainType,
        portalChoiceConfirmed: true,
      })
      await refreshProfile()
      toast.success(t("auth.loginSuccess"))
      navigating = true
      navigateAfterAuth(resolveSessionDestination(nextSession, nextPath))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("auth.loginFailed"))
    } finally {
      if (!navigating) setChoicePending(false)
    }
  }

  if (requestedPortal === "partner" && inviteFromURL) {
    return (
      <div className={className} {...props}>
        <PartnerInvitationEntry inviteCode={inviteFromURL} nextPath={nextPath} refreshProfile={refreshProfile} />
      </div>
    )
  }

  if (portal !== "customer") {
    return (
      <div className={className} {...props}>
        <PasswordResetDialog open={passwordResetOpen} onOpenChange={setPasswordResetOpen} />
        <BusinessPortalChoiceDialog
          open={Boolean(portalChoiceSession)}
          onOpenChange={(open) => {
            if (!open && !choicePending) setPortalChoiceSession(null)
          }}
          options={portalChoiceSession?.availablePortals ?? []}
          pending={choicePending}
          onSelect={chooseBusinessPortal}
        />
        <StaffPortalLoginLayout
          isPending={isPending}
          onForgotPassword={() => setPasswordResetOpen(true)}
          onSubmit={handleSubmit}
          nextPath={nextPath}
          portal={portal}
          refreshProfile={refreshProfile}
        />
      </div>
    )
  }

  return (
    <RailOpsLoginShell className={className} showPlatformAdminEntry={false} showEnterpriseEntry {...props}>
      <GuestDeviceInfoDialog open={guestInfoOpen} onOpenChange={setGuestInfoOpen} />
      <PasswordResetDialog open={passwordResetOpen} onOpenChange={setPasswordResetOpen} />
      <RailOpsLoginCard
        title={t("auth.loginHero.cardTitle")}
        subtitle={t("remoteTopbar.brandTitle")}
        headerExtra={(
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            className="shrink-0 text-[var(--railops-text-secondary)] hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)]"
            onClick={() => setGuestInfoOpen(true)}
            aria-label={t("auth.customer.guestInfo")}
            title={t("auth.customer.guestInfo")}
          >
            <InfoIcon className="size-4" />
          </Button>
        )}
      >
        <div className="mt-5">
          <CustomerEntryForm isPending={isPending} onAccountSubmit={handleSubmit} onForgotPassword={() => setPasswordResetOpen(true)} />
        </div>
      </RailOpsLoginCard>
    </RailOpsLoginShell>
  )
}
