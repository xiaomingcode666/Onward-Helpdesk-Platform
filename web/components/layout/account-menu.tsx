"use client"

import { useEffect, useState, type ReactNode } from "react"
import {
  ArrowLeftToLineIcon,
  ChevronDownIcon,
  KeyRoundIcon,
  LanguagesIcon,
  Loader2Icon,
  LogOutIcon,
  SaveIcon,
  UserRoundIcon,
} from "lucide-react"
import { toast } from "sonner"

import { SelectField, StandardModal } from "@railops/ui"
import { useAuth } from "@/components/auth-provider"
import { ChangePasswordDialog } from "@/components/change-password-dialog"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  SELECTABLE_LOCALES,
  normalizeLocale,
  normalizeSelectableLocale,
  type AppLocale,
  type SelectableLocale,
} from "@/i18n/config"
import { useAppLocale } from "@/i18n/provider"
import { updateOwnProfile } from "@/lib/api/auth"
import {
  getAccountDomainLabel,
  getAccountRoleLabel,
  getAccountSubjectLabel,
} from "@/lib/account-context-i18n"
import { restoreDelegatedReturnSession } from "@/lib/auth"

type AccountMenuProps = {
  domain: "platform" | "enterprise" | "partner" | "customer"
}

const PROFILE_LOCALE_LABELS: Record<SelectableLocale, string> = {
  "zh-CN": "简体中文",
  "en-US": "English",
}

function nullableFormValue(value: string) {
  const normalized = value.trim()
  return normalized ? normalized : null
}

export function AccountMenu({ domain }: AccountMenuProps) {
  const { locale, setLocale, t } = useAppLocale()
  const { session, signOut, refreshProfile } = useAuth()
  const [profileOpen, setProfileOpen] = useState(false)
  const [profileSaving, setProfileSaving] = useState(false)
  const [profileForm, setProfileForm] = useState({
    nickname: "",
    email: "",
    mobile: "",
  })
  const [languageOpen, setLanguageOpen] = useState(false)
  const [languageSaving, setLanguageSaving] = useState(false)
  const [languageValue, setLanguageValue] = useState<AppLocale>(normalizeSelectableLocale(locale))
  const [changePasswordOpen, setChangePasswordOpen] = useState(false)
  const user = session?.user
  const displayName = user?.nickname || user?.username || t("remoteShell.account")
  const fallback = displayName.slice(0, 1).toUpperCase() || "U"
  const isEmployeeSupportMode = session?.supportMode === "employee_portal"
  const delegatedModeLabel = session?.domainType === "customer"
    ? t("account.customerSupportMode")
    : isEmployeeSupportMode
      ? t("account.employeeSupportMode")
      : t("account.platformSupportMode")
  const delegatedReturnLabel = session?.domainType === "customer" || isEmployeeSupportMode
    ? t("account.returnEnterprise")
    : t("account.returnPlatform")
  const accountTypeLabel = session?.supportGrantId
    ? delegatedModeLabel
    : domain === "customer"
      ? t("railopsExtract.account.customerAccount")
    : session?.roles?.[0]
      ? getAccountRoleLabel(session.roles[0], locale)
      : getAccountDomainLabel(session?.domainType || domain, locale)

  function returnFromDelegatedSession() {
    const restored = restoreDelegatedReturnSession()
    if (!restored) {
      void signOut()
      return
    }
    if (restored.domainType === "platform") {
      window.location.assign("/platform/tenants")
      return
    }
    if (restored.domainType === "enterprise") {
      window.location.assign(isEmployeeSupportMode ? "/enterprise/people" : "/enterprise/customer-users")
      return
    }
    window.location.assign("/enterprise")
  }

  useEffect(() => {
    if (!profileOpen) return
    setProfileForm({
      nickname: user?.nickname || "",
      email: user?.email || "",
      mobile: user?.mobile || "",
    })
  }, [profileOpen, user?.email, user?.mobile, user?.nickname])

  useEffect(() => {
    if (!languageOpen) return
    setLanguageValue(normalizeSelectableLocale(user?.locale || session?.locale || locale))
  }, [languageOpen, locale, session?.locale, user?.locale])

  function currentTimezone() {
    return user?.timezone || session?.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Shanghai"
  }

  function currentProfileLocale() {
    return normalizeLocale(user?.locale || session?.locale || locale)
  }

  async function handleProfileSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setProfileSaving(true)
    try {
      await updateOwnProfile({
        nickname: profileForm.nickname,
        avatar: user?.avatar || "",
        email: nullableFormValue(profileForm.email),
        mobile: nullableFormValue(profileForm.mobile),
        locale: currentProfileLocale(),
        timezone: currentTimezone(),
      })
      await refreshProfile()
      toast.success(t("account.profileSaved"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("account.profileSaveFailed"))
    } finally {
      setProfileSaving(false)
    }
  }

  async function handleLanguageSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const nextLocale = normalizeSelectableLocale(languageValue)
    setLanguageSaving(true)
    try {
      const nextSession = await updateOwnProfile({
        nickname: user?.nickname || "",
        avatar: user?.avatar || "",
        email: user?.email ? user.email : null,
        mobile: user?.mobile ? user.mobile : null,
        locale: nextLocale,
        timezone: currentTimezone(),
      })
      await refreshProfile()
      setLocale(normalizeLocale(nextSession.user?.locale || nextLocale))
      setLanguageOpen(false)
      toast.success(t("account.languageSaved"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("account.languageSaveFailed"))
    } finally {
      setLanguageSaving(false)
    }
  }

  const isPlatformAdmin = (session?.roles || []).some((role) => role === "super_admin" || role === "platform_admin")
  const identityRows: [string, ReactNode][] = [
    ...(domain === "customer" ? [] : [
      [t("account.userId"), user?.id ? String(user.id) : t("account.notSet")] as [string, ReactNode],
      [t("account.domain"), getAccountDomainLabel(session?.domainType || domain, locale)] as [string, ReactNode],
      ...(isPlatformAdmin ? [] : [
        [t("account.tenant"), session?.tenantId ? String(session.tenantId) : t("account.notSet")] as [string, ReactNode],
      ]),
      [
        t("account.subject"),
        session?.subjectType
          ? getAccountSubjectLabel(session.subjectType, session.subjectId, locale)
          : t("account.notSet"),
      ] as [string, ReactNode],
      [
        t("account.roles"),
        (session?.roles || []).length > 0 ? (
          <span className="flex flex-wrap gap-1.5">
            {session?.roles.map((role) => (
              <Badge key={role} variant="secondary">{getAccountRoleLabel(role, locale)}</Badge>
            ))}
          </span>
        ) : t("account.notSet"),
      ] as [string, ReactNode],
    ]),
  ]

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <button
              type="button"
              className="flex h-8 min-w-8 items-center gap-1.5 rounded-md border border-[var(--railops-border)] bg-[var(--railops-surface)] px-1.5 text-left shadow-[var(--railops-card-shadow)] transition hover:border-[#bfdbfe] hover:bg-[var(--railops-primary-bg)] sm:min-w-36 sm:px-2"
              aria-label={t("account.openMenu")}
            />
          }
        >
          <Avatar className="size-6 rounded-md">
            <AvatarImage src={user?.avatar || ""} alt={displayName} />
            <AvatarFallback className="rounded-md bg-[var(--railops-primary)] text-xs text-white">{fallback}</AvatarFallback>
          </Avatar>
          <span className="hidden min-w-0 flex-1 sm:block">
            <span className="block truncate text-xs font-semibold text-foreground">{displayName}</span>
            <span className="block truncate text-rhd-2xs leading-3 text-muted-foreground">
              {accountTypeLabel}
            </span>
          </span>
            <ChevronDownIcon className="hidden size-3 text-[var(--railops-text-secondary)] sm:block" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-64">
          <DropdownMenuGroup>
            <DropdownMenuLabel className="px-2 py-2 font-normal">
              <div className="truncate text-sm font-semibold text-foreground">{displayName}</div>
              <div className="truncate text-xs text-muted-foreground">{user?.username || "-"}</div>
            </DropdownMenuLabel>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={() => setProfileOpen(true)}>
            <UserRoundIcon />
            {t("account.profile")}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setLanguageOpen(true)}>
            <LanguagesIcon />
            {t("account.languageSettings")}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setChangePasswordOpen(true)}>
            <KeyRoundIcon />
            {t("account.changePassword")}
          </DropdownMenuItem>
          {session?.supportGrantId ? (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={returnFromDelegatedSession}>
                <ArrowLeftToLineIcon />
                {delegatedReturnLabel}
              </DropdownMenuItem>
            </>
          ) : null}
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={() => void signOut()}>
            <LogOutIcon />
            {t("nav.signOut")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <StandardModal
        open={profileOpen}
        onCancel={() => setProfileOpen(false)}
        width={512}
        title={t("account.profile")}
        footer={null}
      >
          <form className="space-y-4" onSubmit={handleProfileSubmit}>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1.5 sm:col-span-2">
                <Label htmlFor="account-profile-username">{t("account.username")}</Label>
                <Input id="account-profile-username" value={user?.username || ""} disabled />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="account-profile-nickname">{t("account.nickname")}</Label>
                <Input
                  id="account-profile-nickname"
                  value={profileForm.nickname}
                  onChange={(event) => setProfileForm((current) => ({ ...current, nickname: event.target.value }))}
                  placeholder={t("account.notSet")}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="account-profile-email">{t("account.email")}</Label>
                <Input
                  id="account-profile-email"
                  type="email"
                  value={profileForm.email}
                  onChange={(event) => setProfileForm((current) => ({ ...current, email: event.target.value }))}
                  placeholder={t("account.notSet")}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="account-profile-mobile">{t("account.mobile")}</Label>
                <Input
                  id="account-profile-mobile"
                  value={profileForm.mobile}
                  onChange={(event) => setProfileForm((current) => ({ ...current, mobile: event.target.value }))}
                  placeholder={t("account.notSet")}
                />
              </div>
            </div>
            <div className="flex justify-end">
              <Button type="submit" disabled={profileSaving}>
                {profileSaving ? <Loader2Icon className="size-4 animate-spin" /> : <SaveIcon className="size-4" />}
                {profileSaving ? t("account.savingProfile") : t("account.saveProfile")}
              </Button>
            </div>
          </form>
          {identityRows.length > 0 ? (
            <dl className="divide-y divide-border border-y border-border">
              {identityRows.map(([label, value]) => (
                <div key={label} className="grid grid-cols-[112px_minmax(0,1fr)] gap-3 py-2.5 text-sm">
                  <dt className="text-muted-foreground">{label}</dt>
                  <dd className="min-w-0 break-words font-medium text-foreground">{value}</dd>
                </div>
              ))}
            </dl>
          ) : null}
          {session?.supportGrantId ? (
            <div className="border-l-2 border-primary/20 bg-muted px-3 py-2 text-xs text-muted-foreground">
              {delegatedModeLabel} · #{session.supportGrantId} · {session.impersonatedBy || user?.username}
            </div>
          ) : null}
      </StandardModal>

      <StandardModal
        open={languageOpen}
        onCancel={() => setLanguageOpen(false)}
        width={448}
        title={t("account.languageSettings")}
        footer={null}
      >
          <form className="space-y-4" onSubmit={handleLanguageSubmit}>
            <SelectField
              label={t("account.locale")}
              htmlFor="account-language-locale"
              style={{ marginBottom: 0 }}
              selectProps={{
                id: "account-language-locale",
                value: languageValue,
                onChange: (value) => setLanguageValue(normalizeSelectableLocale(value)),
                prefix: <LanguagesIcon className="size-4 text-muted-foreground" />,
                options: SELECTABLE_LOCALES.map((item) => ({
                  value: item,
                  label: PROFILE_LOCALE_LABELS[item],
                })),
                style: { width: "100%" },
              }}
            />
            <div className="flex justify-end">
              <Button type="submit" disabled={languageSaving}>
                {languageSaving ? <Loader2Icon className="size-4 animate-spin" /> : <SaveIcon className="size-4" />}
                {languageSaving ? t("account.savingLanguage") : t("account.saveLanguage")}
              </Button>
            </div>
          </form>
      </StandardModal>

      <ChangePasswordDialog
        open={changePasswordOpen}
        onOpenChange={setChangePasswordOpen}
        onSuccess={signOut}
      />
    </>
  )
}
