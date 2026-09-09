"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import {
  DownloadIcon,
  EyeIcon,
  FileTextIcon,
  Loader2Icon,
  PencilIcon,
  Trash2Icon,
  UploadIcon,
} from "lucide-react"
import type { TableColumnsType } from "antd"
import { DataTable, IconButton, RailopsButton, SelectField, StandardModal, StatusTag, type StatusTagTone } from "@railops/ui"

import { useAuth } from "@/components/auth-provider"
import { CanUseButton } from "@/components/layout/permission-guard"
import { ErrorState, EmptyState, LoadingSkeleton } from "@/components/shared/error-states"
import { useI18n } from "@/i18n/provider"
import {
  deleteProductManualFile,
  getProductManualFileContent,
  getProductManualFiles,
  updateProductManualFile,
  uploadProductManualFile,
  type ProductManualFile,
  type ProductManualFileMutation,
} from "@/lib/api/enterprise-products"

interface ManualsTabProps {
  productId: number
}

type I18nT = ReturnType<typeof useI18n>

function formatFileSize(bytes: number) {
  if (!bytes || bytes < 0) return "-"
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`
}

function getFileExtension(filename: string) {
  const extension = filename.split(".").pop()?.trim().toLowerCase()
  return extension && extension !== filename.toLowerCase() ? extension : ""
}

function formatFileType(item: ProductManualFile) {
  const extension = getFileExtension(item.filename)
  if (extension) return extension === "markdown" ? "MD" : extension.toUpperCase()

  const mimeSubtype = item.mime_type.split("/").pop()?.split(";")[0]?.trim()
  return mimeSubtype ? mimeSubtype.toUpperCase() : item.provider || "-"
}

function supportsInlinePreview(item: ProductManualFile) {
  const extension = getFileExtension(item.filename)
  return (
    item.mime_type === "application/pdf" ||
    item.mime_type.startsWith("text/") ||
    ["pdf", "txt", "md", "markdown", "html", "htm"].includes(extension)
  )
}

function supportsDocxPreview(item: ProductManualFile) {
  return (
    getFileExtension(item.filename) === "docx" ||
    item.mime_type === "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
  )
}

function manualVisibilityLabel(t: I18nT, visibility: string) {
  return visibility === "internal"
    ? t("productCenter.manuals.visibilityInternal")
    : t("productCenter.manuals.visibilityPublic")
}

function manualSyncLabel(t: I18nT, item: ProductManualFile) {
  if (item.rag_sync_status === "indexed") return t("productCenter.manuals.syncIndexed")
  if (item.rag_sync_status === "failed") return t("productCenter.manuals.syncFailed")
  return item.knowledge_base_id > 0
    ? t("productCenter.manuals.syncWaitingIndex")
    : t("productCenter.manuals.syncWaitingKnowledgeBase")
}

function manualVisibilityTone(visibility: string): StatusTagTone {
  return visibility === "internal" ? "warning" : "success"
}

function manualSyncTone(item: ProductManualFile): StatusTagTone {
  if (item.rag_sync_status === "indexed") return "success"
  if (item.rag_sync_status === "failed") return "error"
  if (item.knowledge_base_id > 0) return "warning"
  return "disabled"
}

function manualErrorMessage(t: I18nT, error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback || t("productCenter.manuals.loadFailed")
}

export function ManualsTab({ productId }: ManualsTabProps) {
  const t = useI18n()
  const { session } = useAuth()
  const inputRef = useRef<HTMLInputElement | null>(null)
  const docxPreviewRef = useRef<HTMLDivElement | null>(null)
  const [items, setItems] = useState<ProductManualFile[]>([])
  const [loading, setLoading] = useState(true)
  const [itemsLoaded, setItemsLoaded] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [error, setError] = useState("")
  const [previewItem, setPreviewItem] = useState<ProductManualFile | null>(null)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [previewError, setPreviewError] = useState("")
  const [editingItem, setEditingItem] = useState<ProductManualFile | null>(null)
  const [editForm, setEditForm] = useState<ProductManualFileMutation>({
    title: "",
    language: "default",
    version: "v1.0",
    visibility: "public",
  })
  const [savingEdit, setSavingEdit] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const canManageManuals = CanUseButton("product.update", session?.permissions)

  const load = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const response = await getProductManualFiles(productId)
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || t("productCenter.manuals.loadFailed"))
      }
      setItems(response.data)
      setItemsLoaded(true)
    } catch (err) {
      setError(manualErrorMessage(t, err, t("productCenter.manuals.loadFailed")))
    } finally {
      setLoading(false)
    }
  }, [productId, t])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    setPage(1)
  }, [items.length])

  const pagedItems = useMemo(
    () => items.slice((page - 1) * pageSize, page * pageSize),
    [items, page, pageSize],
  )

  useEffect(() => {
    if (!previewItem || !supportsDocxPreview(previewItem) || !docxPreviewRef.current) {
      setPreviewLoading(false)
      setPreviewError("")
      return
    }

    const controller = new AbortController()
    const item = previewItem
    const container = docxPreviewRef.current
    container.replaceChildren()
    setPreviewLoading(true)
    setPreviewError("")

    async function renderDocxPreview() {
      try {
        const { blob: fileBlob } = await getProductManualFileContent(productId, item.id, controller.signal)
        const { renderAsync } = await import("docx-preview")
        if (controller.signal.aborted || !container.isConnected) return

        await renderAsync(fileBlob, container, container, {
          breakPages: true,
          ignoreLastRenderedPageBreak: false,
          renderHeaders: true,
          renderFooters: true,
          renderFootnotes: true,
          renderEndnotes: true,
          useBase64URL: true,
        })
      } catch (err) {
        if (controller.signal.aborted) return
        setPreviewError(manualErrorMessage(t, err, t("productCenter.manuals.docxPreviewFailed")))
      } finally {
        if (!controller.signal.aborted) {
          setPreviewLoading(false)
        }
      }
    }

    void renderDocxPreview()

    return () => {
      controller.abort()
      container.replaceChildren()
    }
  }, [previewItem, productId, t])

  async function handleUpload(files: FileList | null) {
    if (!files || files.length === 0) return
    setUploading(true)
    setError("")
    try {
      for (const file of Array.from(files)) {
        const response = await uploadProductManualFile(productId, file)
        if (!response.success) {
          throw new Error(response.error?.message || t("productCenter.manuals.uploadFailed", { name: file.name }))
        }
      }
      await load()
    } catch (err) {
      setError(manualErrorMessage(t, err, t("productCenter.manuals.uploadFailedFallback")))
    } finally {
      if (inputRef.current) {
        inputRef.current.value = ""
      }
      setUploading(false)
    }
  }

  async function handleDelete(item: ProductManualFile) {
    setDeletingId(item.id)
    setError("")
    try {
      const response = await deleteProductManualFile(productId, item.id)
      if (!response.success) {
        throw new Error(response.error?.message || t("productCenter.manuals.deleteFailed"))
      }
      await load()
    } catch (err) {
      setError(manualErrorMessage(t, err, t("productCenter.manuals.deleteFailed")))
    } finally {
      setDeletingId(null)
    }
  }

  function startEdit(item: ProductManualFile) {
    setEditingItem(item)
    setEditForm({
      title: item.title || item.filename,
      language: item.language || "default",
      version: item.version || "v1.0",
      visibility: item.visibility === "internal" ? "internal" : "public",
    })
  }

  async function handleSaveEdit() {
    if (!editingItem || !editForm.title.trim()) return
    setSavingEdit(true)
    setError("")
    try {
      const response = await updateProductManualFile(productId, editingItem.id, {
        ...editForm,
        title: editForm.title.trim(),
        language: editForm.language.trim() || "default",
        version: editForm.version.trim() || "v1.0",
      })
      if (!response.success) {
        throw new Error(response.error?.message || t("productCenter.manuals.updateFailed"))
      }
      setEditingItem(null)
      await load()
    } catch (err) {
      setError(manualErrorMessage(t, err, t("productCenter.manuals.updateFailed")))
    } finally {
      setSavingEdit(false)
    }
  }

  const initialLoading = loading && !itemsLoaded
  const manualColumns: TableColumnsType<ProductManualFile> = [
    {
      title: t("productCenter.manuals.columns.fileName"),
      dataIndex: "filename",
      key: "filename",
      width: 300,
      render: (_, item) => (
        <div className="rhd-railops-manual-file-cell">
          <strong title={item.title || item.filename}>{item.title || item.filename}</strong>
          <span title={item.filename}>{item.filename}</span>
          <small>{t("productCenter.manuals.originalFile")} · {item.provider || t("productCenter.manuals.storageProvider")}</small>
        </div>
      ),
    },
    {
      title: t("productCenter.manuals.columns.size"),
      dataIndex: "file_size",
      key: "file_size",
      width: 100,
      render: (value) => formatFileSize(Number(value)),
    },
    {
      title: t("productCenter.manuals.columns.type"),
      key: "type",
      width: 90,
      render: (_, item) => (
        <span className="rhd-railops-code-text" title={item.mime_type || item.provider || "-"}>
          {formatFileType(item)}
        </span>
      ),
    },
    {
      title: t("productCenter.manuals.columns.languageVersion"),
      key: "languageVersion",
      width: 150,
      render: (_, item) => (
        <span className="rhd-railops-muted-line" title={`${item.language || t("productCenter.manuals.defaultLanguage")} · ${item.version || t("productCenter.manuals.defaultVersion")}`}>
          {item.language || t("productCenter.manuals.defaultLanguage")} · {item.version || t("productCenter.manuals.defaultVersion")}
        </span>
      ),
    },
    {
      title: t("productCenter.manuals.columns.visibility"),
      dataIndex: "visibility",
      key: "visibility",
      width: 110,
      render: (_, item) => (
        <StatusTag tone={manualVisibilityTone(item.visibility)}>
          {manualVisibilityLabel(t, item.visibility)}
        </StatusTag>
      ),
    },
    {
      title: t("productCenter.manuals.columns.sync"),
      dataIndex: "rag_sync_status",
      key: "sync",
      width: 180,
      render: (_, item) => (
        <div className="rhd-railops-manual-sync-cell">
          <StatusTag tone={manualSyncTone(item)}>
            {manualSyncLabel(t, item)}
          </StatusTag>
          {item.rag_sync_error ? (
            <span title={item.rag_sync_error}>{item.rag_sync_error}</span>
          ) : null}
        </div>
      ),
    },
    {
      title: t("productCenter.manuals.columns.uploadedAt"),
      dataIndex: "uploaded_at",
      key: "uploaded_at",
      width: 150,
      render: (_, item) => item.uploaded_at || item.created_at || "-",
    },
    {
      title: t("productCenter.manuals.columns.uploadedBy"),
      dataIndex: "uploaded_by",
      key: "uploaded_by",
      width: 120,
      render: (_, item) => (
        <span className="rhd-railops-muted-line" title={item.uploaded_by || "-"}>
          {item.uploaded_by || "-"}
        </span>
      ),
    },
    {
      title: t("productCenter.actions"),
      key: "actions",
      fixed: "right",
      width: 120,
      align: "right",
      render: (_, item) => (
        <div className="rhd-railops-row-actions">
          {canManageManuals ? (
            <IconButton
              icon={<PencilIcon className="size-4" />}
              tooltip={t("productCenter.manuals.editAction")}
              aria-label={t("productCenter.manuals.editAction")}
              onClick={() => startEdit(item)}
            />
          ) : null}
          {item.url ? (
            <IconButton
              icon={<EyeIcon className="size-4" />}
              tooltip={t("productCenter.manuals.previewAction")}
              aria-label={t("productCenter.manuals.previewAction")}
              onClick={() => setPreviewItem(item)}
            />
          ) : (
            <IconButton
              icon={<EyeIcon className="size-4" />}
              tooltip={t("productCenter.manuals.fileUnavailable")}
              aria-label={t("productCenter.manuals.fileUnavailable")}
              disabled
            />
          )}
          {canManageManuals ? (
            <IconButton
              icon={deletingId === item.id ? <Loader2Icon className="size-4 animate-spin" /> : <Trash2Icon className="size-4" />}
              tooltip={t("productCenter.manuals.deleteAction")}
              aria-label={t("productCenter.manuals.deleteAction")}
              danger
              disabled={deletingId === item.id || uploading}
              onClick={() => void handleDelete(item)}
            />
          ) : null}
        </div>
      ),
    },
  ]

  return (
    <div className="rhd-railops-product-tab">
      {error ? (
        <div className="rhd-railops-product-tab-error">
          <ErrorState
            title={t("productCenter.manuals.errorTitle")}
            description={error}
            action={{ label: t("productCenter.retry"), onClick: load }}
          />
        </div>
      ) : null}

      {canManageManuals ? (
        <section className="rhd-railops-product-upload-strip">
          <div>
            <strong>{t("productCenter.manuals.uploadTitle")}</strong>
          </div>
          <input
            ref={inputRef}
            type="file"
            className="hidden"
            multiple
            accept=".pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.md,.markdown,.txt,.html,.htm"
            onChange={(event) => void handleUpload(event.target.files)}
          />
          <RailopsButton
            onClick={() => inputRef.current?.click()}
            disabled={uploading}
          >
            {uploading ? (
              <Loader2Icon className="size-4 animate-spin" />
            ) : (
              <UploadIcon className="size-4" />
            )}
            {uploading ? t("productCenter.saving") : t("productCenter.manuals.chooseFiles")}
          </RailopsButton>
        </section>
      ) : null}

      {initialLoading ? (
        <LoadingSkeleton count={3} layout="table" />
      ) : items.length === 0 ? (
        <EmptyState
          title={t("productCenter.manuals.emptyTitle")}
          className="rhd-railops-product-tab-empty"
        />
      ) : (
        <section className="rhd-railops-product-table-module railops-content-module">
          <header className="railops-content-module-header">
            <div className="railops-content-module-title">
              <strong>{t("productCenter.manuals.tableTitle", { count: items.length })}</strong>
            </div>
          </header>
          <div className="railops-content-module-body">
            <DataTable<ProductManualFile>
              className="rhd-railops-product-table"
              columns={manualColumns}
              dataSource={pagedItems}
              rowKey="id"
              scroll={{ x: 1240 }}
              size="middle"
              total={items.length}
              current={page}
              pageSize={pageSize}
              paginationProps={{ showSizeChanger: true }}
              onPageChange={(nextPage, nextPageSize) => {
                if (nextPageSize !== pageSize) {
                  setPageSize(nextPageSize)
                  setPage(1)
                  return
                }
                setPage(nextPage)
              }}
            />
          </div>
        </section>
      )}

      <StandardModal
        open={previewItem !== null}
        onCancel={() => setPreviewItem(null)}
        width={1120}
        title={
          <div className="min-w-0">
            <strong>{t("productCenter.manuals.previewTitle")}</strong>
            <p className="mt-0.5 truncate text-xs font-normal text-muted-foreground" title={previewItem?.title || previewItem?.filename}>
              {previewItem?.title || previewItem?.filename} · {previewItem ? formatFileSize(previewItem.file_size) : ""} · {previewItem ? formatFileType(previewItem) : ""}
            </p>
          </div>
        }
        footer={
          <>
            <RailopsButton
              onClick={() => previewItem?.url && window.open(previewItem.url, "_blank", "noopener,noreferrer")}
            >
              <DownloadIcon className="size-4" />
              {t("productCenter.manuals.downloadOriginal")}
            </RailopsButton>
            <RailopsButton onClick={() => setPreviewItem(null)}>{t("productCenter.manuals.close")}</RailopsButton>
          </>
        }
      >
        {previewItem ? (
          <>
            {supportsDocxPreview(previewItem) ? (
                <div className="relative h-[68vh] min-h-[420px] overflow-auto rounded-md border bg-muted/40">
                  <div
                    ref={docxPreviewRef}
                    className="min-h-full [&_.docx-wrapper]:!min-h-full [&_.docx-wrapper]:!bg-muted/40 [&_.docx-wrapper]:!py-6"
                  />
                  {previewLoading ? (
                    <div className="absolute inset-0 flex items-center justify-center bg-background/80 backdrop-blur-sm">
                      <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        <Loader2Icon className="size-4 animate-spin" />
                        {t("productCenter.manuals.loadingDocx")}
                      </div>
                    </div>
                  ) : null}
                  {previewError ? (
                    <div className="absolute inset-0 flex flex-col items-center justify-center bg-background px-6 text-center">
                      <FileTextIcon className="mb-4 size-10 text-muted-foreground" />
                      <div className="font-medium">{t("productCenter.manuals.docxPreviewFailed")}</div>
                      <div className="mt-1 max-w-lg text-sm text-muted-foreground">{previewError}</div>
                    </div>
                  ) : null}
                </div>
              ) : supportsInlinePreview(previewItem) ? (
                <iframe
                  title={t("productCenter.manuals.filePreviewTitle", { title: previewItem.title || previewItem.filename })}
                  src={previewItem.url}
                  sandbox=""
                  className="h-[68vh] min-h-[420px] w-full rounded-md border bg-background"
                />
              ) : (
                <div className="flex h-[56vh] min-h-[360px] flex-col items-center justify-center rounded-md border bg-muted/20 px-6 text-center">
                  <FileTextIcon className="mb-4 size-10 text-muted-foreground" />
                  <div className="font-medium">{t("productCenter.manuals.previewUnsupported")}</div>
                  <div className="mt-1 text-sm text-muted-foreground">{t("productCenter.manuals.previewUnsupportedDescription")}</div>
                </div>
              )}
          </>
        ) : null}
      </StandardModal>



      <StandardModal
        open={editingItem !== null}
        onCancel={() => setEditingItem(null)}
        width={520}
        title={t("productCenter.manuals.editTitle")}
        footer={
          <>
            <RailopsButton onClick={() => setEditingItem(null)} disabled={savingEdit}>
              {t("productCenter.cancel")}
            </RailopsButton>
            <RailopsButton onClick={() => void handleSaveEdit()} disabled={savingEdit || !editForm.title.trim()}>
              {savingEdit ? <Loader2Icon className="size-4 animate-spin" /> : null}
              {savingEdit ? t("productCenter.saving") : t("productCenter.save")}
            </RailopsButton>
          </>
        }
      >
        <div className="space-y-4">
              <label className="block space-y-1 text-sm">
                <span>{t("productCenter.manuals.manualName")}</span>
                <input className="w-full rounded-md border px-3 py-2" value={editForm.title} onChange={(event) => setEditForm((current) => ({ ...current, title: event.target.value }))} />
              </label>
              <div className="grid gap-3 sm:grid-cols-2">
                <label className="block space-y-1 text-sm">
                  <span>{t("productCenter.manuals.language")}</span>
                  <input className="w-full rounded-md border px-3 py-2" value={editForm.language} onChange={(event) => setEditForm((current) => ({ ...current, language: event.target.value }))} />
                </label>
                <label className="block space-y-1 text-sm">
                  <span>{t("productCenter.manuals.version")}</span>
                  <input className="w-full rounded-md border px-3 py-2" value={editForm.version} onChange={(event) => setEditForm((current) => ({ ...current, version: event.target.value }))} />
                </label>
              </div>
              <SelectField
                label={t("productCenter.manuals.visibilityLabel")}
                style={{ marginBottom: 0 }}
                selectProps={{
                  "aria-label": t("productCenter.manuals.visibilityLabel"),
                  value: editForm.visibility,
                  onChange: (value) => setEditForm((current) => ({ ...current, visibility: (value as string) ?? "public" })),
                  options: [
                    { value: "public", label: t("productCenter.manuals.visibilityPublic") },
                    { value: "internal", label: t("productCenter.manuals.visibilityInternal") },
                  ],
                  style: { width: "100%" },
                }}
              />
              <div className="text-xs text-muted-foreground">{t("productCenter.manuals.replaceHint")}</div>
        </div>
      </StandardModal>
    </div>
  )
}
