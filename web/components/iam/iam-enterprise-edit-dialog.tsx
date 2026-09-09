"use client"

import { useEffect, useMemo, useState, type ReactNode } from "react"
import { Loader2Icon, PencilIcon } from "lucide-react"

import { Input } from "antd"
import { RailopsButton, SelectField, StandardModal } from "@railops/ui"

import {
  updateEnterpriseCustomerUser,
  updateEnterpriseMember,
  updateEnterprisePartner,
  type EnterpriseIAMCustomerUser,
  type EnterpriseIAMMember,
  type EnterpriseIAMPartner,
  type IAMRole,
} from "@/lib/api/platform-iam"
import { useI18n } from "@/i18n/provider"

export type IAMEnterpriseEditTarget =
  | { kind: "member"; item: EnterpriseIAMMember }
  | { kind: "customer"; item: EnterpriseIAMCustomerUser }
  | { kind: "partner"; item: EnterpriseIAMPartner }

const initialForm = {
  displayName: "",
  email: "",
  mobile: "",
  jobTitle: "",
  memberType: "employee",
  roleCode: "",
  dispatchEnabled: false,
  customerOrg: "",
  locale: "zh-CN",
  timezone: "Asia/Shanghai",
  partnerNo: "",
  partnerName: "",
  partnerType: "service_supplier",
  countryRegion: "",
  contactName: "",
  status: "0",
  dispatchTouched: false,
}

export function IAMEnterpriseEditDialog({
  open,
  roles = [],
  target,
  tenantId,
  onOpenChange,
  onSaved,
}: {
  open: boolean
  roles?: IAMRole[]
  target: IAMEnterpriseEditTarget | null
  tenantId: number
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const t = useI18n()
  const [form, setForm] = useState(initialForm)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const selectableRoles = useMemo(() => roles.filter((role) => role.status === 0 && !role.code.endsWith("_seed")), [roles])

  useEffect(() => {
    if (!open || !target) return
    if (target.kind === "member") {
      setForm({
        ...initialForm,
        displayName: target.item.display_name || target.item.username || "",
        email: target.item.email || "",
        mobile: target.item.mobile || "",
        jobTitle: target.item.job_title || "",
        memberType: target.item.member_type || "employee",
        roleCode: target.item.roles[0] || selectableRoles[0]?.code || "",
        dispatchEnabled: target.item.dispatch_enabled,
        status: String(target.item.status),
      })
    } else if (target.kind === "customer") {
      // 客户用户无角色概念,不读取角色
      setForm({
        ...initialForm,
        displayName: target.item.display_name || "",
        email: target.item.email || "",
        mobile: target.item.phone || "",
        customerOrg: target.item.customer_org_name || "",
        locale: target.item.locale || "zh-CN",
        timezone: target.item.timezone || "Asia/Shanghai",
        status: String(target.item.status),
      })
    } else {
      setForm({
        ...initialForm,
        partnerNo: target.item.partner_no || "",
        partnerName: target.item.name || "",
        partnerType: target.item.partner_type || "service_supplier",
        countryRegion: target.item.country_region || "",
        contactName: target.item.contact_name || "",
        status: String(target.item.status),
      })
    }
    setError("")
  }, [open, selectableRoles, target])

  const title = target?.kind === "member" ? t("iamExtract.dialog.editEnterpriseMember") : target?.kind === "customer" ? t("iamExtract.dialog.editCustomerUser") : t("iamExtract.dialog.editPartner")

  function close(next: boolean) {
    if (saving) return
    onOpenChange(next)
    if (!next) {
      setForm(initialForm)
      setError("")
    }
  }

  async function submit() {
    if (!target) return
    if (target.kind !== "partner" && !form.displayName.trim()) {
      setError(t("iamExtract.dialog.validationName"))
      return
    }
    if (target.kind === "customer" && !form.customerOrg.trim()) {
      setError(t("iamExtract.dialog.validationCustomerOrg"))
      return
    }
    if (target.kind === "partner" && !form.partnerName.trim()) {
      setError(t("iamExtract.dialog.validationPartnerName"))
      return
    }
    setSaving(true)
    setError("")
    try {
      if (target.kind === "member") {
        await updateEnterpriseMember(target.item.id, {
          displayName: form.displayName.trim(),
          email: form.email.trim(),
          mobile: form.mobile.trim(),
          jobTitle: form.jobTitle.trim(),
          memberType: form.memberType.trim() || "employee",
          roleCodes: form.roleCode ? [form.roleCode] : undefined,
          dispatchEnabled: form.dispatchTouched ? form.dispatchEnabled : undefined,
          status: Number(form.status),
        }, tenantId)
      } else if (target.kind === "customer") {
        // 客户用户无角色概念,不提交角色
        await updateEnterpriseCustomerUser(target.item.id, {
          displayName: form.displayName.trim(),
          email: form.email.trim(),
          mobile: form.mobile.trim(),
          customerOrg: form.customerOrg.trim(),
          locale: form.locale.trim() || "zh-CN",
          timezone: form.timezone.trim() || "Asia/Shanghai",
          status: Number(form.status),
        }, tenantId)
      } else {
        await updateEnterprisePartner(target.item.id, {
          partnerNo: form.partnerNo.trim(),
          partnerName: form.partnerName.trim(),
          partnerType: form.partnerType.trim() || "service_supplier",
          countryRegion: form.countryRegion.trim(),
          contactName: form.contactName.trim(),
          status: Number(form.status),
        }, tenantId)
      }
      onSaved()
      close(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("iamExtract.dialog.saveFailed"))
    } finally {
      setSaving(false)
    }
  }

  return (
    <StandardModal
      open={open}
      onCancel={() => close(false)}
      title={<span className="flex items-center gap-2"><PencilIcon className="size-5" />{title}</span>}
      width={576}
      footer={
        <>
          <RailopsButton onClick={() => close(false)} disabled={saving}>{t("iamExtract.dialog.cancel")}</RailopsButton>
          <RailopsButton variant="primary" onClick={() => void submit()} disabled={saving}>{saving ? <Loader2Icon className="animate-spin" /> : <PencilIcon />}{saving ? t("iamExtract.dialog.saving") : t("iamExtract.dialog.saveChanges")}</RailopsButton>
        </>
      }
    >
      <form id="enterprise-iam-edit-form" className="grid gap-4 sm:grid-cols-2" onSubmit={(event) => { event.preventDefault(); void submit() }}>
          {target?.kind === "partner" ? (
            <>
              <Field label={t("iamExtract.dialog.partnerName")}><Input value={form.partnerName} onChange={(event) => setForm((value) => ({ ...value, partnerName: event.target.value }))} autoFocus /></Field>
              <Field label={t("iamExtract.dialog.partnerNo")}><Input value={form.partnerNo} onChange={(event) => setForm((value) => ({ ...value, partnerNo: event.target.value }))} /></Field>
              <Field label={t("iamExtract.dialog.partnerType")}><Input value={form.partnerType} onChange={(event) => setForm((value) => ({ ...value, partnerType: event.target.value }))} /></Field>
              <Field label={t("iamExtract.dialog.region")}><Input value={form.countryRegion} onChange={(event) => setForm((value) => ({ ...value, countryRegion: event.target.value }))} /></Field>
              <Field label={t("iamExtract.dialog.fieldContactName")}><Input value={form.contactName} onChange={(event) => setForm((value) => ({ ...value, contactName: event.target.value }))} /></Field>
            </>
          ) : (
            <>
              <Field label={t("iamExtract.dialog.name")}><Input value={form.displayName} onChange={(event) => setForm((value) => ({ ...value, displayName: event.target.value }))} autoFocus /></Field>
              <Field label={t("iamExtract.dialog.email")}><Input type="email" value={form.email} onChange={(event) => setForm((value) => ({ ...value, email: event.target.value }))} /></Field>
              {target?.kind === "customer" ? (
                <>
                  <Field label={t("iamExtract.dialog.mobile")}><Input value={form.mobile} onChange={(event) => setForm((value) => ({ ...value, mobile: event.target.value }))} /></Field>
                  <Field label={t("iamExtract.dialog.customerOrg")}><Input value={form.customerOrg} onChange={(event) => setForm((value) => ({ ...value, customerOrg: event.target.value }))} /></Field>
                  <Field label={t("iamExtract.dialog.language")}><Input value={form.locale} onChange={(event) => setForm((value) => ({ ...value, locale: event.target.value }))} /></Field>
                  <Field label={t("iamExtract.dialog.timezone")}><Input value={form.timezone} onChange={(event) => setForm((value) => ({ ...value, timezone: event.target.value }))} /></Field>
                </>
              ) : (
                <>
                  <Field label={t("iamExtract.dialog.mobile")}><Input value={form.mobile} onChange={(event) => setForm((value) => ({ ...value, mobile: event.target.value }))} /></Field>
                  <Field label={t("iamExtract.dialog.jobTitle")}><Input value={form.jobTitle} onChange={(event) => setForm((value) => ({ ...value, jobTitle: event.target.value }))} /></Field>
                  <SelectField
                    label={t("iamExtract.dialog.employmentType")}
                    style={{ marginBottom: 0 }}
                    selectProps={{
                      "aria-label": t("iamExtract.dialog.employmentType"),
                      value: form.memberType,
                      onChange: (value) => setForm((current) => ({ ...current, memberType: (value as string) ?? "employee" })),
                      options: [
                        { value: "employee", label: t("iamExtract.dialog.employee") },
                        { value: "contractor", label: t("iamExtract.dialog.externalCollaborator") },
                        { value: "owner", label: t("iamExtract.dialog.enterpriseOwner") },
                      ],
                      style: { width: "100%" },
                    }}
                  />
                  <SelectField
                    label={t("iamExtract.dialog.dispatch")}
                    style={{ marginBottom: 0 }}
                    selectProps={{
                      "aria-label": t("iamExtract.dialog.dispatch"),
                      value: form.dispatchEnabled ? "1" : "0",
                      onChange: (value) => setForm((current) => ({ ...current, dispatchEnabled: (value as string) === "1", dispatchTouched: true })),
                      options: [
                        { value: "1", label: t("iamExtract.dialog.dispatchEnabled") },
                        { value: "0", label: t("iamExtract.dialog.dispatchDisabled") },
                      ],
                      style: { width: "100%" },
                    }}
                  />
                </>
              )}
              {target?.kind !== "customer" ? (
                <SelectField
                  label={t("iamExtract.dialog.role")}
                  className="sm:col-span-2"
                  style={{ marginBottom: 0 }}
                  selectProps={{
                    "aria-label": t("iamExtract.dialog.role"),
                    placeholder: t("iamExtract.dialog.selectRole"),
                    value: form.roleCode || undefined,
                    onChange: (value) => setForm((current) => ({ ...current, roleCode: (value as string) ?? "" })),
                    options: selectableRoles.map((role) => ({ value: role.code, label: role.name })),
                    style: { width: "100%" },
                  }}
                />
              ) : null}
            </>
          )}
          <SelectField
            label={t("iamExtract.dialog.status")}
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("iamExtract.dialog.status"),
              value: form.status,
              onChange: (value) => setForm((current) => ({ ...current, status: (value as string) ?? "0" })),
              options: [
                { value: "0", label: t("iamExtract.common.enabled") },
                { value: "1", label: t("iamExtract.common.disabled") },
              ],
              style: { width: "100%" },
            }}
          />
          {error ? <div className="border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive sm:col-span-2">{error}</div> : null}
      </form>
    </StandardModal>
  )
}

function Field({ label, wide, children }: { label: string; wide?: boolean; children: ReactNode }) {
  return <label className={`grid gap-1.5 text-sm font-medium text-slate-700${wide ? " sm:col-span-2" : ""}`}>{label}{children}</label>
}
