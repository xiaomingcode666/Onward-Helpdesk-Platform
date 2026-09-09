"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import {
  ArchiveIcon,
  CheckCircle2Icon,
  GitBranchIcon,
  PencilIcon,
  PlusIcon,
  RefreshCwIcon,
} from "lucide-react"
import { Input, Skeleton } from "antd"
import type { TableColumnsType } from "antd"
import {
  CheckboxField,
  ContentModule,
  DataTable,
  IconButton,
  PageShell,
  RailopsButton,
  StandardModal,
  SearchField,
  SelectField,
  StatusTag,
  TableToolbar,
  type StatusTagTone,
} from "@railops/ui"

import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { CanUseButton } from "@/components/layout/permission-guard"
import { ProductTree, buildProductTreeGroups, getProductDirectoryLabel } from "@/components/product/product-tree"
import { EmptyState, ErrorState, ForbiddenState } from "@/components/shared/error-states"
import { useAuth } from "@/components/auth-provider"
import { useI18n } from "@/i18n/provider"
import {
  createFaultTreeNode,
  listFaultTreeNodes,
  updateFaultTreeNode,
  type EnterpriseFaultTreeNode,
  type FaultTreeNodeType,
  type FaultTreeRiskLevel,
} from "@/lib/api/diagnosis"
import { fetchProductCount, listProducts } from "@/lib/api/enterprise-products"
import type { ProductListItem } from "@/lib/api/types"

type NodeFormState = {
  title: string
  description: string
  nodeType: FaultTreeNodeType
  parentId: string
  faultPattern: EnterpriseFaultTreeNode["fault_pattern"]
  riskLevel: FaultTreeRiskLevel
  triggerConditions: string
  orderIndex: string
  isComposite: boolean
}

const emptyNodeForm: NodeFormState = {
  title: "",
  description: "",
  nodeType: "symptom",
  parentId: "",
  faultPattern: "persistent",
  riskLevel: "low",
  triggerConditions: "[]",
  orderIndex: "0",
  isComposite: false,
}

type I18nT = ReturnType<typeof useI18n>
type FaultTreeDisplayRow = { node: EnterpriseFaultTreeNode; depth: number }

function nodeTypeLabel(t: I18nT, nodeType: FaultTreeNodeType) {
  if (nodeType === "symptom") return t("enterpriseDiagnosis.nodeType.symptom")
  if (nodeType === "check") return t("enterpriseDiagnosis.nodeType.check")
  if (nodeType === "action") return t("enterpriseDiagnosis.nodeType.action")
  return t("enterpriseDiagnosis.nodeType.diagnosis")
}

function riskLabel(t: I18nT, risk: FaultTreeRiskLevel) {
  if (risk === "low") return t("enterpriseDiagnosis.risk.low")
  if (risk === "medium") return t("enterpriseDiagnosis.risk.medium")
  if (risk === "high") return t("enterpriseDiagnosis.risk.high")
  return t("enterpriseDiagnosis.risk.critical")
}

function statusLabel(t: I18nT, status: EnterpriseFaultTreeNode["status"]) {
  if (status === "published") return t("enterpriseDiagnosis.status.published")
  if (status === "archived") return t("enterpriseDiagnosis.status.archived")
  return t("enterpriseDiagnosis.status.draft")
}

function statusTone(status: EnterpriseFaultTreeNode["status"]): StatusTagTone {
  if (status === "published") return "success"
  if (status === "archived") return "disabled"
  return "neutral"
}

function riskTone(risk: FaultTreeRiskLevel): StatusTagTone {
  if (risk === "critical") return "error"
  if (risk === "high") return "warning"
  if (risk === "medium") return "blue"
  return "success"
}

function nodeTypeTone(nodeType: FaultTreeNodeType): StatusTagTone {
  if (nodeType === "action") return "success"
  if (nodeType === "check") return "blue"
  if (nodeType === "diagnosis") return "warning"
  return "neutral"
}

function toNodeForm(node: EnterpriseFaultTreeNode | null, parentId = ""): NodeFormState {
  if (!node) return { ...emptyNodeForm, parentId }
  return {
    title: node.title,
    description: node.description,
    nodeType: node.node_type,
    parentId: node.parent_id,
    faultPattern: node.fault_pattern,
    riskLevel: node.risk_level,
    triggerConditions: node.trigger_conditions || "[]",
    orderIndex: String(node.order_index),
    isComposite: node.is_composite,
  }
}

function buildNodeRows(nodes: EnterpriseFaultTreeNode[]): FaultTreeDisplayRow[] {
  const byParent = new Map<string, EnterpriseFaultTreeNode[]>()
  const ids = new Set(nodes.map((node) => node.id))
  nodes.forEach((node) => {
    const parent = node.parent_id && ids.has(node.parent_id) ? node.parent_id : ""
    const rows = byParent.get(parent) ?? []
    rows.push(node)
    byParent.set(parent, rows)
  })
  byParent.forEach((rows) => rows.sort((a, b) => a.order_index - b.order_index || a.title.localeCompare(b.title)))

  const result: Array<{ node: EnterpriseFaultTreeNode; depth: number }> = []
  const walk = (parentId: string, depth: number) => {
    for (const node of byParent.get(parentId) ?? []) {
      result.push({ node, depth })
      walk(node.id, depth + 1)
    }
  }
  walk("", 0)
  return result
}

function FaultTreeNodeDialog({
  open,
  product,
  nodes,
  editingNode,
  initialParentId,
  onOpenChange,
  onSaved,
}: {
  open: boolean
  product: ProductListItem | null
  nodes: EnterpriseFaultTreeNode[]
  editingNode: EnterpriseFaultTreeNode | null
  initialParentId: string
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const t = useI18n()
  const [form, setForm] = useState<NodeFormState>(emptyNodeForm)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const nodeTypeLabels = {
    symptom: t("enterpriseDiagnosis.nodeType.symptom"),
    check: t("enterpriseDiagnosis.nodeType.check"),
    action: t("enterpriseDiagnosis.nodeType.action"),
    diagnosis: t("enterpriseDiagnosis.nodeType.diagnosis"),
  } satisfies Record<FaultTreeNodeType, string>
  const riskLabels = {
    low: t("enterpriseDiagnosis.risk.low"),
    medium: t("enterpriseDiagnosis.risk.medium"),
    high: t("enterpriseDiagnosis.risk.high"),
    critical: t("enterpriseDiagnosis.risk.critical"),
  } satisfies Record<FaultTreeRiskLevel, string>
  const faultPatternLabels = {
    persistent: t("enterpriseDiagnosis.faultPattern.persistent"),
    intermittent: t("enterpriseDiagnosis.faultPattern.intermittent"),
    conditional: t("enterpriseDiagnosis.faultPattern.conditional"),
  } satisfies Record<NodeFormState["faultPattern"], string>

  useEffect(() => {
    if (!open) return
    setForm(toNodeForm(editingNode, initialParentId))
    setError("")
  }, [editingNode, initialParentId, open])

  const handleSubmit = useCallback(async () => {
    if (!product || !form.title.trim()) {
      setError(t("enterpriseDiagnosis.dialog.enterNodeTitle"))
      return
    }
    try {
      JSON.parse(form.triggerConditions || "[]")
    } catch {
      setError(t("enterpriseDiagnosis.dialog.triggerJsonError"))
      return
    }

    setSaving(true)
    setError("")
    const payload = {
      parent_id: form.parentId,
      title: form.title.trim(),
      description: form.description.trim(),
      node_type: form.nodeType,
      fault_pattern: form.faultPattern,
      trigger_conditions: form.triggerConditions.trim() || "[]",
      is_composite: form.isComposite,
      risk_level: form.riskLevel,
      order_index: Number(form.orderIndex) || 0,
    }
    try {
      const response = editingNode
        ? await updateFaultTreeNode(editingNode.id, payload)
        : await createFaultTreeNode({ product_id: product.id, ...payload })
      if (!response.success) {
        throw new Error(response.error?.message || t("enterpriseDiagnosis.dialog.saveFailed"))
      }
      onSaved()
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("enterpriseDiagnosis.dialog.saveFailed"))
    } finally {
      setSaving(false)
    }
  }, [editingNode, form, onOpenChange, onSaved, product, t])

  return (
    <StandardModal
      open={open}
      onCancel={() => onOpenChange(false)}
      title={editingNode ? t("enterpriseDiagnosis.dialog.editTitle") : t("enterpriseDiagnosis.dialog.createTitle")}
      width={672}
      rootClassName="rhd-railops-scrollable-modal"
      footer={
        <>
          <RailopsButton onClick={() => onOpenChange(false)} disabled={saving}>{t("common.cancel")}</RailopsButton>
          <RailopsButton onClick={handleSubmit} disabled={saving || !product}>{saving ? t("enterpriseDiagnosis.dialog.saving") : t("enterpriseDiagnosis.dialog.save")}</RailopsButton>
        </>
      }
    >
        <div className="grid gap-4 sm:grid-cols-2">
          <label className="grid gap-1.5 sm:col-span-2">
            <span className="text-sm font-medium">{t("enterpriseDiagnosis.dialog.nodeTitle")}</span>
            <Input
              value={form.title}
              placeholder={t("enterpriseDiagnosis.dialog.nodeTitlePlaceholder")}
              onChange={(event) => setForm((current) => ({ ...current, title: event.target.value }))}
            />
          </label>
          <SelectField
            label={t("enterpriseDiagnosis.dialog.nodeType")}
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("enterpriseDiagnosis.dialog.nodeType"),
              value: form.nodeType,
              onChange: (value) => setForm((current) => ({ ...current, nodeType: (value as string) as FaultTreeNodeType })),
              options: Object.entries(nodeTypeLabels).map(([value, label]) => ({ value, label })),
              style: { width: "100%" },
            }}
          />
          <SelectField
            label={t("enterpriseDiagnosis.dialog.parentNode")}
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("enterpriseDiagnosis.dialog.parentNode"),
              placeholder: t("enterpriseDiagnosis.dialog.rootNode"),
              value: form.parentId || undefined,
              onChange: (value) => setForm((current) => ({ ...current, parentId: (value as string) ?? "" })),
              options: nodes
                .filter((node) => node.id !== editingNode?.id)
                .map((node) => ({ value: String(node.id), label: node.title })),
              style: { width: "100%" },
            }}
          />
          <SelectField
            label={t("enterpriseDiagnosis.dialog.faultPattern")}
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("enterpriseDiagnosis.dialog.faultPattern"),
              value: form.faultPattern,
              onChange: (value) => setForm((current) => ({ ...current, faultPattern: (value as string) as NodeFormState["faultPattern"] })),
              options: Object.entries(faultPatternLabels).map(([value, label]) => ({ value, label })),
              style: { width: "100%" },
            }}
          />
          <SelectField
            label={t("enterpriseDiagnosis.dialog.riskLevel")}
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("enterpriseDiagnosis.dialog.riskLevel"),
              value: form.riskLevel,
              onChange: (value) => setForm((current) => ({ ...current, riskLevel: (value as string) as FaultTreeRiskLevel })),
              options: Object.entries(riskLabels).map(([value, label]) => ({ value, label })),
              style: { width: "100%" },
            }}
          />
          <label className="grid gap-1.5 sm:col-span-2">
            <span className="text-sm font-medium">{t("enterpriseDiagnosis.dialog.description")}</span>
            <Input.TextArea
              value={form.description}
              placeholder={t("enterpriseDiagnosis.dialog.descriptionPlaceholder")}
              onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))}
            />
          </label>
          <label className="grid gap-1.5 sm:col-span-2">
            <span className="text-sm font-medium">{t("enterpriseDiagnosis.dialog.triggerConditions")}</span>
            <Input.TextArea
              className="font-mono"
              value={form.triggerConditions}
              placeholder={t("enterpriseDiagnosis.dialog.triggerPlaceholder")}
              onChange={(event) => setForm((current) => ({ ...current, triggerConditions: event.target.value }))}
            />
          </label>
          <label className="grid gap-1.5">
            <span className="text-sm font-medium">{t("enterpriseDiagnosis.dialog.order")}</span>
            <Input
              type="number"
              value={form.orderIndex}
              onChange={(event) => setForm((current) => ({ ...current, orderIndex: event.target.value }))}
            />
          </label>
          <CheckboxField
            className="self-end pb-2"
            style={{ marginBottom: 0 }}
            checkboxProps={{
              checked: form.isComposite,
              onChange: (event) => setForm((current) => ({ ...current, isComposite: event.target.checked })),
            }}
          >
            {t("enterpriseDiagnosis.dialog.compositeNode")}
          </CheckboxField>
        </div>

        {error ? <div className="rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</div> : null}

        
    </StandardModal>
  )
}

export default function EnterpriseDiagnosisPage() {
  const t = useI18n()
  const router = useRouter()
  const searchParams = useSearchParams()
  const breadcrumbItems = useRouteBreadcrumbItems()
  const { ready: authReady, session } = useAuth()
  const isDispatchOnly = authReady && session?.featureFlags?.ai === false
  const canUpdateFaultTree = CanUseButton("product.update", session?.permissions)
  const [products, setProducts] = useState<ProductListItem[]>([])
  const [productTotal, setProductTotal] = useState(0)
  const [productsLoading, setProductsLoading] = useState(true)
  const [productsError, setProductsError] = useState("")
  const [selectedProductId, setSelectedProductId] = useState<number | null>(null)
  const [treeSearch, setTreeSearch] = useState("")
  const [nodes, setNodes] = useState<EnterpriseFaultTreeNode[]>([])
  const [nodesLoading, setNodesLoading] = useState(false)
  const [nodesError, setNodesError] = useState("")
  const [nodeSearch, setNodeSearch] = useState("")
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingNode, setEditingNode] = useState<EnterpriseFaultTreeNode | null>(null)
  const [initialParentId, setInitialParentId] = useState("")
  const [busyNodeId, setBusyNodeId] = useState("")

  const loadProducts = useCallback(async () => {
    setProductsLoading(true)
    setProductsError("")
    try {
      const [response, countResponse] = await Promise.all([
        listProducts({ page_size: 200 }),
        fetchProductCount().catch(() => null),
      ])
      if (!response.success || !response.data) throw new Error(response.error?.message || t("enterpriseDiagnosis.loadProductsFailed"))
      setProducts(response.data)
      setProductTotal(countResponse?.success && countResponse.data ? countResponse.data.total : response.data.length)
      const requested = Number(searchParams.get("product_id"))
      setSelectedProductId((current) => {
        if (current && response.data.some((product) => product.id === current)) return current
        if (requested > 0 && response.data.some((product) => product.id === requested)) return requested
        return response.data[0]?.id ?? null
      })
    } catch (err) {
      setProductsError(err instanceof Error ? err.message : t("enterpriseDiagnosis.loadProductsFailed"))
    } finally {
      setProductsLoading(false)
    }
  }, [searchParams, t])

  const loadNodes = useCallback(async (productId: number) => {
    setNodesLoading(true)
    setNodesError("")
    try {
      const response = await listFaultTreeNodes(productId)
      if (!response.success || !response.data) throw new Error(response.error?.message || t("enterpriseDiagnosis.loadNodesFailed"))
      setNodes(response.data)
    } catch (err) {
      setNodes([])
      setNodesError(err instanceof Error ? err.message : t("enterpriseDiagnosis.loadNodesFailed"))
    } finally {
      setNodesLoading(false)
    }
  }, [t])

  useEffect(() => {
    if (!authReady) return
    if (isDispatchOnly) {
      setProductsLoading(false)
      return
    }
    void loadProducts()
  }, [authReady, isDispatchOnly, loadProducts])
  useEffect(() => {
    if (isDispatchOnly) {
      setNodes([])
      setNodesLoading(false)
      return
    }
    if (!selectedProductId) {
      setNodes([])
      return
    }
    void loadNodes(selectedProductId)
  }, [loadNodes, selectedProductId])

  const filteredProducts = useMemo(() => {
    const keyword = treeSearch.trim().toLowerCase()
    if (!keyword) return products
    return products.filter((product) => `${product.name} ${product.code} ${product.category} ${product.product_line}`.toLowerCase().includes(keyword))
  }, [products, treeSearch])
  const treeGroups = useMemo(() => buildProductTreeGroups(filteredProducts), [filteredProducts])
  const selectedProduct = useMemo(() => products.find((product) => product.id === selectedProductId) ?? null, [products, selectedProductId])
  const displayRows = useMemo(() => {
    const keyword = nodeSearch.trim().toLowerCase()
    const rows = buildNodeRows(nodes)
    if (!keyword) return rows
    return rows.filter(({ node }) => `${node.title} ${node.description} ${node.node_type}`.toLowerCase().includes(keyword))
  }, [nodeSearch, nodes])
  const publishedCount = nodes.filter((node) => node.status === "published").length
  const draftCount = nodes.filter((node) => node.status === "draft").length
  const criticalCount = nodes.filter((node) => node.risk_level === "critical" || node.risk_level === "high").length

  const handleSelectProduct = useCallback((productId: number) => {
    setSelectedProductId(productId)
    const params = new URLSearchParams(searchParams.toString())
    params.set("product_id", String(productId))
    router.replace(`/enterprise/diagnosis?${params.toString()}`, { scroll: false })
  }, [router, searchParams])

  const openCreate = useCallback((parentId = "") => {
    setEditingNode(null)
    setInitialParentId(parentId)
    setDialogOpen(true)
  }, [])

  const openEdit = useCallback((node: EnterpriseFaultTreeNode) => {
    setEditingNode(node)
    setInitialParentId("")
    setDialogOpen(true)
  }, [])

  const changeStatus = useCallback(async (node: EnterpriseFaultTreeNode, status: EnterpriseFaultTreeNode["status"]) => {
    setBusyNodeId(node.id)
    setNodesError("")
    try {
      const response = await updateFaultTreeNode(node.id, { status })
      if (!response.success) throw new Error(response.error?.message || t("enterpriseDiagnosis.updateStatusFailed"))
      if (selectedProductId) await loadNodes(selectedProductId)
    } catch (err) {
      setNodesError(err instanceof Error ? err.message : t("enterpriseDiagnosis.updateStatusFailed"))
    } finally {
      setBusyNodeId("")
    }
  }, [loadNodes, selectedProductId, t])

  const faultTreeColumns = useMemo<TableColumnsType<FaultTreeDisplayRow>>(() => [
    {
      title: t("enterpriseDiagnosis.columns.node"),
      key: "node",
      width: 360,
      render: (_, { node, depth }) => (
        <div className="rhd-railops-diagnosis-node" style={{ paddingLeft: Math.min(depth, 5) * 20 }}>
          <GitBranchIcon className="size-4 shrink-0" />
          <div className="min-w-0">
            <strong>{node.title}</strong>
            {node.description ? <span>{node.description}</span> : null}
          </div>
        </div>
      ),
    },
    {
      title: t("enterpriseDiagnosis.columns.type"),
      key: "type",
      width: 124,
      render: (_, { node }) => (
        <StatusTag tone={nodeTypeTone(node.node_type)}>{nodeTypeLabel(t, node.node_type)}</StatusTag>
      ),
    },
    {
      title: t("enterpriseDiagnosis.columns.risk"),
      key: "risk",
      width: 124,
      render: (_, { node }) => (
        <StatusTag tone={riskTone(node.risk_level)}>{riskLabel(t, node.risk_level)}</StatusTag>
      ),
    },
    {
      title: t("enterpriseDiagnosis.columns.status"),
      key: "status",
      width: 124,
      render: (_, { node }) => (
        <StatusTag tone={statusTone(node.status)}>{statusLabel(t, node.status)}</StatusTag>
      ),
    },
    {
      title: t("enterpriseDiagnosis.columns.order"),
      key: "order",
      dataIndex: ["node", "order_index"],
      width: 96,
    },
    {
      title: t("enterpriseDiagnosis.columns.actions"),
      key: "actions",
      width: 164,
      align: "right",
      render: (_, { node }) => canUpdateFaultTree ? (
        <div className="rhd-railops-diagnosis-actions">
          <IconButton
            icon={<PlusIcon className="size-4" />}
            tooltip={t("enterpriseDiagnosis.actions.addChild")}
            aria-label={t("enterpriseDiagnosis.actions.addChild")}
            onClick={() => openCreate(node.id)}
          />
          <IconButton
            icon={<PencilIcon className="size-4" />}
            tooltip={t("enterpriseDiagnosis.actions.edit")}
            aria-label={t("enterpriseDiagnosis.actions.edit")}
            onClick={() => openEdit(node)}
          />
          {node.status !== "published" ? (
            <IconButton
              icon={<CheckCircle2Icon className="size-4" />}
              tooltip={t("enterpriseDiagnosis.actions.publish")}
              aria-label={t("enterpriseDiagnosis.actions.publish")}
              disabled={busyNodeId === node.id}
              onClick={() => void changeStatus(node, "published")}
            />
          ) : null}
          {node.status !== "archived" ? (
            <IconButton
              icon={<ArchiveIcon className="size-4" />}
              tooltip={t("enterpriseDiagnosis.actions.archive")}
              aria-label={t("enterpriseDiagnosis.actions.archive")}
              disabled={busyNodeId === node.id}
              onClick={() => void changeStatus(node, "archived")}
            />
          ) : null}
        </div>
      ) : null,
    },
  ], [busyNodeId, canUpdateFaultTree, changeStatus, openCreate, openEdit, t])

  if (isDispatchOnly) {
    return (
      <PageShell
        title={t("enterpriseDiagnosis.pageTitle")}
        breadcrumb={breadcrumbItems}
        className="enterprise-redesign ent-object-page ent-light-workbench-page rhd-railops-diagnosis-page"
      >
        <ForbiddenState
          description={t("enterpriseDiagnosis.aiDisabledDescription")}
          action={{ label: t("common.backHome"), href: "/enterprise" }}
        />
      </PageShell>
    )
  }

  return (
    <PageShell
      title={t("enterpriseDiagnosis.pageTitle")}
      breadcrumb={breadcrumbItems}
      className="enterprise-redesign ent-object-page ent-light-workbench-page rhd-railops-diagnosis-page"
    >
      {productsError && products.length === 0 ? (
        <section className="ent-panel"><ErrorState title={t("enterpriseDiagnosis.productLoadFailed")} description={productsError} action={{ label: t("common.retry"), onClick: loadProducts }} /></section>
      ) : (
        <section className="ent-product-workspace rhd-railops-diagnosis-workspace">
          <ProductTree
            title={t("enterpriseDiagnosis.productDirectory")}
            totalCount={productTotal}
            countLabel={t("enterpriseDiagnosis.productCount", { count: productTotal })}
            groups={treeGroups}
            loading={productsLoading}
            selectedGroupKey={selectedProduct ? getProductDirectoryLabel(selectedProduct) : treeGroups[0]?.key ?? null}
            selectedProductId={selectedProductId}
            search={treeSearch}
            searchPlaceholder={t("products.searchPlaceholder")}
            searchAriaLabel={t("enterpriseDiagnosis.searchAriaLabel")}
            onSearchChange={setTreeSearch}
            onSelectGroup={(group) => {
              const id = group.products[0]?.id
              if (id) handleSelectProduct(id)
            }}
            onSelectProduct={(product) => handleSelectProduct(product.id)}
            renderProductMeta={(product) => <small className="kb-product-code">{product.code || `PROD-${product.id}`}</small>}
            renderProductBadge={(product) => <span className="ent-product-list-badge">{product.status === "active" ? t("enterpriseDiagnosis.enabled") : product.status}</span>}
          />

          <main className="ent-product-editor rhd-railops-diagnosis-editor">
            <ContentModule
              className="rhd-railops-diagnosis-module"
              title={selectedProduct ? t("enterpriseDiagnosis.selectedTreeTitle", { name: selectedProduct.name }) : t("enterpriseDiagnosis.treeTitle")}
              note={selectedProduct?.code || t("enterpriseDiagnosis.selectProductHint")}
              extra={(
                <div className="rhd-railops-diagnosis-header-actions">
                  <IconButton
                    icon={<RefreshCwIcon className={nodesLoading ? "size-4 animate-spin" : "size-4"} />}
                    tooltip={t("enterpriseDiagnosis.refresh")}
                    aria-label={t("enterpriseDiagnosis.refresh")}
                    disabled={!selectedProductId || nodesLoading}
                    onClick={() => selectedProductId && void loadNodes(selectedProductId)}
                  />
                  {canUpdateFaultTree ? (
                    <RailopsButton onClick={() => openCreate()} disabled={!selectedProductId}>
                      <PlusIcon className="size-4" />{t("enterpriseDiagnosis.newRootNode")}
                    </RailopsButton>
                  ) : null}
                </div>
              )}
            >

              {selectedProduct ? (
                <>
                  <div className="rhd-railops-diagnosis-stats">
                    <div className="rhd-railops-diagnosis-stat"><span>{t("enterpriseDiagnosis.stats.published")}</span><strong>{publishedCount}</strong></div>
                    <div className="rhd-railops-diagnosis-stat"><span>{t("enterpriseDiagnosis.stats.draft")}</span><strong>{draftCount}</strong></div>
                    <div className="rhd-railops-diagnosis-stat"><span>{t("enterpriseDiagnosis.stats.highRisk")}</span><strong>{criticalCount}</strong></div>
                  </div>
                  <TableToolbar
                    className="rhd-railops-diagnosis-toolbar"
                    search={(
                      <SearchField
                        className="rhd-railops-diagnosis-search"
                        value={nodeSearch}
                        placeholder={t("enterpriseDiagnosis.searchPlaceholder")}
                        onChange={(event) => setNodeSearch(event.target.value)}
                      />
                    )}
                    filters={<span className="rhd-railops-table-summary">{displayRows.length} / {nodes.length}</span>}
                  />
                  {nodesError ? <div className="mb-4 rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">{nodesError}</div> : null}
                  {nodesLoading ? (
                    <div className="space-y-2">{Array.from({ length: 6 }).map((_, index) => <Skeleton.Node key={index} active className="w-full" style={{ width: "100%", height: 48 }} />)}</div>
                  ) : displayRows.length === 0 ? (
                    <EmptyState title={nodes.length ? t("enterpriseDiagnosis.noMatchNodes") : t("enterpriseDiagnosis.noTreeTitle")} action={nodes.length || !canUpdateFaultTree ? undefined : { label: t("enterpriseDiagnosis.newRootNode"), onClick: () => openCreate() }} />
                  ) : (
                    <div className="rhd-railops-diagnosis-table-wrap">
                      <DataTable<FaultTreeDisplayRow>
                        className="rhd-railops-device-table rhd-railops-diagnosis-table"
                        size="small"
                        columns={faultTreeColumns}
                        dataSource={displayRows}
                        emptyDescription={t("enterpriseDiagnosis.noMatchNodes")}
                        rowKey={(record) => record.node.id}
                        scroll={{ x: 860 }}
                      />
                    </div>
                  )}
                </>
              ) : (
                <EmptyState title={t("enterpriseDiagnosis.noProductsTitle")} />
              )}
            </ContentModule>
          </main>
        </section>
      )}

      <FaultTreeNodeDialog
        open={canUpdateFaultTree && dialogOpen}
        product={selectedProduct}
        nodes={nodes}
        editingNode={editingNode}
        initialParentId={initialParentId}
        onOpenChange={setDialogOpen}
        onSaved={() => selectedProductId && void loadNodes(selectedProductId)}
      />
    </PageShell>
  )
}
