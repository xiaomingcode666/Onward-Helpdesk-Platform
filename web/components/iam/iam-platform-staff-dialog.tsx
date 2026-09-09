"use client"

import { useEffect, useState, type ReactNode } from "react"
import { Loader2Icon, PencilIcon } from "lucide-react"

import { Input } from "antd"
import { RailopsButton, SelectField, StandardModal } from "@railops/ui"

import { updatePlatformStaff, type PlatformStaff } from "@/lib/api/platform-iam"
import { useI18n } from "@/i18n/provider"

const initialForm = {
  nickname: "",
  email: "",
  mobile: "",
  teamCode: "",
  jobTitle: "",
  supportLevel: "",
  employmentType: "employee",
  status: "0",
}

export function IAMPlatformStaffDialog({
  staff,
  open,
  onOpenChange,
  onSaved,
}: {
  staff: PlatformStaff | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const t = useI18n()
  const [form, setForm] = useState(initialForm)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!staff || !open) return
    setForm({
      nickname: staff.displayName || staff.username || "",
      email: staff.email || "",
      mobile: staff.mobile || "",
      teamCode: staff.teamCode || "",
      jobTitle: staff.jobTitle || "",
      supportLevel: staff.supportLevel || "",
      employmentType: staff.employmentType || "employee",
      status: String(staff.status),
    })
    setError("")
  }, [open, staff])

  function close(next: boolean) {
    if (saving) return
    onOpenChange(next)
    if (!next) {
      setForm(initialForm)
      setError("")
    }
  }

  async function submit() {
    if (!staff) return
    if (!form.nickname.trim()) {
      setError(t("iamExtract.dialog.validationPlatformStaffName"))
      return
    }
    setSaving(true)
    setError("")
    try {
      await updatePlatformStaff({
        platformStaffId: staff.id,
        nickname: form.nickname.trim(),
        email: form.email.trim(),
        mobile: form.mobile.trim(),
        teamCode: form.teamCode.trim(),
        jobTitle: form.jobTitle.trim(),
        supportLevel: form.supportLevel.trim(),
        employmentType: form.employmentType.trim() || "employee",
        status: Number(form.status),
      })
      onSaved()
      close(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("iamExtract.dialog.platformStaffSaveFailed"))
    } finally {
      setSaving(false)
    }
  }

  return (
    <StandardModal
      open={open}
      onCancel={() => close(false)}
      title={<span className="flex items-center gap-2"><PencilIcon className="size-5" />{t("iamExtract.dialog.editPlatformStaff")}</span>}
      width={576}
      footer={
        <>
          <RailopsButton onClick={() => close(false)} disabled={saving}>{t("iamExtract.dialog.cancel")}</RailopsButton>
          <RailopsButton variant="primary" onClick={() => void submit()} disabled={saving}>{saving ? <Loader2Icon className="animate-spin" /> : <PencilIcon />}{saving ? t("iamExtract.dialog.saving") : t("iamExtract.dialog.saveChanges")}</RailopsButton>
        </>
      }
    >
      <form id="platform-staff-edit-form" className="grid gap-4 sm:grid-cols-2" onSubmit={(event) => { event.preventDefault(); void submit() }}>
          <Field label={t("iamExtract.dialog.name")}>
            <Input value={form.nickname} onChange={(event) => setForm((value) => ({ ...value, nickname: event.target.value }))} autoFocus />
          </Field>
          <Field label={t("iamExtract.dialog.email")}>
            <Input type="email" value={form.email} onChange={(event) => setForm((value) => ({ ...value, email: event.target.value }))} />
          </Field>
          <Field label={t("iamExtract.dialog.mobile")}>
            <Input value={form.mobile} onChange={(event) => setForm((value) => ({ ...value, mobile: event.target.value }))} />
          </Field>
          <Field label={t("iamExtract.dialog.platformTeam")}>
            <Input value={form.teamCode} onChange={(event) => setForm((value) => ({ ...value, teamCode: event.target.value }))} placeholder="operations" />
          </Field>
          <Field label={t("iamExtract.dialog.jobTitle")}>
            <Input value={form.jobTitle} onChange={(event) => setForm((value) => ({ ...value, jobTitle: event.target.value }))} />
          </Field>
          <Field label={t("iamExtract.dialog.supportLevel")}>
            <Input value={form.supportLevel} onChange={(event) => setForm((value) => ({ ...value, supportLevel: event.target.value }))} placeholder="administrator" />
          </Field>
          <SelectField
            label={t("iamExtract.dialog.employmentType")}
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("iamExtract.dialog.employmentType"),
              value: form.employmentType,
              onChange: (value) => setForm((current) => ({ ...current, employmentType: (value as string) ?? "employee" })),
              options: [
                { value: "employee", label: t("iamExtract.dialog.employee") },
                { value: "contractor", label: t("iamExtract.dialog.externalCollaborator") },
                { value: "auditor", label: t("iamExtract.dialog.auditor") },
              ],
              style: { width: "100%" },
            }}
          />
          <SelectField
            label={t("iamExtract.dialog.accountStatus")}
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("iamExtract.dialog.accountStatus"),
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

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="grid gap-1.5 text-sm font-medium text-slate-700">{label}{children}</label>
}
