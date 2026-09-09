"use client"

import { useEffect, useMemo, useState } from "react"
import { KeyRoundIcon, Loader2Icon } from "lucide-react"

import { Checkbox as AntCheckbox, Input } from "antd"
import { RailopsButton, SearchField, SelectField, StandardModal, StatusTag } from "@railops/ui"
import { useAppLocale, useI18n } from "@/i18n/provider"
import type { AdminPermission } from "@/lib/api/admin"
import { saveIAMRole, saveIAMRolePolicy, type IAMRole } from "@/lib/api/platform-iam"
import { getPermissionDisplayName, getPermissionGroupName } from "@/lib/permission-i18n"

const permissionPresets = {
  read_only: [
    "product.view", "productModel.view", "device.view", "customer.view", "conversation.view",
    "ticket.view", "ticket.repairHistory.view", "meeting.view", "notification.view",
  ],
  service_operations: [
    "product.view", "productModel.view", "device.view", "device.update", "customer.view",
    "conversation.view", "conversation.send", "conversation.assign", "ticket.view", "ticket.create",
    "ticket.update", "ticket.assign", "ticket.changeStatus", "ticket.progress", "meeting.view", "meeting.create",
    "notification.view", "notification.update",
  ],
  iam_manager: [
    "user.view", "user.create", "user.update", "user.assignRole", "role.view", "role.create",
    "role.update", "role.assignPermission", "permission.view", "session.view", "session.revoke",
  ],
  finance_management: [
    "tenant.view", "report.view", "report.export", "finance.view", "finance.manage",
    "aiConfig.view", "productAIUsageCredential.view", "notification.view",
  ],
} as const

type Preset = keyof typeof permissionPresets

const iamRoleLabelClass = "grid gap-1.5 text-sm font-medium text-foreground"
const iamRoleMutedTextClass = "text-muted-foreground"

export function IAMRoleDialog({
  domainType,
  tenantId,
  open,
  onOpenChange,
  onSaved,
  permissionCatalog = [],
  role = null,
}: {
  domainType: "platform" | "enterprise"
  tenantId: number
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
  permissionCatalog?: AdminPermission[]
  role?: IAMRole | null
}) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const [name, setName] = useState("")
  const [code, setCode] = useState("")
  const [description, setDescription] = useState("")
  const [preset, setPreset] = useState<Preset>("read_only")
  const [permissionSearch, setPermissionSearch] = useState("")
  const [selectedPermissions, setSelectedPermissions] = useState<string[]>([...permissionPresets.read_only])
  const [sortNo, setSortNo] = useState("100")
  const [status, setStatus] = useState("0")
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const editing = Boolean(role)

  useEffect(() => {
    if (!open) return
    if (role) {
      setName(role.name || "")
      setCode(role.code || "")
      setDescription(role.description || "")
      setSelectedPermissions(role.permissions ?? [])
      setSortNo(String(role.sortNo ?? 100))
      setStatus(String(role.status ?? 0))
    } else {
      setName("")
      setCode("")
      setDescription("")
      setPreset("read_only")
      setSelectedPermissions([...permissionPresets.read_only])
      setSortNo("100")
      setStatus("0")
    }
    setPermissionSearch("")
    setError("")
  }, [open, role])

  const visiblePermissions = useMemo(() => {
    const keyword = permissionSearch.trim().toLowerCase()
    return permissionCatalog.filter((permission) => {
      if (permission.status !== 0) return false
      if (!keyword) return true
      const displayName = getPermissionDisplayName(permission.code, permission.name, locale)
      const groupName = getPermissionGroupName(permission.groupName, locale)
      return [permission.name, displayName, permission.code, permission.groupName, groupName, permission.apiPath, permission.method]
        .filter(Boolean)
        .join(" ")
        .toLowerCase()
        .includes(keyword)
    })
  }, [locale, permissionCatalog, permissionSearch])

  function close(next: boolean) {
    if (saving) return
    onOpenChange(next)
    if (!next) {
      setName("")
      setCode("")
      setDescription("")
      setPreset("read_only")
      setPermissionSearch("")
      setSelectedPermissions([...permissionPresets.read_only])
      setSortNo("100")
      setStatus("0")
      setError("")
    }
  }

  function applyPreset(next: Preset) {
    setPreset(next)
    setSelectedPermissions([...permissionPresets[next]])
  }

  function togglePermission(permissionCode: string, checked: boolean) {
    setSelectedPermissions((current) => {
      if (checked) {
        return current.includes(permissionCode) ? current : [...current, permissionCode].sort()
      }
      return current.filter((code) => code !== permissionCode)
    })
  }

  async function submit() {
    const normalizedCode = role?.code ?? code.trim().toLowerCase().replace(/[^a-z0-9_]+/g, "_").replace(/^_+|_+$/g, "")
    if (!name.trim() || !normalizedCode) {
      setError(t("role.customNameAndCodeRequired"))
      return
    }
    setSaving(true)
    setError("")
    try {
      const currentRole = role
      const savedRole = await saveIAMRole({
        id: currentRole?.id,
        tenantId,
        domainType,
        name: name.trim(),
        code: normalizedCode,
        description: description.trim(),
        isBuiltin: currentRole?.isBuiltin ?? false,
        sortNo: Number(sortNo) || 100,
        status: Number(status),
      })
      await saveIAMRolePolicy(savedRole.id, selectedPermissions, tenantId, domainType)
      onSaved()
      close(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("role.customSaveFailed"))
    } finally {
      setSaving(false)
    }
  }

  return (
    <StandardModal
      open={open}
      onCancel={() => close(false)}
      title={<span className="flex items-center gap-2"><KeyRoundIcon className="size-5" />{editing ? t("role.customEditTitle") : t("role.customCreateTitle")}</span>}
      width={672}
      footer={
        <>
          <RailopsButton onClick={() => close(false)} disabled={saving}>{t("role.cancel")}</RailopsButton>
          <RailopsButton variant="primary" onClick={() => void submit()} disabled={saving}>{saving ? <Loader2Icon className="animate-spin" /> : <KeyRoundIcon />}{saving ? t("role.customSaving") : t("role.customSave")}</RailopsButton>
        </>
      }
    >
      <form id="iam-role-form" className="grid gap-4 sm:grid-cols-2" onSubmit={(event) => { event.preventDefault(); void submit() }}>
          <label className={iamRoleLabelClass}>{t("role.name")}<Input value={name} onChange={(event) => setName(event.target.value)} autoFocus placeholder={t("role.customNamePlaceholder")} /></label>
          <label className={iamRoleLabelClass}>{t("role.code")}<Input value={code} onChange={(event) => setCode(event.target.value)} autoComplete="off" disabled={editing} placeholder="regional_service_manager" /></label>
          <label className={`${iamRoleLabelClass} sm:col-span-2`}>{t("role.customResponsibility")}<Input value={description} onChange={(event) => setDescription(event.target.value)} placeholder={t("role.customResponsibilityPlaceholder")} /></label>
          <label className={iamRoleLabelClass}>
            {t("role.customSort")}
            <Input inputMode="numeric" value={sortNo} onChange={(event) => setSortNo(event.target.value.replace(/\D/g, ""))} />
          </label>
          <SelectField
            label={t("role.columnStatus")}
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("role.columnStatus"),
              value: status,
              onChange: (value) => setStatus((value as string) ?? "0"),
              options: [
                { value: "0", label: t("status.ok") },
                { value: "1", label: t("status.disabled") },
              ],
              style: { width: "100%" },
            }}
          />
          <div className="grid gap-1.5">
            <SelectField
              label={t("role.customPermissionPreset")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("role.customPermissionPreset"),
                value: preset,
                onChange: (value) => applyPreset((value as string) as Preset),
                options: [
                  { value: "read_only", label: t("role.customPresetReadOnly") },
                  { value: "service_operations", label: t("role.customPresetServiceOperations") },
                  { value: "iam_manager", label: t("role.customPresetIAMManager") },
                  { value: "finance_management", label: t("role.customPresetFinanceManagement") },
                ],
                style: { width: "100%" },
              }}
            />
            <span className={`text-xs font-normal ${iamRoleMutedTextClass}`}>{t("role.customSelectedPermissions", { count: selectedPermissions.length })}</span>
          </div>
          <label className={iamRoleLabelClass}>
            {t("role.customSearchPermission")}
            <SearchField allowClear value={permissionSearch} onChange={(event) => setPermissionSearch(event.target.value)} placeholder={t("role.customSearchPermissionPlaceholder")} />
          </label>
          <div className="max-h-64 overflow-y-auto rounded-md border border-border bg-card sm:col-span-2">
            {visiblePermissions.length ? visiblePermissions.map((permission) => {
              const displayName = getPermissionDisplayName(permission.code, permission.name, locale)
              const groupName = getPermissionGroupName(permission.groupName, locale)
              return (
                <label key={permission.code} className="flex cursor-pointer items-start gap-3 border-b border-border px-3 py-2.5 transition-colors last:border-b-0 hover:bg-muted/50">
                  <AntCheckbox
                    className="mt-1"
                    checked={selectedPermissions.includes(permission.code)}
                    onChange={(event) => togglePermission(permission.code, event.target.checked)}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="flex min-w-0 flex-wrap items-center gap-2">
                      <span className="min-w-0 text-sm font-medium text-foreground">{displayName}</span>
                      {groupName ? <StatusTag tone="neutral" className="max-w-full shrink-0 truncate">{groupName}</StatusTag> : null}
                    </span>
                    <span className={`mt-0.5 block truncate text-xs font-normal ${iamRoleMutedTextClass}`}>{permission.code} · {permission.method || "ANY"} {permission.apiPath || "-"}</span>
                  </span>
                </label>
              )
            }) : (
              <div className={`px-3 py-6 text-center text-sm ${iamRoleMutedTextClass}`}>{t("role.customNoPermissions")}</div>
            )}
          </div>
          {error ? <div className="rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive sm:col-span-2">{error}</div> : null}
      </form>
    </StandardModal>
  )
}
