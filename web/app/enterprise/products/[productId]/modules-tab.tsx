"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import Link from "next/link"
import {
  CheckIcon,
  Link2Icon,
  PencilIcon,
  PlusIcon,
  PowerIcon,
  ShieldCheckIcon,
  XIcon,
} from "lucide-react"
import { Checkbox as AntCheckbox, Pagination as AntPagination, Table as AntTable } from "antd"
import type { TableColumnsType } from "antd"
import { CheckboxField, ContentModule, SelectField, StandardModal, StatusTag, type StatusTagTone } from "@railops/ui"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { useAuth } from "@/components/auth-provider"
import { CanUseButton } from "@/components/layout/permission-guard"
import { useI18n } from "@/i18n/provider"
import {
  createProductModule,
  disableProductModule,
  enableProductModule,
  getProductModuleModels,
  getProductModules,
  listProductModels,
  replaceProductModuleModels,
  updateProductModule,
  type ProductModel,
  type ProductModule,
} from "@/lib/api/enterprise-products"
import {
  fetchEnterpriseIAMPartners,
  type EnterpriseIAMPartner,
} from "@/lib/api/platform-iam"
import { ErrorState, EmptyState, LoadingSkeleton } from "@/components/shared/error-states"

interface ModulesTabProps {
  productId: number
}

type I18nT = ReturnType<typeof useI18n>

type ModuleFormState = {
  module_code: string
  name: string
  default_supplier_id: string
  is_safety_critical: boolean
}

const emptyModuleForm: ModuleFormState = {
  module_code: "",
  name: "",
  default_supplier_id: "",
  is_safety_critical: false,
}

function formFromModule(module: ProductModule): ModuleFormState {
  return {
    module_code: module.module_code || "",
    name: module.name || "",
    default_supplier_id: String(module.default_supplier_id || ""),
    is_safety_critical: module.is_safety_critical,
  }
}

function statusLabel(t: I18nT, status: string) {
  switch (status) {
    case "active":
      return t("productCenter.modules.status.active")
    case "inactive":
      return t("productCenter.modules.status.inactive")
    default:
      return status || t("productCenter.modules.status.unknown")
  }
}

function moduleTypeLabel(t: I18nT, isSafetyCritical: boolean) {
  return isSafetyCritical
    ? t("productCenter.modules.type.safetyCritical")
    : t("productCenter.modules.type.normal")
}

function statusTone(status: string): StatusTagTone {
  switch (status) {
    case "active":
      return "success"
    case "inactive":
      return "disabled"
    default:
      return "neutral"
  }
}

function moduleErrorMessage(t: I18nT, error: unknown, fallback: string) {
  const message = error instanceof Error ? error.message : ""
  if (message.includes("module code already exists")) return t("productCenter.modules.codeExists")
  if (message.includes("module code is required")) return t("productCenter.modules.codeRequired")
  if (message.includes("module name is required")) return t("productCenter.modules.nameRequired")
  if (message.includes("default supplier must be an active supplier")) {
    return t("productCenter.modules.supplierRequired")
  }
  if (message.includes("product module not found")) return t("productCenter.modules.notFound")
  if (message.includes("product model not found")) return t("productCenter.modules.modelNotFound")
  if (message.includes("does not belong to the current tenant")) return t("productCenter.modules.productForbidden")
  return message || fallback
}

export function ModulesTab({ productId }: ModulesTabProps) {
  const t = useI18n()
  const { session } = useAuth()
  const canCreateModule = CanUseButton("productModel.create", session?.permissions)
  const canUpdateModule = CanUseButton("productModel.update", session?.permissions)
  const [modules, setModules] = useState<ProductModule[]>([])
  const [models, setModels] = useState<ProductModel[]>([])
  const [suppliers, setSuppliers] = useState<EnterpriseIAMPartner[]>([])
  const [loading, setLoading] = useState(true)
  const [modulesLoaded, setModulesLoaded] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const [form, setForm] = useState<ModuleFormState>(emptyModuleForm)
  const [createOpen, setCreateOpen] = useState(false)
  const [createError, setCreateError] = useState("")
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editingForm, setEditingForm] = useState<ModuleFormState>(emptyModuleForm)
  const [selectedModuleId, setSelectedModuleId] = useState<number | null>(null)
  const [selectedModelIds, setSelectedModelIds] = useState<number[]>([])
  const [scopeSaving, setScopeSaving] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  const load = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const [moduleResponse, modelResponse, partnerResponse] = await Promise.all([
        getProductModules(productId),
        listProductModels(productId),
        fetchEnterpriseIAMPartners({ page: 1, limit: 200 }).catch(() => null),
      ])
      if (!moduleResponse.success || !moduleResponse.data) {
        throw new Error(moduleResponse.error?.message || t("productCenter.modules.loadFailed"))
      }
      if (!modelResponse.success || !modelResponse.data) {
        throw new Error(modelResponse.error?.message || t("productCenter.modules.modelLoadFailed"))
      }
      setModules(moduleResponse.data)
      setModels(modelResponse.data)
      setSuppliers(
        partnerResponse?.results.filter((partner) => partner.status === 0) ?? [],
      )
      setModulesLoaded(true)
    } catch (err) {
      setError(moduleErrorMessage(t, err, t("productCenter.modules.loadFailed")))
    } finally {
      setLoading(false)
    }
  }, [productId, t])

  useEffect(() => {
    void load()
  }, [load])

  const selectedModule = useMemo(
    () => modules.find((item) => item.id === selectedModuleId) ?? null,
    [modules, selectedModuleId]
  )

  useEffect(() => {
    setPage(1)
  }, [modules.length])

  const pagedModules = useMemo(
    () => modules.slice((page - 1) * pageSize, page * pageSize),
    [modules, page, pageSize],
  )

  const supplierNames = useMemo(
    () => new Map(suppliers.map((supplier) => [supplier.id, supplier.name])),
    [suppliers],
  )

  async function handleCreate() {
    const moduleCode = form.module_code.trim()
    const moduleName = form.name.trim()
    if (!moduleCode || !moduleName) {
      const message = !moduleCode ? t("productCenter.modules.codeRequired") : t("productCenter.modules.nameRequired")
      setCreateError(message)
      return
    }
    setSaving(true)
    setError("")
    setCreateError("")
    try {
      const response = await createProductModule(productId, {
        module_code: moduleCode,
        name: moduleName,
        default_supplier_id: Number(form.default_supplier_id) || 0,
        is_safety_critical: form.is_safety_critical,
      })
      if (!response.success) {
        throw new Error(response.error?.message || t("productCenter.modules.createFailed"))
      }
      setForm(emptyModuleForm)
      await load()
      setCreateOpen(false)
      toast.success(t("productCenter.modules.createdSuccess"))
    } catch (err) {
      const message = moduleErrorMessage(t, err, t("productCenter.modules.createFailed"))
      setCreateError(message)
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  async function handleUpdate(moduleId: number) {
    setSaving(true)
    setError("")
    try {
      const response = await updateProductModule(productId, moduleId, {
        module_code: editingForm.module_code,
        name: editingForm.name,
        default_supplier_id: Number(editingForm.default_supplier_id) || 0,
        is_safety_critical: editingForm.is_safety_critical,
      })
      if (!response.success) {
        throw new Error(response.error?.message || t("productCenter.modules.updateFailed"))
      }
      setEditingId(null)
      await load()
      toast.success(t("productCenter.modules.updatedSuccess"))
    } catch (err) {
      const message = moduleErrorMessage(t, err, t("productCenter.modules.updateFailed"))
      setError(message)
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  async function handleToggleStatus(module: ProductModule) {
    setSaving(true)
    setError("")
    try {
      const response =
      module.status === "active"
          ? await disableProductModule(productId, module.id)
          : await enableProductModule(productId, module.id)
      if (!response.success) {
        throw new Error(response.error?.message || t("productCenter.modules.statusUpdateFailed"))
      }
      await load()
      toast.success(
        module.status === "active"
          ? t("productCenter.modules.disabledSuccess")
          : t("productCenter.modules.enabledSuccess"),
      )
    } catch (err) {
      const message = moduleErrorMessage(t, err, t("productCenter.modules.statusUpdateFailed"))
      setError(message)
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  async function handleEditScope(module: ProductModule) {
    setScopeSaving(true)
    setError("")
    try {
      const response = await getProductModuleModels(productId, module.id)
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || t("productCenter.modules.scopeLoadFailed"))
      }
      setSelectedModuleId(module.id)
      setSelectedModelIds(response.data.model_ids || [])
    } catch (err) {
      const message = moduleErrorMessage(t, err, t("productCenter.modules.scopeLoadFailed"))
      setError(message)
      toast.error(message)
    } finally {
      setScopeSaving(false)
    }
  }

  async function handleSaveScope() {
    if (!selectedModuleId) return
    setScopeSaving(true)
    setError("")
    try {
      const response = await replaceProductModuleModels(productId, selectedModuleId, selectedModelIds)
      if (!response.success) {
        throw new Error(response.error?.message || t("productCenter.modules.scopeSaveFailed"))
      }
      await load()
      setSelectedModuleId(null)
      setSelectedModelIds([])
      toast.success(t("productCenter.modules.scopeSavedSuccess"))
    } catch (err) {
      const message = moduleErrorMessage(t, err, t("productCenter.modules.scopeSaveFailed"))
      setError(message)
      toast.error(message)
    } finally {
      setScopeSaving(false)
    }
  }

  function toggleModelSelection(modelId: number) {
    setSelectedModelIds((current) =>
      current.includes(modelId)
        ? current.filter((item) => item !== modelId)
        : [...current, modelId]
    )
  }

  function startEdit(module: ProductModule) {
    setEditingId(module.id)
    setEditingForm(formFromModule(module))
  }

  function handleCreateOpenChange(open: boolean) {
    if (saving) return
    setCreateOpen(open)
    setCreateError("")
    if (!open) setForm(emptyModuleForm)
  }

  function openCreateDialog() {
    setError("")
    setCreateError("")
    setForm(emptyModuleForm)
    setCreateOpen(true)
  }

  const initialLoading = loading && !modulesLoaded
  const moduleColumns: TableColumnsType<ProductModule> = [
    {
      title: t("productCenter.modules.columns.name"),
      dataIndex: "name",
      key: "name",
      width: 190,
      render: (_, module) => {
        const isEditing = editingId === module.id
        return isEditing ? (
          <Input
            aria-label={t("productCenter.modules.editName")}
            value={editingForm.name}
            onChange={(event) =>
              setEditingForm((current) => ({ ...current, name: event.target.value }))
            }
          />
        ) : (
          <strong className="rhd-railops-primary-text">{module.name}</strong>
        )
      },
    },
    {
      title: t("productCenter.modules.columns.code"),
      dataIndex: "module_code",
      key: "module_code",
      width: 150,
      render: (_, module) => {
        const isEditing = editingId === module.id
        return isEditing ? (
          <Input
            aria-label={t("productCenter.modules.editCode")}
            value={editingForm.module_code}
            onChange={(event) =>
              setEditingForm((current) => ({
                ...current,
                module_code: event.target.value,
              }))
            }
          />
        ) : (
          <span className="rhd-railops-code-text">{module.module_code}</span>
        )
      },
    },
    {
      title: t("productCenter.modules.columns.type"),
      dataIndex: "is_safety_critical",
      key: "is_safety_critical",
      width: 150,
      render: (_, module) => {
        const isEditing = editingId === module.id
        return isEditing ? (
          <Button
            type="button"
            size="sm"
            variant={editingForm.is_safety_critical ? "default" : "outline"}
            onClick={() =>
              setEditingForm((current) => ({
                ...current,
                is_safety_critical: !current.is_safety_critical,
              }))
            }
          >
            {moduleTypeLabel(t, editingForm.is_safety_critical)}
          </Button>
        ) : (
          <StatusTag tone={module.is_safety_critical ? "warning" : "neutral"}>
            {moduleTypeLabel(t, module.is_safety_critical)}
          </StatusTag>
        )
      },
    },
    {
      title: t("productCenter.modules.columns.defaultSupplier"),
      dataIndex: "default_supplier_id",
      key: "default_supplier_id",
      width: 190,
      render: (_, module) => {
        const isEditing = editingId === module.id
        return isEditing ? (
          <SelectField
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("productCenter.modules.editDefaultSupplier"),
              placeholder: t("productCenter.modules.supplierEmpty"),
              value: editingForm.default_supplier_id || undefined,
              onChange: (value) =>
                setEditingForm((current) => ({
                  ...current,
                  default_supplier_id: (value as string) ?? "",
                })),
              options: suppliers.map((supplier) => ({ value: String(supplier.id), label: supplier.name })),
              style: { width: "100%" },
            }}
          />
        ) : (
          supplierNames.get(module.default_supplier_id) || t("productCenter.modules.notBound")
        )
      },
    },
    {
      title: t("productCenter.modules.columns.models"),
      dataIndex: "model_names",
      key: "model_names",
      width: 240,
      render: (_, module) => (
        <span className="rhd-railops-muted-line" title={module.model_names.join(" / ")}>
          {module.model_names.length > 0 ? module.model_names.join(" / ") : t("productCenter.modules.notConfigured")}
        </span>
      ),
    },
    {
      title: t("productCenter.modules.columns.status"),
      dataIndex: "status",
      key: "status",
      width: 120,
      render: (_, module) => (
        <StatusTag tone={statusTone(module.status)}>
          {statusLabel(t, module.status)}
        </StatusTag>
      ),
    },
    {
      title: t("productCenter.modules.columns.actions"),
      key: "actions",
      fixed: "right",
      width: 150,
      align: "right",
      render: (_, module) => {
        const isEditing = editingId === module.id
        return !canUpdateModule ? null : isEditing ? (
          <div className="rhd-railops-row-actions">
            <Button
              type="button"
              size="icon"
              variant="ghost"
              onClick={() => handleUpdate(module.id)}
              disabled={saving || !editingForm.module_code.trim() || !editingForm.name.trim()}
            >
              <CheckIcon className="size-4" />
            </Button>
            <Button
              type="button"
              size="icon"
              variant="ghost"
              onClick={() => setEditingId(null)}
              disabled={saving}
            >
              <XIcon className="size-4" />
            </Button>
          </div>
        ) : (
          <div className="rhd-railops-row-actions">
            <Button type="button" size="icon" variant="ghost" title={t("productCenter.modules.editAction")} onClick={() => startEdit(module)}>
              <PencilIcon className="size-4" />
            </Button>
            <Button
              type="button"
              size="icon"
              variant="ghost"
              onClick={() => handleEditScope(module)}
              disabled={scopeSaving}
              title={t("productCenter.modules.manageScope")}
            >
              <Link2Icon className="size-4" />
            </Button>
            <Button
              type="button"
              size="icon"
              variant="ghost"
              onClick={() => handleToggleStatus(module)}
              disabled={saving}
              title={module.status === "active" ? t("productCenter.disable") : t("productCenter.enable")}
            >
              <PowerIcon className="size-4" />
            </Button>
          </div>
        )
      },
    },
  ]

  return (
    <div className="rhd-railops-product-tab">
      {error ? (
        <div className="rhd-railops-product-tab-error">
          <ErrorState
            title={t("productCenter.modules.errorTitle")}
            description={error}
            action={{ label: t("productCenter.retry"), onClick: load }}
          />
        </div>
      ) : null}

      <div className="rhd-railops-product-tab-toolbar">
        <StatusTag tone="neutral">{t("productCenter.modules.tableTitle", { count: modules.length })}</StatusTag>
        {canCreateModule ? (
          <Button type="button" onClick={openCreateDialog}>
            <PlusIcon className="size-4" />
            {t("productCenter.modules.createTitle")}
          </Button>
        ) : null}
      </div>
      <StandardModal
        open={canCreateModule && createOpen}
        onCancel={() => handleCreateOpenChange(false)}
        title={t("productCenter.modules.createTitle")}
        width={512}
        footer={
          <>
            <Button type="button" variant="outline" onClick={() => handleCreateOpenChange(false)} disabled={saving}>
              {t("productCenter.cancel")}
            </Button>
            <Button
              type="submit"
              form="create-product-module-form"
              disabled={saving || !form.module_code.trim() || !form.name.trim()}
            >
              {saving ? t("productCenter.modules.creating") : t("productCenter.modules.confirmCreate")}
            </Button>
          
          </>
        }
      >
        <form
          id="create-product-module-form"
          className="grid gap-4 py-1 sm:grid-cols-2"
          onSubmit={(event) => {
            event.preventDefault()
            void handleCreate()
          }}
        >
          <label className="grid gap-2 text-sm font-medium">
            <span>{t("productCenter.modules.code")}</span>
            <Input
              autoFocus
              required
              aria-label={t("productCenter.modules.code")}
              placeholder={t("productCenter.modules.moduleCodePlaceholder")}
              value={form.module_code}
              onChange={(event) => {
                setCreateError("")
                setForm((current) => ({ ...current, module_code: event.target.value }))
              }}
            />
          </label>
          <label className="grid gap-2 text-sm font-medium">
            <span>{t("productCenter.modules.name")}</span>
            <Input
              required
              aria-label={t("productCenter.modules.name")}
              placeholder={t("productCenter.modules.moduleNamePlaceholder")}
              value={form.name}
              onChange={(event) => {
                setCreateError("")
                setForm((current) => ({ ...current, name: event.target.value }))
              }}
            />
          </label>
          <SelectField
            label={t("productCenter.modules.defaultSupplier")}
            className="sm:col-span-2"
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("productCenter.modules.defaultSupplier"),
              placeholder: t("productCenter.modules.supplierEmpty"),
              value: form.default_supplier_id || undefined,
              onChange: (value) =>
                setForm((current) => ({ ...current, default_supplier_id: (value as string) ?? "" })),
              options: suppliers.map((supplier) => ({ value: String(supplier.id), label: supplier.name })),
              style: { width: "100%" },
            }}
          />
          <CheckboxField
            className="sm:col-span-2 rounded-md border border-input p-3"
            style={{ marginBottom: 0 }}
            checkboxProps={{
              checked: form.is_safety_critical,
              onChange: (event) =>
                setForm((current) => ({ ...current, is_safety_critical: event.target.checked })),
            }}
          >
            <ShieldCheckIcon className="size-4 text-muted-foreground" />
            {t("productCenter.modules.safetyCritical")}
          </CheckboxField>
        </form>
        {suppliers.length === 0 ? (
          <Button className="w-fit px-0" variant="link" size="sm" render={<Link href="/enterprise/partner" />}>
            {t("productCenter.modules.supplierSetupLink")}
          </Button>
        ) : null}
        {createError ? (
          <p role="alert" className="text-sm text-destructive">{createError}</p>
        ) : null}
        
      </StandardModal>

      {initialLoading ? (
        <LoadingSkeleton count={3} layout="table" />
      ) : modules.length === 0 ? (
        <EmptyState
          title={t("productCenter.modules.emptyTitle")}
          className="rhd-railops-product-tab-empty"
        />
      ) : (
        <ContentModule className="rhd-railops-product-table-module">
          <AntTable<ProductModule>
            className="rhd-railops-product-table"
            columns={moduleColumns}
            dataSource={pagedModules}
            pagination={false}
            rowKey="id"
            scroll={{ x: 1120 }}
            size="middle"
          />
          <div className="rhd-railops-table-footer">
            <AntPagination
              current={page}
              pageSize={pageSize}
              total={modules.length}
              showSizeChanger
              size="small"
              onChange={(nextPage, nextPageSize) => {
                if (nextPageSize !== pageSize) {
                  setPageSize(nextPageSize)
                  setPage(1)
                  return
                }
                setPage(nextPage)
              }}
            />
          </div>
        </ContentModule>
      )}

      {canUpdateModule && selectedModule ? (
        <section className="rhd-railops-product-scope-panel">
          <header>
            <strong>{t("productCenter.modules.selectedScopeTitle", { name: selectedModule.name })}</strong>
          </header>
          <div className="rhd-railops-product-scope-body">
            {models.length === 0 ? (
              <EmptyState
                title={t("productCenter.modules.scopeEmptyTitle")}
              />
            ) : (
              <div className="rhd-railops-product-scope-grid">
                {models.map((model) => {
                  const checked = selectedModelIds.includes(model.id)
                  return (
                    <label
                      key={model.id}
                      className={checked ? "is-selected" : ""}
                    >
                      <AntCheckbox
                        checked={checked}
                        onChange={() => toggleModelSelection(model.id)}
                      />
                      <div>
                        <div className="font-medium">{model.name}</div>
                        <div className="text-xs text-muted-foreground">{model.model_code}</div>
                      </div>
                    </label>
                  )
                })}
              </div>
            )}
            <div className="rhd-railops-product-scope-actions">
              <Button type="button" onClick={handleSaveScope} disabled={scopeSaving}>
                {scopeSaving ? t("productCenter.modules.savingScope") : t("productCenter.modules.saveScope")}
              </Button>
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setSelectedModuleId(null)
                  setSelectedModelIds([])
                }}
                disabled={scopeSaving}
              >
                {t("productCenter.cancel")}
              </Button>
            </div>
          </div>
        </section>
      ) : null}
    </div>
  )
}
