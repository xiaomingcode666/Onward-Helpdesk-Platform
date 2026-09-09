"use client"

import { useEffect, useState } from "react"
import { CheckCircle2Icon, CheckIcon, CopyIcon, Loader2Icon, UserPlusIcon } from "lucide-react"

import { Input } from "antd"
import { RailopsButton, SelectField, StandardModal } from "@railops/ui"

import {
  fetchEnterpriseIAMRoles,
  fetchIAMRoles,
  inviteEnterpriseMember,
  inviteEnterpriseCustomerUser,
  inviteEnterprisePartnerAdmin,
  invitePlatformStaff,
  type EnterpriseCustomerInviteResult,
  type EnterprisePartnerAdminInviteResult,
  type IAMRole,
} from "@/lib/api/platform-iam"
import { useAppLocale, useI18n } from "@/i18n/provider"

export type IAMInviteKind = "platform-staff" | "enterprise-people" | "enterprise-customer-users" | "enterprise-partners"

type PortalInviteResult = EnterpriseCustomerInviteResult | EnterprisePartnerAdminInviteResult

const initialForm = {
  username: "",
  displayName: "",
  password: "",
  email: "",
  mobile: "",
  roleCode: "",
  jobTitle: "",
  teamCode: "",
  customerOrg: "",
  partnerName: "",
  partnerNo: "",
  countryRegion: "",
}

function isSelectableRole(role: IAMRole) {
  return role.status === 0 && !role.code.endsWith("_seed")
}

export function IAMInviteDialog({
  kind,
  tenantId,
  open,
  onOpenChange,
  onSaved,
}: {
  kind: IAMInviteKind
  tenantId: number
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const [form, setForm] = useState(initialForm)
  const [roles, setRoles] = useState<IAMRole[]>([])
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const [portalInvite, setPortalInvite] = useState<PortalInviteResult | null>(null)
  const [copied, setCopied] = useState(false)
  const dialogCopy = {
    "platform-staff": {
      title: t("iamExtract.dialog.invitePlatformStaff"),
      submit: t("iamExtract.dialog.createPlatformAccount"),
    },
    "enterprise-people": {
      title: t("iamExtract.dialog.inviteEnterpriseMember"),
      submit: t("iamExtract.dialog.submitEnterpriseInvitation"),
    },
    "enterprise-customer-users": {
      title: t("iamExtract.dialog.inviteCustomerUser"),
      submit: t("iamExtract.dialog.submitCustomerInvitation"),
    },
    "enterprise-partners": {
      title: t("iamExtract.dialog.invitePartnerAdmin"),
      submit: t("iamExtract.dialog.sendPartnerInvitation"),
    },
  }[kind]

  const domainType = kind === "platform-staff"
    ? "platform"
    : kind === "enterprise-customer-users"
      ? "customer"
      : kind === "enterprise-partners"
        ? "partner"
        : "enterprise"

  useEffect(() => {
    // 客户用户无角色概念,不加载角色列表
    if (!open || kind === "enterprise-partners" || kind === "enterprise-customer-users") return
    const loadRoles = kind === "platform-staff"
      ? fetchIAMRoles({ tenantId: 0, domainType })
      : fetchEnterpriseIAMRoles(tenantId, domainType)
    void loadRoles
      .then((items) => {
        const selectableRoles = items.filter(isSelectableRole)
        setRoles(selectableRoles)
        const preferred = kind === "platform-staff"
          ? "platform_operations"
          : "enterprise_viewer"
        setForm((value) => ({
          ...value,
          roleCode: selectableRoles.find((item) => item.code === preferred)?.code ?? selectableRoles[0]?.code ?? "",
        }))
      })
      .catch((err) => setError(err instanceof Error ? err.message : t("iamExtract.dialog.roleListLoadFailed")))
  }, [domainType, kind, open, t, tenantId])

  function close(nextOpen: boolean) {
    if (!saving) {
      onOpenChange(nextOpen)
      if (!nextOpen) {
        setForm(initialForm)
        setError("")
        setPortalInvite(null)
        setCopied(false)
      }
    }
  }

  async function submit() {
    if (kind === "enterprise-customer-users" && !form.email.trim()) {
      setError(t("iamExtract.dialog.validationCustomerEmail"))
      return
    }
    if (kind === "enterprise-partners" && (!form.email.trim() || !form.partnerName.trim())) {
      setError(t("iamExtract.dialog.validationPartnerEmail"))
      return
    }
    if (kind !== "enterprise-customer-users" && kind !== "enterprise-partners" && (!form.username.trim() || !form.displayName.trim())) {
      setError(t("iamExtract.dialog.validationLoginAccountName"))
      return
    }
    if (kind !== "enterprise-customer-users" && kind !== "enterprise-partners" && form.password.length < 8) {
      setError(t("iamExtract.dialog.passwordTooShort"))
      return
    }
    if (kind === "enterprise-customer-users" && !form.customerOrg.trim()) {
      setError(t("iamExtract.dialog.validationCustomerOrg"))
      return
    }
    if (kind === "enterprise-partners" && !form.partnerName.trim()) {
      setError(t("iamExtract.dialog.validationPartnerName"))
      return
    }
    setSaving(true)
    setError("")
    try {
      if (kind === "platform-staff") {
        await invitePlatformStaff({
          username: form.username.trim(), nickname: form.displayName.trim(), password: form.password,
          email: form.email.trim(), mobile: form.mobile.trim(), roleCodes: [form.roleCode],
          teamCode: form.teamCode.trim(), jobTitle: form.jobTitle.trim(), employmentType: "employee",
        })
      } else if (kind === "enterprise-people") {
        await inviteEnterpriseMember({
          username: form.username.trim(), displayName: form.displayName.trim(), password: form.password,
          email: form.email.trim(), mobile: form.mobile.trim(), jobTitle: form.jobTitle.trim(),
          memberType: "employee", roleCodes: [form.roleCode], dispatchEnabled: form.roleCode === "service_engineer",
        }, tenantId)
      } else if (kind === "enterprise-customer-users") {
        const invitation = await inviteEnterpriseCustomerUser({
          displayName: form.displayName.trim(), email: form.email.trim(), customerOrg: form.customerOrg.trim(),
          locale: "zh-CN", timezone: "Asia/Shanghai",
        }, tenantId)
        setPortalInvite(invitation)
        onSaved()
        return
      } else {
        const invitation = await inviteEnterprisePartnerAdmin({
          partnerName: form.partnerName.trim(), partnerNo: form.partnerNo.trim(), countryRegion: form.countryRegion.trim(),
          partnerType: "service_supplier", contactName: form.displayName.trim(), displayName: form.displayName.trim(),
          email: form.email.trim(), mobile: form.mobile.trim(),
        }, tenantId)
        setPortalInvite(invitation)
        onSaved()
        return
      }
      onSaved()
      close(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("iamExtract.dialog.accountCreateFailed"))
    } finally {
      setSaving(false)
    }
  }

  async function copyPortalInvitation() {
    if (!portalInvite) return
    const invitationURL = typeof window === "undefined"
      ? portalInvite.registrationUrl
      : new URL(portalInvite.registrationUrl, window.location.origin).toString()
    try {
      await navigator.clipboard.writeText(invitationURL)
      setCopied(true)
    } catch {
      setError(t("iamExtract.dialog.copyFailed"))
    }
  }

  return (
    <StandardModal
      open={open}
      onCancel={() => close(false)}
      title={<span className="flex items-center gap-2"><UserPlusIcon className="size-5" />{dialogCopy.title}</span>}
      width={576}
      footer={
        <>
          <RailopsButton onClick={() => close(false)} disabled={saving}>{portalInvite ? t("iamExtract.dialog.done") : t("iamExtract.dialog.cancel")}</RailopsButton>
          {!portalInvite ? <RailopsButton variant="primary" onClick={() => void submit()} disabled={saving || (kind !== "enterprise-partners" && kind !== "enterprise-customer-users" && !form.roleCode)}>
            {saving ? <Loader2Icon className="animate-spin" /> : <UserPlusIcon />}{saving ? t("iamExtract.dialog.processing") : dialogCopy.submit}
          </RailopsButton> : null}
        </>
      }
    >
      {portalInvite ? (
          <div className="space-y-4">
            <div className="flex items-start gap-3 rounded-md border border-primary/20 bg-primary/10 p-4">
              <CheckCircle2Icon className="mt-0.5 size-5 shrink-0 text-primary" />
              <div><div className="text-sm font-semibold text-primary">{t("iamExtract.dialog.inviteGenerated")}</div><div className="mt-1 text-xs leading-5 text-muted-foreground">{portalInvite.email} · {"customerOrgName" in portalInvite ? portalInvite.customerOrgName : portalInvite.partnerName}</div></div>
            </div>
            <label className="grid gap-1.5 text-sm font-medium text-slate-700">{t("iamExtract.dialog.inviteCode")}
              <Input readOnly value={portalInvite.inviteCode} className="font-mono text-sm" />
            </label>
            <label className="grid gap-1.5 text-sm font-medium text-slate-700">{t("iamExtract.dialog.inviteLink")}
              <div className="flex gap-2">
                <Input readOnly value={portalInvite.registrationUrl} className="font-mono text-xs" />
                <RailopsButton onClick={() => void copyPortalInvitation()}>{copied ? <CheckIcon /> : <CopyIcon />}{copied ? t("iamExtract.dialog.copied") : t("iamExtract.dialog.copy")}</RailopsButton>
              </div>
            </label>
            <p className="text-xs leading-5 text-muted-foreground">{t("iamExtract.dialog.inviteLinkExpires", { time: new Date(portalInvite.expiresAt).toLocaleString(locale, { hour12: false }) })}</p>
            {portalInvite.emailSent === false ? <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-700">{t("iamExtract.dialog.emailNotSent", { reason: portalInvite.emailError || t("iamExtract.dialog.smtpFallback") })}</div> : null}
            {error ? <div className="border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</div> : null}
          </div>
        ) : <form id="iam-invite-form" className="grid gap-4 sm:grid-cols-2" onSubmit={(event) => { event.preventDefault(); void submit() }}>
          {kind === "enterprise-partners" ? (
            <>
              <Field label={t("iamExtract.dialog.partnerName")}><Input value={form.partnerName} onChange={(event) => setForm((value) => ({ ...value, partnerName: event.target.value }))} autoFocus /></Field>
              <Field label={t("iamExtract.dialog.partnerNo")}><Input value={form.partnerNo} onChange={(event) => setForm((value) => ({ ...value, partnerNo: event.target.value }))} placeholder={t("iamExtract.dialog.leaveAutoGenerate")} /></Field>
              <Field label={t("iamExtract.dialog.region")} wide><Input value={form.countryRegion} onChange={(event) => setForm((value) => ({ ...value, countryRegion: event.target.value }))} /></Field>
            </>
          ) : null}
          {kind === "enterprise-customer-users" ? (
            <Field label={t("iamExtract.dialog.customerOrg")} wide><Input value={form.customerOrg} onChange={(event) => setForm((value) => ({ ...value, customerOrg: event.target.value }))} autoFocus placeholder={t("iamExtract.dialog.customerOrgPlaceholder")} /></Field>
          ) : null}
          {kind !== "enterprise-customer-users" && kind !== "enterprise-partners" ? <Field label={t("iamExtract.dialog.loginAccount")}><Input value={form.username} onChange={(event) => setForm((value) => ({ ...value, username: event.target.value }))} autoFocus autoComplete="off" /></Field> : null}
          <Field label={kind === "enterprise-customer-users" ? t("iamExtract.dialog.customerNameOptional") : kind === "enterprise-partners" ? t("iamExtract.dialog.contactNameOptional") : t("iamExtract.dialog.name")}><Input value={form.displayName} onChange={(event) => setForm((value) => ({ ...value, displayName: event.target.value }))} /></Field>
          <Field label={kind === "enterprise-customer-users" ? t("iamExtract.dialog.invitedEmail") : kind === "enterprise-partners" ? t("iamExtract.dialog.invitedEmail") : t("iamExtract.dialog.email")}><Input type="email" value={form.email} onChange={(event) => setForm((value) => ({ ...value, email: event.target.value }))} required={kind === "enterprise-customer-users" || kind === "enterprise-partners"} /></Field>
          {kind !== "enterprise-customer-users" ? <Field label={t("iamExtract.dialog.mobile")}><Input value={form.mobile} onChange={(event) => setForm((value) => ({ ...value, mobile: event.target.value }))} /></Field> : null}
          {kind === "platform-staff" ? <Field label={t("iamExtract.dialog.platformTeam")}><Input value={form.teamCode} onChange={(event) => setForm((value) => ({ ...value, teamCode: event.target.value }))} placeholder="operations" /></Field> : null}
          {kind === "platform-staff" || kind === "enterprise-people" ? <Field label={t("iamExtract.dialog.jobTitle")}><Input value={form.jobTitle} onChange={(event) => setForm((value) => ({ ...value, jobTitle: event.target.value }))} /></Field> : null}
          {kind !== "enterprise-partners" && kind !== "enterprise-customer-users" ? (
            <SelectField
              label={t("iamExtract.dialog.role")}
              className="sm:col-span-2"
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("iamExtract.dialog.role"),
                placeholder: t("iamExtract.dialog.selectRole"),
                value: form.roleCode || undefined,
                onChange: (value) => setForm((current) => ({ ...current, roleCode: (value as string) ?? "" })),
                options: roles.map((role) => ({ value: role.code, label: role.name })),
                style: { width: "100%" },
              }}
            />
          ) : null}
          {kind !== "enterprise-customer-users" && kind !== "enterprise-partners" ? <Field label={t("iamExtract.dialog.initialPassword")} wide><Input type="password" value={form.password} onChange={(event) => setForm((value) => ({ ...value, password: event.target.value }))} autoComplete="new-password" placeholder={t("iamExtract.dialog.passwordMinLength")} /></Field> : null}
          {error ? <div className="border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive sm:col-span-2">{error}</div> : null}
        </form>}
    </StandardModal>
  )
}

function Field({ label, wide, children }: { label: string; wide?: boolean; children: React.ReactNode }) {
  return <label className={`grid gap-1.5 text-sm font-medium text-slate-700${wide ? " sm:col-span-2" : ""}`}>{label}{children}</label>
}
