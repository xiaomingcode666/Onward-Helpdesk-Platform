"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import { CheckIcon, PencilIcon, PlusIcon, PowerIcon, XIcon } from "lucide-react"
import { Pagination as AntPagination, Table as AntTable } from "antd"
import type { TableColumnsType } from "antd"
import { ContentModule, StandardModal, StatusTag, type StatusTagTone } from "@railops/ui"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { useAuth } from "@/components/auth-provider"
import { CanUseButton } from "@/components/layout/permission-guard"
import { ErrorState, EmptyState, LoadingSkeleton } from "@/components/shared/error-states"
import { useI18n } from "@/i18n/provider"
import {
  createProductModel,
  disableProductModel,
  enableProductModel,
  listProductModels,
  updateProductModel,
  type ProductModel,
} from "@/lib/api/enterprise-products"

interface ModelsTabProps {
  productId: number
}

type I18nT = ReturnType<typeof useI18n>

type ModelFormState = {
  model_code: string
  name: string
  version_policy: string
  region_scope_json: string
}

const emptyModelForm: ModelFormState = {
  model_code: "",
  name: "",
  version_policy: "",
  region_scope_json: "",
}

function formFromModel(model: ProductModel): ModelFormState {
  return {
    model_code: model.model_code || "",
    name: model.name || "",
    version_policy: model.version_policy || "",
    region_scope_json: formatRegionScope(model.region_scope_json),
  }
}

function statusLabel(t: I18nT, status: string) {
  switch (status) {
    case "active":
      return t("productCenter.model.active")
    case "inactive":
      return t("productCenter.model.inactive")
    case "deleted":
      return t("productCenter.model.deletedState")
    default:
      return status || t("productCenter.model.unknown")
  }
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

function formatRegionScope(value: string) {
  if (!value.trim()) return ""
  try {
    const parsed = JSON.parse(value)
    if (Array.isArray(parsed)) {
      return parsed.map(String).filter(Boolean).join("、")
    }
  } catch {
    // Keep legacy non-JSON values editable instead of blocking the form.
  }
  return value
}

function serializeRegionScope(value: string) {
  const trimmed = value.trim()
  if (!trimmed) return "[]"
  try {
    const parsed = JSON.parse(trimmed)
    if (Array.isArray(parsed)) {
      return JSON.stringify(parsed.map(String).map((item) => item.trim()).filter(Boolean))
    }
  } catch {
    // Plain text is the primary input format.
  }
  return JSON.stringify(
    trimmed
      .split(/[，,、]/)
      .map((item) => item.trim())
      .filter(Boolean)
  )
}

function modelErrorMessage(t: I18nT, error: unknown, fallback: string) {
  const message = error instanceof Error ? error.message : ""
  if (message.includes("model code already exists")) return t("productCenter.model.codeExists")
  if (message.includes("modelCode is required")) return t("productCenter.model.codeRequired")
  if (message.includes("model name is required")) return t("productCenter.model.nameRequired")
  if (message.includes("product model not found")) return t("productCenter.model.notFound")
  if (message.includes("product not found")) return t("productCenter.model.productMissing")
  if (message.includes("does not belong to tenant")) return t("productCenter.model.productForbidden")
  return message || fallback
}

export function ModelsTab({ productId }: ModelsTabProps) {
  const t = useI18n()
  const { session } = useAuth()
  const canCreateModel = CanUseButton("productModel.create", session?.permissions)
  const canUpdateModel = CanUseButton("productModel.update", session?.permissions)
  const [models, setModels] = useState<ProductModel[]>([])
  const [loading, setLoading] = useState(true)
  const [modelsLoaded, setModelsLoaded] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const [form, setForm] = useState<ModelFormState>(emptyModelForm)
  const [createOpen, setCreateOpen] = useState(false)
  const [createError, setCreateError] = useState("")
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editingForm, setEditingForm] = useState<ModelFormState>(emptyModelForm)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  const load = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const response = await listProductModels(productId)
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || t("productCenter.model.loadFailed"))
      }
      setModels(response.data)
      setModelsLoaded(true)
    } catch (err) {
      setError(modelErrorMessage(t, err, t("productCenter.model.loadFailed")))
    } finally {
      setLoading(false)
    }
  }, [productId, t])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    setPage(1)
  }, [models.length])

  const pagedModels = useMemo(
    () => models.slice((page - 1) * pageSize, page * pageSize),
    [models, page, pageSize],
  )

  async function handleCreate() {
    const modelCode = form.model_code.trim()
    const modelName = form.name.trim()
    if (!modelCode || !modelName) {
      const message = !modelCode ? t("productCenter.model.codeRequired") : t("productCenter.model.nameRequired")
      setCreateError(message)
      return
    }
    setSaving(true)
    setError("")
    setCreateError("")
    try {
      const response = await createProductModel(productId, {
        ...form,
        model_code: modelCode,
        name: modelName,
        version_policy: form.version_policy.trim(),
        region_scope_json: serializeRegionScope(form.region_scope_json),
      })
      if (!response.success) {
        throw new Error(response.error?.message || t("productCenter.model.createFailed"))
      }
      setForm(emptyModelForm)
      await load()
      setCreateOpen(false)
      toast.success(t("productCenter.model.createdSuccess"))
    } catch (err) {
      const message = modelErrorMessage(t, err, t("productCenter.model.createFailed"))
      setCreateError(message)
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  async function handleUpdate(modelId: number) {
    setSaving(true)
    setError("")
    try {
      const response = await updateProductModel(productId, modelId, {
        ...editingForm,
        model_code: editingForm.model_code.trim(),
        name: editingForm.name.trim(),
        version_policy: editingForm.version_policy.trim(),
        region_scope_json: serializeRegionScope(editingForm.region_scope_json),
      })
      if (!response.success) {
        throw new Error(response.error?.message || t("productCenter.model.updateFailed"))
      }
      setEditingId(null)
      await load()
      toast.success(t("productCenter.model.updatedSuccess"))
    } catch (err) {
      const message = modelErrorMessage(t, err, t("productCenter.model.updateFailed"))
      setError(message)
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  async function handleToggleStatus(model: ProductModel) {
    setSaving(true)
    setError("")
    try {
      const response =
        model.status === "active"
          ? await disableProductModel(productId, model.id)
          : await enableProductModel(productId, model.id)
      if (!response.success) {
        throw new Error(response.error?.message || t("productCenter.model.statusUpdateFailed"))
      }
      await load()
      toast.success(model.status === "active" ? t("productCenter.model.disabledSuccess") : t("productCenter.model.enabledSuccess"))
    } catch (err) {
      const message = modelErrorMessage(t, err, t("productCenter.model.statusUpdateFailed"))
      setError(message)
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  function startEdit(model: ProductModel) {
    setEditingId(model.id)
    setEditingForm(formFromModel(model))
  }

  function handleCreateOpenChange(open: boolean) {
    if (saving) return
    setCreateOpen(open)
    setCreateError("")
    if (!open) setForm(emptyModelForm)
  }

  function openCreateDialog() {
    setError("")
    setCreateError("")
    setForm(emptyModelForm)
    setCreateOpen(true)
  }

  const initialLoading = loading && !modelsLoaded
  const modelColumns: TableColumnsType<ProductModel> = [
    {
      title: t("productCenter.model.modelCode"),
      dataIndex: "model_code",
      key: "model_code",
      width: 150,
      render: (_, model) => {
        const isEditing = editingId === model.id
        return isEditing ? (
          <Input
            aria-label={t("productCenter.model.editModelCode")}
            value={editingForm.model_code}
            onChange={(event) =>
              setEditingForm((current) => ({
                ...current,
                model_code: event.target.value,
              }))
            }
          />
        ) : (
          <span className="rhd-railops-code-text">{model.model_code}</span>
        )
      },
    },
    {
      title: t("productCenter.model.name"),
      dataIndex: "name",
      key: "name",
      width: 180,
      render: (_, model) => {
        const isEditing = editingId === model.id
        return isEditing ? (
          <Input
            aria-label={t("productCenter.model.editName")}
            value={editingForm.name}
            onChange={(event) =>
              setEditingForm((current) => ({ ...current, name: event.target.value }))
            }
          />
        ) : (
          <strong className="rhd-railops-primary-text">{model.name}</strong>
        )
      },
    },
    {
      title: t("productCenter.model.versionPolicy"),
      dataIndex: "version_policy",
      key: "version_policy",
      width: 170,
      render: (_, model) => {
        const isEditing = editingId === model.id
        return isEditing ? (
          <Input
            aria-label={t("productCenter.model.editVersionPolicy")}
            value={editingForm.version_policy}
            onChange={(event) =>
              setEditingForm((current) => ({
                ...current,
                version_policy: event.target.value,
              }))
            }
          />
        ) : (
          model.version_policy || "-"
        )
      },
    },
    {
      title: t("productCenter.model.regionScope"),
      dataIndex: "region_scope_json",
      key: "region_scope_json",
      width: 220,
      render: (_, model) => {
        const isEditing = editingId === model.id
        return isEditing ? (
          <Input
            aria-label={t("productCenter.model.editRegionScope")}
            value={editingForm.region_scope_json}
            onChange={(event) =>
              setEditingForm((current) => ({
                ...current,
                region_scope_json: event.target.value,
              }))
            }
          />
        ) : (
          <span className="rhd-railops-muted-line" title={formatRegionScope(model.region_scope_json)}>
            {formatRegionScope(model.region_scope_json) || t("productCenter.model.noLimit")}
          </span>
        )
      },
    },
    {
      title: t("productCenter.model.references"),
      key: "references",
      width: 180,
      render: (_, model) => (
        <span className="rhd-railops-muted-line">
          {t("productCenter.model.referencesSummary", { deviceCount: model.device_count, ticketCount: model.ticket_count })}
        </span>
      ),
    },
    {
      title: t("productCenter.model.status"),
      dataIndex: "status",
      key: "status",
      width: 120,
      render: (_, model) => (
        <StatusTag tone={statusTone(model.status)}>
          {statusLabel(t, model.status)}
        </StatusTag>
      ),
    },
    {
      title: t("productCenter.actions"),
      key: "actions",
      fixed: "right",
      width: 130,
      align: "right",
      render: (_, model) => {
        const isEditing = editingId === model.id
        return !canUpdateModel ? null : isEditing ? (
          <div className="rhd-railops-row-actions">
            <Button
              type="button"
              size="icon"
              variant="ghost"
              onClick={() => handleUpdate(model.id)}
              disabled={saving || !editingForm.model_code.trim() || !editingForm.name.trim()}
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
            <Button type="button" size="icon" variant="ghost" title={t("productCenter.edit")} onClick={() => startEdit(model)}>
              <PencilIcon className="size-4" />
            </Button>
            <Button
              type="button"
              size="icon"
              variant="ghost"
              onClick={() => handleToggleStatus(model)}
              disabled={saving}
              title={model.status === "active" ? t("productCenter.disable") : t("productCenter.enable")}
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
            title={t("productCenter.model.loadFailed")}
            description={error}
            action={{ label: t("productCenter.retry"), onClick: load }}
          />
        </div>
      ) : null}

      <div className="rhd-railops-product-tab-toolbar">
        <StatusTag tone="neutral">{t("productCenter.model.tableTitle", { count: models.length })}</StatusTag>
        {canCreateModel ? (
          <Button type="button" onClick={openCreateDialog}>
            <PlusIcon className="size-4" />
            {t("productCenter.model.new")}
          </Button>
        ) : null}
      </div>
      <StandardModal
        open={canCreateModel && createOpen}
        onCancel={() => handleCreateOpenChange(false)}
        title={t("productCenter.model.createTitle")}
        width={512}
        footer={
          <>
            <Button type="button" variant="outline" onClick={() => handleCreateOpenChange(false)} disabled={saving}>
              {t("productCenter.cancel")}
            </Button>
            <Button
              type="submit"
              form="create-product-model-form"
              disabled={saving || !form.model_code.trim() || !form.name.trim()}
            >
              {saving ? t("productCenter.saving") : t("productCenter.model.confirmCreate")}
            </Button>
          
          </>
        }
      >
        <form
          id="create-product-model-form"
          className="grid gap-4 py-1 sm:grid-cols-2"
          onSubmit={(event) => {
            event.preventDefault()
            void handleCreate()
          }}
        >
          <label className="grid gap-2 text-sm font-medium">
            <span>{t("productCenter.model.modelCode")}</span>
            <Input
              autoFocus
              required
              aria-label={t("productCenter.model.modelCode")}
              placeholder={t("productCenter.model.modelCodePlaceholder")}
              value={form.model_code}
              onChange={(event) => {
                setCreateError("")
                setForm((current) => ({ ...current, model_code: event.target.value }))
              }}
            />
          </label>
          <label className="grid gap-2 text-sm font-medium">
            <span>{t("productCenter.model.name")}</span>
            <Input
              required
              aria-label={t("productCenter.model.name")}
              placeholder={t("productCenter.model.namePlaceholder")}
              value={form.name}
              onChange={(event) => {
                setCreateError("")
                setForm((current) => ({ ...current, name: event.target.value }))
              }}
            />
          </label>
          <label className="grid gap-2 text-sm font-medium">
            <span>{t("productCenter.model.versionPolicy")}</span>
            <Input
              aria-label={t("productCenter.model.versionPolicy")}
              placeholder={t("productCenter.model.versionPolicyPlaceholder")}
              value={form.version_policy}
              onChange={(event) => setForm((current) => ({ ...current, version_policy: event.target.value }))}
            />
          </label>
          <label className="grid gap-2 text-sm font-medium">
            <span>{t("productCenter.model.regionScope")}</span>
            <Input
              aria-label={t("productCenter.model.regionScope")}
              placeholder={t("productCenter.model.regionScopePlaceholder")}
              value={form.region_scope_json}
              onChange={(event) => setForm((current) => ({ ...current, region_scope_json: event.target.value }))}
            />
          </label>
        </form>
        {createError ? (
          <p role="alert" className="text-sm text-destructive">{createError}</p>
        ) : null}
        
      </StandardModal>

      {initialLoading ? (
        <LoadingSkeleton count={3} layout="table" />
      ) : models.length === 0 ? (
        <EmptyState
          title={t("productCenter.model.emptyTitle")}
          className="rhd-railops-product-tab-empty"
        />
      ) : (
        <ContentModule className="rhd-railops-product-table-module">
          <AntTable<ProductModel>
            className="rhd-railops-product-table"
            columns={modelColumns}
            dataSource={pagedModels}
            pagination={false}
            rowKey="id"
            scroll={{ x: 980 }}
            size="middle"
          />
          <div className="rhd-railops-table-footer">
            <AntPagination
              current={page}
              pageSize={pageSize}
              total={models.length}
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
    </div>
  )
}
