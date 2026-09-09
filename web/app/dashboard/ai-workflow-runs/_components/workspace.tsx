"use client"

import { useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import {
  AlertTriangleIcon,
  Clock3Icon,
  CircleSlash2Icon,
  EyeIcon,
  HeadsetIcon,
  MessageCircleMoreIcon,
  MessageSquareTextIcon,
  ShieldCheckIcon,
  WorkflowIcon,
} from "lucide-react"
import { toast } from "sonner"

import { ConversationDetailDialog } from "@/app/dashboard/conversation-monitor/_components/detail"
import { DashboardListPage } from "@/components/dashboard/list"
import { JsonTreeViewer } from "@/components/json-tree-viewer"
import { ProjectDialog } from "@/components/project-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  fetchAIAgentsAll,
  fetchAIWorkflowRun,
  fetchAIWorkflowRuns,
  fetchConversationDetail,
  fetchConversationMessages,
  type AdminConversationDetail,
  type AdminMessage,
  type AIAgent,
  type AIWorkflowNodeRun,
  type AIWorkflowRun,
  type AIWorkflowSkillAudit,
} from "@/lib/api/admin"
import { formatDateTime } from "@/lib/utils"
import { useI18n } from "@/i18n/provider"
import { WorkflowRunAuditGraph } from "./workflow-run-audit-graph"

type TFunction = (key: string, values?: Record<string, string | number>) => string

function getStatusOptions(t: TFunction) {
  return [
    { value: "all", label: t("workflowRun.allStatus") },
    { value: "1", label: t("workflowRun.completed") },
    { value: "2", label: t("workflowRun.interrupted") },
    { value: "3", label: t("workflowRun.failed") },
  ]
}

function statusBadgeVariant(statusName?: string) {
  switch ((statusName || "").trim()) {
    case "failed":
      return "destructive" as const
    case "interrupted":
      return "outline" as const
    case "completed":
      return "default" as const
    default:
      return "secondary" as const
  }
}

function formatRunStatus(item: Pick<AIWorkflowRun, "status" | "statusName">) {
  return item.statusName || (item.status ? String(item.status) : "-")
}

export function AIWorkflowRunsWorkspace({
  agentId,
  workflowId,
  layout = "page",
}: {
  agentId?: number
  workflowId?: number
  layout?: "page" | "fragment"
}) {
  const t = useI18n()
  const [agents, setAgents] = useState<AIAgent[]>([])
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [activeRun, setActiveRun] = useState<AIWorkflowRun | null>(null)
  const detailRequestIdRef = useRef(0)
  const [conversationOpen, setConversationOpen] = useState(false)
  const [conversationLoading, setConversationLoading] = useState(false)
  const [activeConversation, setActiveConversation] = useState<AdminConversationDetail | null>(null)
  const [conversationMessages, setConversationMessages] = useState<AdminMessage[]>([])
  const statusOptions = useMemo(() => getStatusOptions(t), [t])
  const agentOptions = useMemo(
    () => [
      { value: "all", label: t("workflowRun.allAgents") },
      ...agents.map((agent) => ({
        value: String(agent.id),
        label: agent.name,
        icon: <ShieldCheckIcon className="size-4" />,
      })),
    ],
    [agents, t]
  )

  useEffect(() => {
    let cancelled = false

    async function loadAgents() {
      try {
        const data = await fetchAIAgentsAll()
        if (!cancelled) {
          setAgents(data)
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(error instanceof Error ? error.message : t("workflowRun.loadAgentsFailed"))
        }
      }
    }

    void loadAgents()
    return () => {
      cancelled = true
    }
  }, [t])

  async function openDetail(runId: number) {
    const requestId = detailRequestIdRef.current + 1
    detailRequestIdRef.current = requestId
    setActiveRun(null)
    setDetailOpen(true)
    setDetailLoading(true)
    try {
      const data = await fetchAIWorkflowRun(runId)
      if (detailRequestIdRef.current === requestId) {
        setActiveRun(data)
      }
    } catch (error) {
      if (detailRequestIdRef.current === requestId) {
        toast.error(error instanceof Error ? error.message : t("workflowRun.loadDetailFailed"))
        setDetailOpen(false)
      }
    } finally {
      if (detailRequestIdRef.current === requestId) {
        setDetailLoading(false)
      }
    }
  }

  async function openConversation(conversationId: number) {
    if (!conversationId) {
      return
    }
    setConversationOpen(true)
    setConversationLoading(true)
    try {
      const [detail, messagePage] = await Promise.all([
        fetchConversationDetail(conversationId),
        fetchConversationMessages({ conversationId, limit: 20 }),
      ])
      setActiveConversation(detail)
      setConversationMessages(messagePage.results ?? [])
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("workflowRun.loadConversationFailed"))
      setConversationOpen(false)
    } finally {
      setConversationLoading(false)
    }
  }

  return (
    <>
      <DashboardListPage<AIWorkflowRun>
        layout={layout}
        filters={[
          {
            name: "conversationId",
            label: t("workflowRun.conversationId"),
            placeholder: t("workflowRun.conversationId"),
            defaultValue: "",
            valueType: "number",
            className: "w-full sm:w-44",
          },
          {
            name: "messageId",
            label: t("workflowRun.messageId"),
            placeholder: t("workflowRun.messageId"),
            defaultValue: "",
            valueType: "number",
            className: "w-full sm:w-40",
          },
          {
            name: "workflowVersionId",
            label: t("workflowRun.workflowVersionId"),
            placeholder: t("workflowRun.workflowVersionId"),
            defaultValue: "",
            valueType: "number",
            className: "w-full sm:w-48",
          },
          ...(agentId
            ? []
            : [{
                name: "aiAgentId",
                label: t("workflowRun.agent"),
                type: "select" as const,
                defaultValue: "all",
                allValue: "all",
                valueType: "number" as const,
                options: agentOptions,
                placeholder: t("workflowRun.agent"),
                searchPlaceholder: t("workflowRun.searchAgent"),
                emptyText: t("workflowRun.emptyAgent"),
                className: "w-full sm:w-56",
              }]),
          {
            name: "status",
            label: t("workflowRun.status"),
            type: "select",
            defaultValue: "all",
            allValue: "all",
            valueType: "number",
            options: statusOptions,
            placeholder: t("workflowRun.status"),
            className: "w-full sm:w-44",
          },
        ]}
        fetchList={(query) => fetchAIWorkflowRuns({
          ...query,
          aiAgentId: agentId ?? query.aiAgentId,
          workflowId: workflowId ?? query.workflowId,
        })}
        getItemId={(item) => item.id}
        getRowClassName={() => "cursor-pointer"}
        onRowClick={(item) => void openDetail(item.id)}
        columns={[
          {
            key: "time",
            label: t("workflowRun.startedAt"),
            className: "w-42 text-xs text-muted-foreground",
            render: (item) => formatDateTime(item.startedAt || item.createdAt),
          },
          {
            key: "workflow",
            label: t("workflowRun.workflow"),
            render: (item) => (
              <div className="min-w-0 space-y-1">
                <div className="flex min-w-0 items-center gap-1.5 truncate font-medium">
                  <WorkflowIcon className="size-3.5 shrink-0 text-primary" />
                  {item.workflowName || `Workflow #${item.workflowId}`}
                </div>
                <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
                  <span>v{item.workflowVersion || "-"}</span>
                  <span>#{item.workflowVersionId || "-"}</span>
                </div>
              </div>
            ),
          },
          {
            key: "agent",
            label: t("workflowRun.agent"),
            className: "w-48",
            render: (item) => (
              <span className="inline-flex min-w-0 items-center gap-1.5 truncate">
                <ShieldCheckIcon className="size-3.5 shrink-0 text-primary" />
                {item.aiAgentName || `#${item.aiAgentId}`}
              </span>
            ),
          },
          {
            key: "skill",
            label: t("workflowRun.skill"),
            className: "min-w-[190px]",
            render: (item) => <SkillAuditCell audit={item.skillAudit} t={t} />,
          },
          {
            key: "message",
            label: t("workflowRun.message"),
            className: "w-48",
            render: (item) => (
              <div className="space-y-1 text-sm">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-auto max-w-full justify-start px-0 py-0 text-sm font-normal hover:bg-transparent hover:text-primary"
                  onClick={(event) => {
                    event.stopPropagation()
                    void openConversation(item.conversationId)
                  }}
                >
                  <MessageCircleMoreIcon className="size-3.5" />
                  <span className="truncate">
                    {t("workflowRun.conversationShort", { id: item.conversationId || "-" })}
                  </span>
                </Button>
                <div className="text-xs text-muted-foreground">
                  {t("workflowRun.messageShort", { id: item.messageId || "-" })}
                </div>
              </div>
            ),
          },
          {
            key: "status",
            label: t("workflowRun.status"),
            className: "w-32",
            render: (item) => (
              <Badge variant={statusBadgeVariant(item.statusName)}>
                {formatRunStatus(item)}
              </Badge>
            ),
          },
          {
            key: "duration",
            label: t("workflowRun.duration"),
            className: "w-28 text-right",
            render: (item) => `${item.durationMs || 0} ms`,
          },
          {
            key: "error",
            label: t("workflowRun.error"),
            className: "w-72 max-w-72",
            render: (item) =>
              item.errorMessage ? (
                <ErrorMessagePreview message={item.errorMessage} />
              ) : (
                <span className="text-muted-foreground">-</span>
              ),
          },
          {
            key: "actions",
            label: t("workflowRun.actions"),
            className: "w-16 text-right",
            render: (item) => (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-8"
                title={t("workflowRun.viewDetail")}
                aria-label={t("workflowRun.viewRunDetail", { id: item.id })}
                onClick={(event) => {
                  event.stopPropagation()
                  void openDetail(item.id)
                }}
              >
                <EyeIcon className="size-4" />
              </Button>
            ),
          },
        ]}
        labels={{
          refresh: t("workflowRun.refresh"),
          query: t("workflowRun.query"),
          loading: t("workflowRun.loading"),
          empty: t("workflowRun.empty"),
          loadFailed: t("workflowRun.loadFailed"),
        }}
      />
      <WorkflowRunDetailDialog
        open={detailOpen}
        loading={detailLoading}
        run={activeRun}
        onOpenChange={(open) => {
          setDetailOpen(open)
          if (!open) {
            detailRequestIdRef.current += 1
            setDetailLoading(false)
            setActiveRun(null)
          }
        }}
        t={t}
        onOpenConversation={openConversation}
      />
      <ConversationDetailDialog
        open={conversationOpen}
        loading={conversationLoading}
        saving={false}
        readOnly
        item={activeConversation}
        detail={activeConversation}
        messages={conversationMessages}
        messagesHasMore={false}
        loadingMoreMessages={false}
        onOpenChange={(open) => {
          setConversationOpen(open)
          if (!open) {
            setActiveConversation(null)
            setConversationMessages([])
          }
        }}
        onOpenAssign={() => undefined}
        onDispatch={async () => undefined}
        onOpenTransfer={() => undefined}
        onRead={async () => undefined}
        onOpenClose={() => undefined}
      />
    </>
  )
}

function ErrorMessagePreview({ message }: { message: string }) {
  return (
    <button
      type="button"
      className="flex w-full min-w-0 items-center gap-1.5 text-left text-xs text-destructive"
      title={message}
      onClick={(event) => event.stopPropagation()}
    >
      <AlertTriangleIcon className="size-3.5 shrink-0" />
      <span className="block min-w-0 truncate">{message}</span>
    </button>
  )
}

function WorkflowRunDetailDialog({
  open,
  loading,
  run,
  onOpenChange,
  t,
  onOpenConversation,
}: {
  open: boolean
  loading: boolean
  run: AIWorkflowRun | null
  onOpenChange: (open: boolean) => void
  t: TFunction
  onOpenConversation: (conversationId: number) => void | Promise<void>
}) {
  return (
    <ProjectDialog
      open={open}
      onOpenChange={onOpenChange}
      title={
        <span className="flex items-center gap-2">
          <WorkflowIcon className="size-4" />
          {t("workflowRun.detailTitle")}
        </span>
      }
      size="xl"
      allowFullscreen
      defaultFullscreen
      bodyClassName="min-h-0"
      footer={
        <Button variant="outline" onClick={() => onOpenChange(false)}>
          {t("workflowRun.close")}
        </Button>
      }
    >
      {loading ? (
        <div className="py-10 text-sm text-muted-foreground">{t("workflowRun.loadingDetail")}</div>
      ) : run ? (
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-2 rounded-md border bg-muted/20 px-2.5 py-2 text-xs">
            <CompactRunMeta label={t("workflowRun.workflow")} value={<span className="inline-flex items-center gap-1"><WorkflowIcon className="size-3.5 shrink-0 text-primary" />{run.workflowName || `#${run.workflowId}`}</span>} />
            <CompactRunMeta label={t("workflowRun.version")} value={`v${run.workflowVersion || "-"} / #${run.workflowVersionId}`} />
            <CompactRunMeta label={t("workflowRun.agent")} value={<span className="inline-flex items-center gap-1"><ShieldCheckIcon className="size-3.5 shrink-0 text-primary" />{run.aiAgentName || `#${run.aiAgentId}`}</span>} />
            <CompactRunMeta label={t("workflowRun.status")} value={formatRunStatus(run)} />
            <CompactRunMeta
              label={t("workflowRun.conversationId")}
              value={
                <Button
                  type="button"
                  variant="link"
                  className="h-auto px-0 py-0 text-xs font-medium"
                  onClick={() => void onOpenConversation(run.conversationId)}
                >
                  #{run.conversationId}
                </Button>
              }
            />
            <CompactRunMeta label={t("workflowRun.messageId")} value={`#${run.messageId}`} />
            <CompactRunMeta label={t("workflowRun.startedAt")} value={run.startedAt ? formatDateTime(run.startedAt) : "-"} />
            <CompactRunMeta label={t("workflowRun.duration")} value={`${run.durationMs || 0} ms`} />
            {run.interruptNodeId ? <CompactRunMeta label={t("workflowRun.interruptNode")} value={run.interruptNodeId} /> : null}
          </div>
          {run.errorMessage ? (
            <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
              {run.errorMessage}
            </div>
          ) : null}
          <WorkflowSkillAuditSummary audit={run.skillAudit} t={t} />
          <WorkflowHumanHandlingSummary run={run} t={t} />
          <WorkflowRunAuditGraph key={run.id} run={run} />
          <div className="space-y-3">
            <div className="text-sm font-medium">{t("workflowRun.nodeDetails")}</div>
            {(run.nodes ?? []).map((node, index) => (
              <WorkflowNodeRunBlock key={node.id || node.nodeId || index} node={node} t={t} />
            ))}
            {!run.nodes || run.nodes.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("workflowRun.emptyNodes")}</p>
            ) : null}
          </div>
        </div>
      ) : (
        <div className="py-10 text-sm text-muted-foreground">{t("workflowRun.notFound")}</div>
      )}
    </ProjectDialog>
  )
}

function WorkflowHumanHandlingSummary({ run, t }: { run: AIWorkflowRun; t: TFunction }) {
  const handling = run.humanHandling
  if (!handling) {
    return null
  }
  const workflowHandoffExecuted = (run.nodes ?? []).some(
    (node) => node.nodeType === "handoff_to_human" && node.statusName === "completed"
  )

  return (
    <section className="space-y-2 border-y py-3" aria-label={t("workflowRun.humanHandling")}>
      <div className="flex flex-wrap items-center gap-2">
        <HeadsetIcon className="size-4 text-muted-foreground" />
        <span className="text-sm font-medium">{t("workflowRun.humanHandling")}</span>
        <Badge variant={handling.handledByHuman ? "default" : "outline"}>
          {handling.handledByHuman
            ? t("workflowRun.handledByHuman")
            : t("workflowRun.waitingForHuman")}
        </Badge>
      </div>
      <p className="text-xs leading-5 text-muted-foreground">
        {workflowHandoffExecuted
          ? t("workflowRun.humanHandledInRun")
          : t("workflowRun.humanHandledOutsideRun")}
      </p>
      <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
        {handling.handlerName || handling.handlerUserId ? (
          <span>
            {t("workflowRun.handler")}: <span className="font-medium text-foreground">{handling.handlerName || `#${handling.handlerUserId}`}</span>
          </span>
        ) : null}
        {handling.handoffAt ? (
          <span>
            {t("workflowRun.handoffAt")}: <span className="font-medium text-foreground">{formatDateTime(handling.handoffAt)}</span>
          </span>
        ) : null}
        {handling.firstHumanReplyAt ? (
          <span>
            {t("workflowRun.firstHumanReplyAt")}: <span className="font-medium text-foreground">{formatDateTime(handling.firstHumanReplyAt)}</span>
          </span>
        ) : null}
        {handling.conversationStatusName ? (
          <span>
            {t("workflowRun.conversationStatus")}: <span className="font-medium text-foreground">{handling.conversationStatusName}</span>
          </span>
        ) : null}
      </div>
      {handling.handoffReason ? (
        <div className="text-xs text-muted-foreground">
          {t("workflowRun.handoffReason")}: <span className="font-medium text-foreground">{handling.handoffReason}</span>
        </div>
      ) : null}
    </section>
  )
}

function SkillAuditCell({ audit, t }: { audit?: AIWorkflowSkillAudit; t: TFunction }) {
  if (audit?.state === "selected" && audit.selectedSkillId > 0) {
    return (
      <div className="min-w-0 space-y-1">
        <div className="flex items-center gap-1.5">
          <ShieldCheckIcon className="size-3.5 shrink-0 text-primary" />
          <span className="truncate text-sm font-medium">
            {audit.selectedSkillName || `Skill #${audit.selectedSkillId}`}
          </span>
          <Badge variant="default">{t("workflowRun.skillSelected")}</Badge>
        </div>
        <div className="truncate text-xs text-muted-foreground">
          {audit.matchReason || t("workflowRun.skillSelectedByRuntime")}
        </div>
      </div>
    )
  }

  if (audit?.state === "not_selected") {
    return (
      <div className="space-y-1">
        <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
          <CircleSlash2Icon className="size-3.5" />
          {t("workflowRun.skillNotSelected")}
        </div>
        <div className="text-xs text-muted-foreground">
          {t("workflowRun.skillCandidatesCount", { count: audit.candidateSkills?.length ?? 0 })}
        </div>
      </div>
    )
  }

  return <span className="text-sm text-muted-foreground">{t("workflowRun.skillNotEnabled")}</span>
}

function WorkflowSkillAuditSummary({ audit, t }: { audit?: AIWorkflowSkillAudit; t: TFunction }) {
  const candidates = audit?.candidateSkills ?? []
  const selected = audit?.state === "selected" && audit.selectedSkillId > 0

  return (
    <section className="grid gap-3 border-y py-3 lg:grid-cols-[minmax(0,1fr)_minmax(280px,0.8fr)]" aria-label={t("workflowRun.skillAudit")}>
      <div className="min-w-0 space-y-2 lg:border-r lg:pr-4">
        <div className="flex flex-wrap items-center gap-2">
          <ShieldCheckIcon className="size-4 text-muted-foreground" />
          <span className="text-sm font-medium">{t("workflowRun.skillAudit")}</span>
          <Badge variant={selected ? "default" : audit?.state === "not_selected" ? "outline" : "secondary"}>
            {selected
              ? t("workflowRun.skillSelected")
              : audit?.state === "not_selected"
                ? t("workflowRun.skillNotSelected")
                : t("workflowRun.skillNotEnabled")}
          </Badge>
        </div>
        {selected ? (
          <div>
            <div className="text-sm font-medium">
              {audit?.selectedSkillName || `Skill #${audit?.selectedSkillId}`}
            </div>
            {audit?.selectedSkillDescription ? (
              <p className="mt-1 text-xs leading-5 text-muted-foreground">
                {audit.selectedSkillDescription}
              </p>
            ) : null}
          </div>
        ) : (
          <p className="text-xs leading-5 text-muted-foreground">
            {audit?.state === "not_selected"
              ? t("workflowRun.skillNotSelectedDescription")
              : t("workflowRun.skillNotEnabledDescription")}
          </p>
        )}
        <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span>{t("workflowRun.skillMatchReason")}: <span className="font-medium text-foreground">{audit?.matchReason || "-"}</span></span>
          <span>{t("workflowRun.messageId")}: <span className="font-medium text-foreground">#{audit?.sourceMessageId || "-"}</span></span>
          <span>{t("workflowRun.skillInvokedTools")}: <span className="font-medium text-foreground">{audit?.invokedToolCodes?.length ?? 0}</span></span>
          <span>{t("workflowRun.skillExposedTools")}: <span className="font-medium text-foreground">{audit?.exposedToolCodes?.length ?? 0}</span></span>
        </div>
      </div>
      <div className="min-w-0 space-y-2 lg:pl-1">
        <div className="flex items-center justify-between gap-2">
          <span className="text-xs font-medium text-muted-foreground">{t("workflowRun.skillCandidates")}</span>
          <span className="text-xs text-muted-foreground">{candidates.length}</span>
        </div>
        {candidates.length > 0 ? (
          <div className="flex flex-wrap gap-2">
            {candidates.map((skill) => (
              <Badge
                key={skill.id}
                variant={skill.id === audit?.selectedSkillId ? "default" : "outline"}
                title={skill.description || skill.name}
              >
                #{skill.id} {skill.name || t("workflowRun.skillUnknown")}
              </Badge>
            ))}
          </div>
        ) : (
          <div className="text-xs text-muted-foreground">{t("workflowRun.skillNoCandidates")}</div>
        )}
      </div>
    </section>
  )
}

function WorkflowNodeRunBlock({ node, t }: { node: AIWorkflowNodeRun; t: TFunction }) {
  const inputPreview = node.inputPreview || ""
  const outputPreview = node.outputPreview || ""
  const inputValue = safeParseJSON(inputPreview)
  const outputValue = safeParseJSON(outputPreview)

  return (
    <div className="rounded-md border bg-background p-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-2">
            <MessageSquareTextIcon className="size-4 shrink-0 text-muted-foreground" />
            <span className="truncate text-sm font-medium">{node.nodeId || `#${node.id}`}</span>
            <Badge variant={statusBadgeVariant(node.statusName)}>{node.statusName || node.status || "-"}</Badge>
          </div>
          <div className="mt-1 flex flex-wrap gap-2 text-xs text-muted-foreground">
            <span>{node.nodeType || "unknown"}</span>
            <span className="inline-flex items-center gap-1">
              <Clock3Icon className="size-3.5" />
              {node.durationMs} ms
            </span>
          </div>
        </div>
        {node.errorMessage ? (
          <div className="flex max-w-xl items-start gap-1.5 text-xs text-destructive">
            <AlertTriangleIcon className="mt-0.5 size-3.5 shrink-0" />
            <span className="line-clamp-2 break-all">{node.errorMessage}</span>
          </div>
        ) : null}
      </div>
      <div className="mt-3 grid gap-3 lg:grid-cols-2">
        <PreviewBlock title={t("workflowRun.input")} raw={inputPreview} value={inputValue} />
        <PreviewBlock title={t("workflowRun.output")} raw={outputPreview} value={outputValue} />
      </div>
    </div>
  )
}

function PreviewBlock({
  title,
  raw,
  value,
}: {
  title: string
  raw: string
  value: unknown
}) {
  return (
    <div className="min-w-0">
      <div className="mb-1 text-xs font-medium text-muted-foreground">{title}</div>
      {value !== null ? (
        <JsonTreeViewer value={value} collapsed={2} />
      ) : raw.trim() ? (
        <pre className="max-h-72 overflow-auto rounded-md border bg-muted/20 p-3 text-xs whitespace-pre-wrap break-all">
          {raw}
        </pre>
      ) : (
        <div className="rounded-md border bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
          -
        </div>
      )}
    </div>
  )
}

function CompactRunMeta({ label, value }: { label: string; value: ReactNode }) {
  return (
    <span className="inline-flex max-w-full items-center gap-1.5 rounded-md border bg-background px-2 py-1">
      <span className="shrink-0 text-muted-foreground">{label}</span>
      <span className="min-w-0 truncate font-medium">{value || "-"}</span>
    </span>
  )
}

function safeParseJSON(raw: string): unknown | null {
  const trimmed = raw.trim()
  if (!trimmed) {
    return null
  }
  try {
    return JSON.parse(trimmed)
  } catch {
    return null
  }
}
