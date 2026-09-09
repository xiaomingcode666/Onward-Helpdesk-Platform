"use client"

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react"
import {
  ArrowRightIcon,
  ArrowDownIcon,
  ArrowUpIcon,
  CheckCircle2Icon,
  CircleAlertIcon,
  DatabaseIcon,
  HistoryIcon,
  LifeBuoyIcon,
  MessageSquareTextIcon,
  PlugIcon,
  SaveIcon,
  SendIcon,
  SettingsIcon,
  ShieldCheckIcon,
  TicketCheckIcon,
  Trash2Icon,
  UserRoundCheckIcon,
  WorkflowIcon,
} from "lucide-react"
import { toast } from "sonner"
import { UnderlineTabs, type RailopsTabItem } from "@railops/ui"

import { ContentEditor } from "@/components/content-editor"
import { OptionCombobox } from "@/components/option-combobox"
import { ModuleLoading } from "@/components/shared/loading-states"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import { useI18n } from "@/i18n/provider"
import { translateCurrentMessage } from "@/i18n/messages"
import {
  createAIAgent,
  fetchAIAgent,
  fetchAIAgentCapabilities,
  fetchAIAgentWorkflow,
  fetchAIConfigsAll,
  fetchAIWorkflowDefaultDefinition,
  fetchAIWorkflowNodeSpecs,
  fetchAIWorkflowVersion,
  fetchAIWorkflowVersions,
  fetchAgentTeamsAll,
  fetchKnowledgeBasesAll,
  fetchMCPCatalog,
  fetchSkillDefinitionsAll,
  publishAIAgentWorkflow,
  saveAIAgentWorkflow,
  updateAIAgent,
  validateAIWorkflow,
  type AIAgent,
  type AIConfig,
  type AIWorkflow,
  type AIWorkflowDefinition,
  type AIWorkflowNodeSpec,
  type AIWorkflowVersion,
  type AIWorkflowValidationResult,
  type AdminAgentTeam,
  type CreateAIAgentPayload,
  type KnowledgeBase,
  type MCPToolCatalogItem,
  type MCPToolSourceType,
  type SkillDefinition,
} from "@/lib/api/admin"
import {
  AIAgentFallbackMode,
  AIAgentHandoffMode,
  AIModelType,
  IMConversationServiceMode,
  Status,
} from "@/lib/generated/enums"
import { WorkflowEditor } from "../../ai-workflows/_components/workflow-editor"

type DirectToolItem = CreateAIAgentPayload["directTools"][number]

type DirectToolOption = {
  value: string
  label: string
  meta: DirectToolItem
  sourceType: MCPToolSourceType
  groupLabel: string
}

export type AIAgentWorkbenchSection =
  | "basic"
  | "model"
  | "knowledge"
  | "skills"
  | "tools"
  | "workflow"
  | "handoff"
  | "versions"

type WorkflowViewMode = "business" | "advanced"

const aiAgentsConfigI18nPrefix = "dashboardExtract.aiAgentsConfig."
type AiAgentsConfigT = ReturnType<typeof useI18n>
const ac = (t: AiAgentsConfigT, key: string, values?: Record<string, string | number>) => t(`${aiAgentsConfigI18nPrefix}${key}`, values)

function createFallbackDefinition(): AIWorkflowDefinition {
  const acm = (key: string, values?: Record<string, string | number>) => translateCurrentMessage(`${aiAgentsConfigI18nPrefix}${key}`, values)
  return {
    schemaVersion: 1,
    entryNodeId: "start_1",
    nodes: [
      {
        id: "start_1",
        type: "start",
        name: acm("nodeName.start"),
        position: { x: 0, y: 80 },
        config: {},
      },
      {
        id: "end_1",
        type: "end",
        name: acm("nodeName.end"),
        position: { x: 260, y: 80 },
        config: {},
      },
    ],
    edges: [{ id: "edge_start_end", source: "start_1", target: "end_1" }],
  }
}

function toText(value: string | number | undefined | null) {
  if (value === undefined || value === null || value === 0) return ""
  return String(value)
}

function uniqueNumbers(input: number[]) {
  return Array.from(new Set(input.filter((id) => Number.isFinite(id) && id > 0)))
}

function isWorkflowPublished(agent: AIAgent | null) {
  return Boolean(agent?.workflowPublished || (agent?.workflowVersionId ?? 0) > 0)
}

function WorkflowBusinessOverview({
  definition,
  workflowPublished,
  workflowVersionId,
  knowledgeCount,
  skillCount,
  teamCount,
  handoffMode,
  disabled,
  onOpenAdvanced,
  onValidate,
  onPublish,
}: {
  definition: AIWorkflowDefinition
  workflowPublished: boolean
  workflowVersionId: number
  knowledgeCount: number
  skillCount: number
  teamCount: number
  handoffMode: string
  disabled: boolean
  onOpenAdvanced: () => void
  onValidate: () => void
  onPublish: () => void
}) {
  const t = useI18n()
  const nodeTypes = new Set(definition.nodes.map((node) => node.type))
  const hasKnowledgePath = nodeTypes.has("knowledge_retrieve")
  const hasTicketPath = nodeTypes.has("human_confirm") && nodeTypes.has("create_ticket")
  const hasHandoffPath = nodeTypes.has("handoff_to_human")
  const hasVideoPath = nodeTypes.has("create_video_meeting")
  const hasKnowledgeCandidatePath = nodeTypes.has("create_knowledge_candidate")
  const actionLabels = [
    hasHandoffPath ? ac(t, "action.handoff") : "",
    hasTicketPath ? ac(t, "action.createTicket") : "",
    hasVideoPath ? ac(t, "action.video") : "",
    hasKnowledgeCandidatePath ? ac(t, "action.knowledgeCandidate") : "",
  ].filter(Boolean)
  const productTeamMode = handoffMode === String(AIAgentHandoffMode.DefaultTeamPool)
  const issues = [
    !workflowPublished ? ac(t, "issues.noPublishedVersion") : "",
    knowledgeCount === 0 ? ac(t, "issues.noKnowledgeBound") : "",
    !hasKnowledgePath ? ac(t, "issues.noKnowledgeNode") : "",
    hasHandoffPath && productTeamMode && teamCount === 0 ? ac(t, "issues.handoffTeamMissing") : "",
  ].filter(Boolean)
  const handoffModeLabels: Record<string, string> = {
    [String(AIAgentHandoffMode.WaitPool)]: ac(t, "handoffModeLabels.waitPool"),
    [String(AIAgentHandoffMode.DefaultTeamPool)]: ac(t, "handoffModeLabels.defaultTeamPool"),
    [String(AIAgentHandoffMode.AIHoldAndNotify)]: ac(t, "handoffModeLabels.aiHoldAndNotify"),
  }
  const stages: {
    title: string
    detail: string
    icon: ReactNode
    status: "ready" | "pending" | "external"
  }[] = [
    {
      title: ac(t, "stages.receive.title"),
      detail: ac(t, "stages.receive.detail"),
      icon: <MessageSquareTextIcon className="size-4" />,
      status: "ready",
    },
    {
      title: ac(t, "stages.diagnose.title"),
      detail: ac(t, "stages.diagnose.detail", { knowledgeCount, skillCount }),
      icon: <DatabaseIcon className="size-4" />,
      status: knowledgeCount > 0 && hasKnowledgePath ? "ready" : "pending",
    },
    {
      title: ac(t, "stages.answer.title"),
      detail: hasHandoffPath ? ac(t, "stages.answer.detailHandoff") : ac(t, "stages.answer.detailContinue"),
      icon: <MessageSquareTextIcon className="size-4" />,
      status: "ready",
    },
    ...(hasHandoffPath
      ? [{
          title: ac(t, "stages.handoff.title"),
          detail: ac(t, "stages.handoff.detail", {
            mode: handoffModeLabels[handoffMode] ?? ac(t, "stages.handoffNotConfigured"),
            teamCount,
          }),
          icon: <UserRoundCheckIcon className="size-4" />,
          status: (!productTeamMode || teamCount > 0) ? "ready" as const : "pending" as const,
        }]
      : []),
    ...(hasTicketPath
      ? [{
          title: ac(t, "stages.ticket.title"),
          detail: ac(t, "stages.ticket.detail"),
          icon: <TicketCheckIcon className="size-4" />,
          status: "ready" as const,
        }]
      : []),
  ]

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto w-full max-w-[1500px] space-y-6 p-5 lg:p-6">
        <section className="flex flex-col gap-3 border-b pb-5 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-lg font-semibold">{ac(t, "overview.title")}</h2>
              <Badge variant={workflowPublished ? "default" : "outline"}>
                {workflowPublished ? ac(t, "overview.publishedVersion", { version: workflowVersionId }) : ac(t, "overview.notPublished")}
              </Badge>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button type="button" variant="outline" onClick={onValidate} disabled={disabled}>
              <CheckCircle2Icon />
              {ac(t, "overview.validateFlow")}
            </Button>
            <Button type="button" variant="outline" onClick={onOpenAdvanced}>
              <WorkflowIcon />
              {ac(t, "overview.advancedEdit")}
            </Button>
            <Button type="button" onClick={onPublish} disabled={disabled}>
              <SendIcon />
              {ac(t, "overview.publishFlow")}
            </Button>
          </div>
        </section>

        <section aria-label={ac(t, "overview.ariaStages")}>
          <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-5">
            {stages.map((stage, index) => (
              <div key={stage.title} className="relative rounded-md border bg-card p-3">
                <div className="flex items-center justify-between gap-2">
                  <span className="flex size-7 items-center justify-center rounded-md bg-muted text-muted-foreground">
                    {stage.icon}
                  </span>
                  <Badge
                    variant={
                      stage.status === "pending"
                        ? "destructive"
                        : stage.status === "external"
                          ? "outline"
                          : "secondary"
                    }
                  >
                    {stage.status === "ready"
                      ? ac(t, "status.ready")
                      : stage.status === "external"
                        ? ac(t, "status.external")
                        : ac(t, "status.pending")}
                  </Badge>
                </div>
                <div className="mt-3 text-sm font-medium">{index + 1}. {stage.title}</div>
                <div className="mt-1 text-xs leading-5 text-muted-foreground">{stage.detail}</div>
                {index < stages.length - 1 ? (
                  <ArrowRightIcon className="absolute -right-3 top-1/2 z-10 hidden size-4 -translate-y-1/2 text-muted-foreground xl:block" />
                ) : null}
              </div>
            ))}
          </div>
        </section>

        <section className="grid border-y py-4 lg:grid-cols-2 lg:divide-x" aria-label={ac(t, "overview.ariaBoundary")}>
          <div className="px-1 py-2 lg:pr-5">
            <div className="flex items-center gap-2 text-sm font-medium">
              <MessageSquareTextIcon className="size-4 text-muted-foreground" /> {ac(t, "overview.inNodeCapability")}
            </div>
          </div>
          <div className="px-1 py-2 lg:px-5">
            <div className="flex items-center gap-2 text-sm font-medium">
              <ShieldCheckIcon className="size-4 text-muted-foreground" /> {ac(t, "overview.actionBoundary")}
            </div>
            <div className="mt-2 flex flex-wrap gap-1.5">
              {actionLabels.length > 0 ? (
                actionLabels.map((label) => <Badge key={label} variant="secondary">{label}</Badge>)
              ) : (
                <Badge variant="outline">{ac(t, "overview.noExternalAction")}</Badge>
              )}
            </div>
          </div>
        </section>

        <section>
          <div className="flex items-center gap-2 text-sm font-medium">
            {issues.length === 0 ? (
              <CheckCircle2Icon className="size-4 text-primary" />
            ) : (
              <CircleAlertIcon className="size-4 text-amber-600" />
            )}
            {ac(t, "overview.launchCheck")}
          </div>
          <div className="mt-2 flex flex-wrap gap-2">
            {issues.length === 0 ? (
              <Badge variant="secondary">{ac(t, "overview.launchReady")}</Badge>
            ) : (
              issues.map((issue) => <Badge key={issue} variant="outline">{issue}</Badge>)
            )}
          </div>
        </section>
      </div>
    </div>
  )
}

export function AIAgentConfigWorkbench({
  agentId,
  contextBadges,
  defaultLlmModel = "",
  initialSection = "basic",
  llmModels = [],
  lockedKnowledgeIds = [],
  notice,
  onAgentSaved,
  onAgentCreated,
  onSectionChange,
  dialogHeaderInset = true,
  workflowBindingPanel,
  workflowEditing = true,
}: {
  agentId?: number | null
  contextBadges?: ReactNode
  defaultLlmModel?: string
  initialSection?: AIAgentWorkbenchSection
  llmModels?: string[]
  lockedKnowledgeIds?: number[]
  notice?: ReactNode
  onAgentSaved?: () => void
  onAgentCreated?: (agent: AIAgent) => void
  onSectionChange?: (section: AIAgentWorkbenchSection) => void
  dialogHeaderInset?: boolean
  workflowBindingPanel?: ReactNode
  workflowEditing?: boolean
}) {
  const t = useI18n()
  const [currentAgentId, setCurrentAgentId] = useState(agentId ?? null)
  const [activeSection, setActiveSection] = useState<AIAgentWorkbenchSection>(initialSection)
  const [agent, setAgent] = useState<AIAgent | null>(null)
  const [, setWorkflow] = useState<AIWorkflow | null>(null)
  const [workflowVersions, setWorkflowVersions] = useState<AIWorkflowVersion[]>([])
  const [nodeSpecs, setNodeSpecs] = useState<AIWorkflowNodeSpec[]>([])
  const [, setValidation] = useState<AIWorkflowValidationResult | null>(null)
  const [metadataLoading, setMetadataLoading] = useState(true)
  const [agentLoading, setAgentLoading] = useState(Boolean(agentId && agentId > 0))
  const [workflowVersionsLoading, setWorkflowVersionsLoading] = useState(false)
  const [savingAgent, setSavingAgent] = useState(false)
  const [savingWorkflow, setSavingWorkflow] = useState(false)
  const loading = metadataLoading || agentLoading || workflowVersionsLoading

  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [aiConfigId, setAIConfigId] = useState("0")
  const [llmModelName, setLlmModelName] = useState("")
  const [serviceMode, setServiceMode] = useState(String(IMConversationServiceMode.AIFirst))
  const [systemPrompt, setSystemPrompt] = useState("")
  const [welcomeMessage, setWelcomeMessage] = useState("")
  const [replyTimeoutSeconds, setReplyTimeoutSeconds] = useState("180")
  const [handoffMode, setHandoffMode] = useState(String(AIAgentHandoffMode.WaitPool))
  const [fallbackMode, setFallbackMode] = useState(String(AIAgentFallbackMode.NoAnswer))
  const [fallbackMessage, setFallbackMessage] = useState("")
  const [selectedKnowledgeIds, setSelectedKnowledgeIds] = useState<number[]>([])
  const [selectedTeamIds, setSelectedTeamIds] = useState<number[]>([])
  const [selectedSkillIds, setSelectedSkillIds] = useState<number[]>([])
  const [directTools, setDirectTools] = useState<DirectToolItem[]>([])
  const [loadedLlmModels, setLoadedLlmModels] = useState<string[]>([])
  const [loadedDefaultLlmModel, setLoadedDefaultLlmModel] = useState("")

  const [definition, setDefinition] = useState<AIWorkflowDefinition>(createFallbackDefinition)
  const [workflowEditorKey, setWorkflowEditorKey] = useState(0)
  const [workflowView, setWorkflowView] = useState<WorkflowViewMode>("business")

  const [aiConfigs, setAIConfigs] = useState<AIConfig[]>([])
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBase[]>([])
  const [agentTeams, setAgentTeams] = useState<AdminAgentTeam[]>([])
  const [skills, setSkills] = useState<SkillDefinition[]>([])
  const [toolCatalog, setToolCatalog] = useState<MCPToolCatalogItem[]>([])
  const [knowledgeToAdd, setKnowledgeToAdd] = useState("")
  const [teamToAdd, setTeamToAdd] = useState("")
  const [skillToAdd, setSkillToAdd] = useState("")
  const [directToolGroupToAdd, setDirectToolGroupToAdd] = useState("")
  const [directToolToAdd, setDirectToolToAdd] = useState("")

  useEffect(() => {
    setCurrentAgentId(agentId ?? null)
  }, [agentId])

  useEffect(() => {
    setActiveSection(initialSection)
  }, [initialSection])

  useEffect(() => {
    if (
      (!workflowEditing && !workflowBindingPanel && activeSection === "workflow") ||
      (!workflowEditing && activeSection === "versions")
    ) {
      setActiveSection("basic")
      onSectionChange?.("basic")
    }
  }, [activeSection, onSectionChange, workflowBindingPanel, workflowEditing])

  useEffect(() => {
    if (llmModels.length > 0) return
    let active = true
    void fetchAIAgentCapabilities()
      .then((response) => {
        if (!active) return
        const capability = response.capabilities.find((item) => item.type === "llm")
        setLoadedLlmModels(capability?.models ?? [])
        setLoadedDefaultLlmModel(capability?.modelName ?? "")
      })
      .catch(() => {
        if (!active) return
        setLoadedLlmModels([])
        setLoadedDefaultLlmModel("")
      })
    return () => {
      active = false
    }
  }, [llmModels.length])

  const lockedKnowledgeIdList = useMemo(
    () => uniqueNumbers(lockedKnowledgeIds),
    [lockedKnowledgeIds]
  )
  const lockedKnowledgeIdSet = useMemo(
    () => new Set(lockedKnowledgeIdList),
    [lockedKnowledgeIdList]
  )

  const replaceWorkflowDefinition = useCallback((nextDefinition: AIWorkflowDefinition) => {
    setDefinition(nextDefinition)
    setWorkflowEditorKey((current) => current + 1)
  }, [])

  const loadData = useCallback(async () => {
    const hasExistingAgent = Boolean(currentAgentId && currentAgentId > 0)
    setMetadataLoading(true)
    setAgentLoading(hasExistingAgent)
    setWorkflowVersionsLoading(false)
    try {
      const [
        specs,
        defaultDefinition,
        configs,
        bases,
        teams,
        skillList,
        catalog,
      ] = await Promise.all([
        fetchAIWorkflowNodeSpecs(),
        fetchAIWorkflowDefaultDefinition().catch(() => createFallbackDefinition()),
        fetchAIConfigsAll({ modelType: AIModelType.LLM }),
        fetchKnowledgeBasesAll({ status: Status.Ok }),
        fetchAgentTeamsAll(),
        fetchSkillDefinitionsAll({ status: Status.Ok }),
        fetchMCPCatalog(),
      ])

      setNodeSpecs(specs ?? [])
      setAIConfigs(configs ?? [])
      setKnowledgeBases(bases ?? [])
      setAgentTeams(teams ?? [])
      setSkills(skillList ?? [])
      setToolCatalog(catalog ?? [])
      setMetadataLoading(false)

      if (!currentAgentId || currentAgentId <= 0) {
        setAgent(null)
        setWorkflow(null)
        setWorkflowVersions([])
        setName("")
        setDescription("")
        setAIConfigId("0")
        setLlmModelName("")
        setServiceMode(String(IMConversationServiceMode.AIFirst))
        setSystemPrompt("")
        setWelcomeMessage("")
        setReplyTimeoutSeconds("180")
        setHandoffMode(String(AIAgentHandoffMode.WaitPool))
        setFallbackMode(String(AIAgentFallbackMode.NoAnswer))
        setFallbackMessage("")
        setSelectedKnowledgeIds(lockedKnowledgeIdList)
        setSelectedTeamIds([])
        setSelectedSkillIds([])
        setDirectTools([])
        replaceWorkflowDefinition(defaultDefinition ?? createFallbackDefinition())
        setValidation(null)
        setAgentLoading(false)
        return
      }

      const [agentDetail, workflowDetail] = await Promise.all([
        fetchAIAgent(currentAgentId),
        fetchAIAgentWorkflow(currentAgentId),
      ])

      setAgent(agentDetail)
      setWorkflow(workflowDetail)
      setName(agentDetail.name)
      setDescription(agentDetail.description || "")
      setAIConfigId(agentDetail.aiConfigId > 0 ? toText(agentDetail.aiConfigId) : "0")
      setLlmModelName(agentDetail.llmModelName || "")
      setServiceMode(String(agentDetail.serviceMode || IMConversationServiceMode.AIFirst))
      setSystemPrompt(agentDetail.systemPrompt || "")
      setWelcomeMessage(agentDetail.welcomeMessage || "")
      setReplyTimeoutSeconds(String(agentDetail.replyTimeoutSeconds ?? 180))
      setHandoffMode(String(agentDetail.handoffMode || AIAgentHandoffMode.WaitPool))
      setFallbackMode(String(agentDetail.fallbackMode || AIAgentFallbackMode.NoAnswer))
      setFallbackMessage(agentDetail.fallbackMessage || "")
      setSelectedKnowledgeIds(
        uniqueNumbers([...lockedKnowledgeIdList, ...(agentDetail.knowledgeIds ?? [])])
      )
      setSelectedTeamIds((agentDetail.teams ?? []).map((team) => team.id))
      setSelectedSkillIds(agentDetail.skillIds ?? [])
      setDirectTools(agentDetail.directTools ?? [])
      replaceWorkflowDefinition(workflowDetail?.draftDefinition ?? defaultDefinition ?? createFallbackDefinition())
      setValidation(null)
      setAgentLoading(false)

      if (workflowDetail?.id > 0) {
        setWorkflowVersionsLoading(true)
        try {
          const [versionPage, resolvedVersion] = await Promise.all([
            fetchAIWorkflowVersions({ workflowId: workflowDetail.id, limit: 20 }),
            agentDetail.workflowVersionId > 0
              ? fetchAIWorkflowVersion(agentDetail.workflowVersionId)
              : Promise.resolve(null),
          ])
          setWorkflowVersions(versionPage.results ?? [])
          if (!workflowEditing && resolvedVersion?.definition) {
            replaceWorkflowDefinition(resolvedVersion.definition)
          }
        } catch (error) {
          toast.error(error instanceof Error ? error.message : ac(t, "toast.workflowVersionLoadFailed"))
        } finally {
          setWorkflowVersionsLoading(false)
        }
      } else {
        setWorkflowVersions([])
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ac(t, "toast.configLoadFailed"))
    } finally {
      setMetadataLoading(false)
      setAgentLoading(false)
      setWorkflowVersionsLoading(false)
    }
  }, [currentAgentId, lockedKnowledgeIdList, replaceWorkflowDefinition, workflowEditing])

  useEffect(() => {
    void loadData()
  }, [loadData])

  const serviceModeOptions = useMemo(
    () => [
      { value: String(IMConversationServiceMode.AIOnly), label: ac(t, "serviceMode.aiOnly") },
      { value: String(IMConversationServiceMode.HumanOnly), label: ac(t, "serviceMode.humanOnly") },
      { value: String(IMConversationServiceMode.AIFirst), label: ac(t, "serviceMode.aiFirst") },
    ],
    [t]
  )
  const workflowNodeTypes = useMemo(
    () => new Set(definition.nodes.map((node) => node.type)),
    [definition.nodes]
  )
  const workflowCapabilityLocked = (agent?.workflowVersionId ?? 0) > 0
  const workflowAllowsHumanHandoff =
    !workflowCapabilityLocked || workflowNodeTypes.has("handoff_to_human")

  useEffect(() => {
    if (workflowAllowsHumanHandoff) return
    setServiceMode(String(IMConversationServiceMode.AIOnly))
    if (activeSection === "handoff") {
      setActiveSection("basic")
      onSectionChange?.("basic")
    }
  }, [activeSection, onSectionChange, workflowAllowsHumanHandoff])
  const handoffModeOptions = useMemo(
    () => [
      { value: String(AIAgentHandoffMode.WaitPool), label: ac(t, "handoffModeLabels.waitPool") },
      { value: String(AIAgentHandoffMode.DefaultTeamPool), label: ac(t, "handoffModeLabels.defaultTeamPool") },
      { value: String(AIAgentHandoffMode.AIHoldAndNotify), label: ac(t, "handoffModeLabels.aiHoldAndNotify") },
    ],
    [t]
  )
  const fallbackModeOptions = useMemo(
    () => [
      { value: String(AIAgentFallbackMode.NoAnswer), label: ac(t, "fallbackMode.noAnswer") },
      { value: String(AIAgentFallbackMode.SuggestRetry), label: ac(t, "fallbackMode.suggestRetry") },
    ],
    [t]
  )
  const aiConfigOptions = useMemo(
    () => [
      { value: "0", label: ac(t, "aiConfig.platform") },
      ...aiConfigs.map((item) => ({ value: String(item.id), label: `${item.name} · ${item.modelName}` })),
    ],
    [aiConfigs, t]
  )
  const selectedAIConfig = useMemo(
    () => aiConfigs.find((item) => String(item.id) === aiConfigId),
    [aiConfigId, aiConfigs]
  )
  const llmModelOptions = useMemo(() => {
    const availableModels = llmModels.length > 0 ? llmModels : loadedLlmModels
    const platformDefaultModel = defaultLlmModel || loadedDefaultLlmModel
    const names = Array.from(
      new Set(
        [...availableModels, llmModelName]
          .map((item) => item.trim())
          .filter(Boolean)
      )
    )
    return [
      {
        value: "",
        label: platformDefaultModel
          ? ac(t, "llm.followPlatformDefaultWithModel", { model: platformDefaultModel })
          : ac(t, "llm.followPlatformDefault"),
      },
      ...names.map((name) => ({
        value: name,
        label: name === platformDefaultModel ? ac(t, "llm.platformDefaultSuffix", { name }) : name,
      })),
    ]
  }, [defaultLlmModel, llmModelName, llmModels, loadedDefaultLlmModel, loadedLlmModels, t])
  const knowledgeOptions = useMemo(
    () => knowledgeBases.map((item) => ({ value: String(item.id), label: item.name })),
    [knowledgeBases]
  )
  const teamOptions = useMemo(
    () => agentTeams.map((item) => ({ value: String(item.id), label: item.name })),
    [agentTeams]
  )
  const skillOptions = useMemo(
    () => skills.map((item) => ({ value: String(item.id), label: item.name })),
    [skills]
  )
  const directToolOptions = useMemo<DirectToolOption[]>(
    () =>
      toolCatalog
        .filter((tool) => !tool.autoInjected && tool.sourceType === "mcp")
        .map((tool) => ({
          value: tool.toolCode,
          label: `${tool.title || tool.toolName} · ${tool.toolCode}`,
          sourceType: tool.sourceType,
          groupLabel: tool.sourceType === "builtin" ? ac(t, "tool.builtin") : tool.serverCode,
          meta: {
            toolCode: tool.toolCode,
            serverCode: tool.serverCode,
            toolName: tool.toolName,
            title: tool.title || tool.toolName,
            description: tool.description || "",
            arguments: undefined,
          },
        })),
    [t, toolCatalog]
  )
  const directToolGroupOptions = useMemo(
    () =>
      Array.from(
        new Map(
          directToolOptions.map((option) => [
            option.groupLabel,
            { value: option.groupLabel, label: option.groupLabel },
          ])
        ).values()
      ),
    [directToolOptions]
  )
  const addableDirectToolOptions = useMemo(
    () =>
      directToolOptions.filter(
        (option) =>
          option.groupLabel === directToolGroupToAdd &&
          !directTools.some((tool) => tool.toolCode === option.value)
      ),
    [directToolGroupToAdd, directToolOptions, directTools]
  )

  function selectedOptions(ids: number[], options: { value: string; label: string }[]) {
    return ids
      .map((id) => options.find((option) => Number(option.value) === id))
      .filter((option): option is { value: string; label: string } => !!option)
  }

  function addSelected(value: string, current: number[], setNext: (ids: number[]) => void) {
    const id = Number(value)
    if (!Number.isFinite(id) || id <= 0 || current.includes(id)) return
    setNext([...current, id])
  }

  function moveKnowledge(index: number, direction: -1 | 1) {
    const targetIndex = index + direction
    if (targetIndex < 0 || targetIndex >= selectedKnowledgeIds.length) return
    const next = [...selectedKnowledgeIds]
    const current = next[index]
    next[index] = next[targetIndex]
    next[targetIndex] = current
    setSelectedKnowledgeIds(next)
  }

  function addDirectTool(value: string) {
    const option = directToolOptions.find((item) => item.value === value)
    if (!option) return
    setDirectTools((current) =>
      current.some((tool) => tool.toolCode === option.meta.toolCode)
        ? current
        : [...current, option.meta]
    )
    setDirectToolToAdd("")
  }

  function buildPayload(): CreateAIAgentPayload {
    return {
      name: name.trim(),
      description: description.trim(),
      aiConfigId: Number(aiConfigId),
      llmModelName: aiConfigId === "0" ? llmModelName.trim() : "",
      serviceMode: workflowAllowsHumanHandoff
        ? Number(serviceMode)
        : IMConversationServiceMode.AIOnly,
      systemPrompt: systemPrompt.trim(),
      welcomeMessage: welcomeMessage.trim(),
      replyTimeoutSeconds: Number(replyTimeoutSeconds),
      teamIds: uniqueNumbers(selectedTeamIds),
      handoffMode: Number(handoffMode),
      fallbackMode: Number(fallbackMode),
      fallbackMessage: fallbackMessage.trim(),
      knowledgeIds: uniqueNumbers([...lockedKnowledgeIdList, ...selectedKnowledgeIds]),
      skillIds: uniqueNumbers(selectedSkillIds),
      directTools,
      graphTools: [],
    }
  }

  async function saveAgentSettings() {
    setSavingAgent(true)
    try {
      const payload = buildPayload()
      if (agent) {
        await updateAIAgent({ id: agent.id, ...payload })
        toast.success(ac(t, "toast.saved"))
        await loadData()
      } else {
        const created = await createAIAgent(payload)
        setCurrentAgentId(created.id)
        setAgent(created)
        toast.success(ac(t, "toast.created"))
        onAgentCreated?.(created)
      }
      onAgentSaved?.()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ac(t, "toast.saveFailed"))
    } finally {
      setSavingAgent(false)
    }
  }

  async function saveWorkflowDraft() {
    if (!currentAgentId) return
    setSavingWorkflow(true)
    try {
      const saved = await saveAIAgentWorkflow({
        agentId: currentAgentId,
        name: "",
        description: "",
        definition,
      })
      setWorkflow(saved)
      toast.success(ac(t, "toast.draftSaved"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ac(t, "toast.draftSaveFailed"))
    } finally {
      setSavingWorkflow(false)
    }
  }

  async function validateWorkflowDraft() {
    setSavingWorkflow(true)
    try {
      const result = await validateAIWorkflow(definition)
      setValidation(result)
      toast[result.valid ? "success" : "error"](
        result.valid ? ac(t, "toast.validatePassed") : ac(t, "toast.validateFailed")
      )
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ac(t, "toast.validateError"))
    } finally {
      setSavingWorkflow(false)
    }
  }

  async function restoreDefaultWorkflow() {
    if (savingWorkflow || loading) return
    setSavingWorkflow(true)
    try {
      const defaultDefinition = await fetchAIWorkflowDefaultDefinition()
      replaceWorkflowDefinition(defaultDefinition ?? createFallbackDefinition())
      setValidation(null)
      toast.success(ac(t, "toast.restored"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ac(t, "toast.restoreFailed"))
    } finally {
      setSavingWorkflow(false)
    }
  }

  async function publishWorkflow() {
    if (!currentAgentId) return
    setSavingWorkflow(true)
    try {
      const saved = await saveAIAgentWorkflow({
        agentId: currentAgentId,
        name: "",
        description: "",
        definition,
      })
      setWorkflow(saved)
      const version = await publishAIAgentWorkflow(currentAgentId, definition)
      toast.success(ac(t, "toast.publishVersion", { version: version.version }))
      setAgent((current) =>
        current ? { ...current, workflowVersionId: version.id } : current
      )
      setWorkflow((current) =>
        current ? { ...current, publishedVersionId: version.id } : saved
      )
      if (saved.id > 0) {
        const versionPage = await fetchAIWorkflowVersions({
          workflowId: saved.id,
          limit: 20,
        })
        setWorkflowVersions(versionPage.results ?? [])
      } else {
        setWorkflowVersions((current) =>
          current.some((item) => item.id === version.id) ? current : [version, ...current]
        )
      }
      const refreshedAgent = await fetchAIAgent(currentAgentId)
      setAgent(refreshedAgent)
      onAgentSaved?.()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ac(t, "toast.publishFailed"))
    } finally {
      setSavingWorkflow(false)
    }
  }

  const sections: { key: AIAgentWorkbenchSection; title: string; icon: ReactNode }[] = [
    { key: "basic", title: ac(t, "section.basic"), icon: <SettingsIcon /> },
    { key: "model", title: ac(t, "section.model"), icon: <SettingsIcon /> },
    { key: "knowledge", title: ac(t, "section.knowledge"), icon: <DatabaseIcon /> },
    ...(workflowEditing || workflowBindingPanel
      ? [{ key: "workflow" as const, title: workflowEditing ? ac(t, "section.workflowEditing") : ac(t, "section.workflowTemplate"), icon: <WorkflowIcon /> }]
      : []),
    { key: "skills", title: ac(t, "section.skills"), icon: <ShieldCheckIcon /> },
    { key: "tools", title: ac(t, "section.tools"), icon: <PlugIcon /> },
    ...(workflowAllowsHumanHandoff
      ? [{ key: "handoff" as const, title: ac(t, "section.handoff"), icon: <LifeBuoyIcon /> }]
      : []),
    ...(workflowEditing
      ? [{ key: "versions" as const, title: ac(t, "section.versions"), icon: <HistoryIcon /> }]
      : []),
  ]
  const sectionTabs: RailopsTabItem[] = sections.map((section) => ({
    value: section.key,
    label: section.title,
    icon: section.icon,
  }))

  const selectedKnowledgeOptions = selectedOptions(selectedKnowledgeIds, knowledgeOptions)
  const selectedTeamOptions = selectedOptions(selectedTeamIds, teamOptions)
  const selectedSkillDefinitions = skills.filter((skill) => selectedSkillIds.includes(skill.id))
  const workflowPublished = isWorkflowPublished(agent)
  const workflowStateText =
    agent?.workflowStateText || (workflowPublished ? ac(t, "workflowState.published") : ac(t, "workflowState.unpublished"))
  const agentStatusText = agent
    ? [ac(t, "agentStatus.enabled"), ac(t, "agentStatus.disabled"), ac(t, "agentStatus.deleted")][agent.status] || agent.statusName
    : ""
  const sectionLoadingMap = {
    basic: agentLoading,
    model: metadataLoading || agentLoading,
    knowledge: metadataLoading || agentLoading,
    skills: metadataLoading || agentLoading,
    tools: metadataLoading || agentLoading,
    workflow: metadataLoading || agentLoading || workflowVersionsLoading,
    handoff: metadataLoading || agentLoading || workflowVersionsLoading,
    versions: agentLoading || workflowVersionsLoading,
  } satisfies Record<AIAgentWorkbenchSection, boolean>
  const sectionLoading = sectionLoadingMap[activeSection]
  const loadingExistingAgent =
    agentLoading && Boolean(currentAgentId && currentAgentId > 0) && agent?.id !== currentAgentId
  const contentShellClassName = activeSection === "workflow" && !sectionLoading ? "h-full min-h-0" : "w-full p-6"

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden bg-background">
      <div className={`flex shrink-0 flex-col gap-2 border-b px-4 py-2 sm:flex-row sm:items-center sm:justify-between sm:gap-4 sm:px-5 ${dialogHeaderInset ? "sm:pr-28" : ""}`}>
        <div className="flex min-w-0 items-start gap-2.5 sm:items-center">
          <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
            <ShieldCheckIcon className="size-4" />
          </div>
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <h1 className="min-w-0 basis-full truncate text-base font-semibold sm:basis-auto">
              {loadingExistingAgent ? ac(t, "header.loadingExisting") : agent?.name ?? ac(t, "header.newAgent")}
            </h1>
            {loadingExistingAgent ? (
              <Badge variant="outline">{ac(t, "header.loading")}</Badge>
            ) : (
              <>
                {agent ? <Badge variant="secondary">{agentStatusText}</Badge> : null}
                <Badge variant={workflowPublished ? "default" : "outline"}>
                  {workflowStateText}
                </Badge>
                {workflowPublished ? (
                  <Badge variant="secondary">
                    {workflowEditing ? ac(t, "badge.draftWorkflow") : ac(t, "badge.fixedWorkflow")}
                  </Badge>
                ) : null}
                <Badge variant={agent?.activeReleaseId ? "default" : "outline"}>
                  {agent?.activeReleaseId ? ac(t, "badge.deployed") : ac(t, "badge.notDeployed")}
                </Badge>
              </>
            )}
            {contextBadges}
          </div>
        </div>
        <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto sm:justify-end">
          {activeSection === "workflow" ? (
            null
          ) : (
            <>
              <Button
                type="button"
                variant="outline"
                className="flex-1 sm:flex-none"
                disabled={savingAgent || loading}
                onClick={saveAgentSettings}
              >
                <SaveIcon className="size-4" />
                {ac(t, "actions.save")}
              </Button>
              {workflowEditing ? (
                <Button
                  type="button"
                  className="flex-1 sm:flex-none"
                  disabled={savingWorkflow || loading || !currentAgentId}
                  onClick={publishWorkflow}
                >
                  <SendIcon className="size-4" />
                  {ac(t, "actions.publishWorkflowVersion")}
                </Button>
              ) : null}
            </>
          )}
        </div>
      </div>

      <div className="flex min-h-0 flex-1 flex-col bg-background">
        {notice ? (
          <div className="shrink-0 border-b border-sky-200 bg-sky-50 px-5 py-2 text-sm text-sky-900">
            {notice}
          </div>
        ) : null}
        <div className="shrink-0 bg-muted/20 px-4 py-2">
          <div className="no-scrollbar w-full overflow-x-auto [&_.railops-underline-tab-list]:min-w-[640px]">
            <UnderlineTabs
              ariaLabel={ac(t, "aria.tabs")}
              items={sectionTabs}
              value={activeSection}
              onChange={(value) => {
                setActiveSection(value as AIAgentWorkbenchSection)
                onSectionChange?.(value as AIAgentWorkbenchSection)
              }}
            />
          </div>
        </div>

        <main className="min-h-0 flex-1 overflow-y-auto bg-background">
          <div className={contentShellClassName}>
            {sectionLoading ? (
              <AIAgentWorkbenchSectionLoading section={activeSection} />
            ) : (
              <>
              {activeSection === "basic" ? (
                <ConfigSection>
                  <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                    <FieldBlock label={ac(t, "field.name")}>
                      <Input value={name} onChange={(event) => setName(event.target.value)} />
                    </FieldBlock>
                    <FieldBlock label={ac(t, "field.serviceMode")}>
                      <OptionCombobox
                        value={serviceMode}
                        options={workflowAllowsHumanHandoff
                          ? serviceModeOptions
                          : [{ value: String(IMConversationServiceMode.AIOnly), label: ac(t, "serviceMode.aiOnly") }]}
                        placeholder={ac(t, "placeholder.selectServiceMode")}
                        disabled={!workflowAllowsHumanHandoff}
                        onChange={setServiceMode}
                      />
                    </FieldBlock>
                  </div>
                  <FieldBlock label={ac(t, "field.description")}>
                    <Textarea rows={4} value={description} onChange={(event) => setDescription(event.target.value)} />
                  </FieldBlock>
                </ConfigSection>
              ) : null}

              {activeSection === "model" ? (
                <ConfigSection>
                  <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
                    <FieldBlock label={ac(t, "field.modelRoute")}>
                      <OptionCombobox
                        value={aiConfigId}
                        options={aiConfigOptions}
                        placeholder={ac(t, "placeholder.selectModelRoute")}
                        searchPlaceholder={ac(t, "placeholder.searchModelRoute")}
                        emptyText={ac(t, "placeholder.emptyModelRoute")}
                        onChange={setAIConfigId}
                      />
                    </FieldBlock>
                    <FieldBlock label={ac(t, "field.chatModel")}>
                      {aiConfigId === "0" ? (
                        <OptionCombobox
                          value={llmModelName}
                          options={llmModelOptions}
                          placeholder={ac(t, "placeholder.selectChatModel")}
                          searchPlaceholder={ac(t, "placeholder.searchChatModel")}
                          emptyText={ac(t, "placeholder.emptyChatModel")}
                          onChange={setLlmModelName}
                        />
                      ) : (
                        <Input value={selectedAIConfig?.modelName || ""} disabled />
                      )}
                    </FieldBlock>
                    <FieldBlock label={ac(t, "field.replyTimeoutSeconds")}>
                      <Input
                        type="number"
                        min={0}
                        step={1}
                        value={replyTimeoutSeconds}
                        onChange={(event) => setReplyTimeoutSeconds(event.target.value)}
                      />
                    </FieldBlock>
                  </div>
                  <FieldBlock label={ac(t, "field.systemPrompt")}>
                    <ContentEditor
                      value={{ mode: "markdown", raw: systemPrompt }}
                      allowedModes={["markdown"]}
                      height={360}
                      onChange={(next) => setSystemPrompt(next.raw)}
                    />
                  </FieldBlock>
                  <FieldBlock label={ac(t, "field.welcomeMessage")}>
                    <Textarea rows={5} value={welcomeMessage} onChange={(event) => setWelcomeMessage(event.target.value)} />
                  </FieldBlock>
                </ConfigSection>
              ) : null}

              {activeSection === "knowledge" ? (
                <ConfigSection>
                  <AddRow
                    value={knowledgeToAdd}
                    options={knowledgeOptions.filter((option) => !selectedKnowledgeIds.includes(Number(option.value)))}
                    placeholder={ac(t, "placeholder.selectKnowledgeBase")}
                    onValueChange={setKnowledgeToAdd}
                    onAdd={() => {
                      addSelected(knowledgeToAdd, selectedKnowledgeIds, setSelectedKnowledgeIds)
                      setKnowledgeToAdd("")
                    }}
                  />
                  <div className="space-y-2 rounded-md border p-3">
                    {selectedKnowledgeOptions.length === 0 ? (
                      <div className="text-sm text-muted-foreground">{ac(t, "knowledge.empty")}</div>
                    ) : (
                      selectedKnowledgeOptions.map((option, index) => {
                        const knowledgeId = Number(option.value)
                        const locked = lockedKnowledgeIdSet.has(knowledgeId)
                        return (
                        <div key={option.value} className="flex items-center gap-2">
                          <Badge variant="secondary" className="min-w-8 justify-center">{index + 1}</Badge>
                          <div className="flex flex-1 items-center gap-2 text-sm">
                            <span>{option.label}</span>
                            {locked ? <Badge variant="outline">{ac(t, "knowledge.productDefault")}</Badge> : null}
                          </div>
                          <Button
                            type="button"
                            variant="outline"
                            size="icon-sm"
                            disabled={index === 0 || locked}
                            onClick={() => moveKnowledge(index, -1)}
                          >
                            <ArrowUpIcon />
                          </Button>
                          <Button
                            type="button"
                            variant="outline"
                            size="icon-sm"
                            disabled={index === selectedKnowledgeOptions.length - 1 || locked}
                            onClick={() => moveKnowledge(index, 1)}
                          >
                            <ArrowDownIcon />
                          </Button>
                          <Button
                            type="button"
                            variant="outline"
                            size="icon-sm"
                            disabled={locked}
                            onClick={() => setSelectedKnowledgeIds((current) => current.filter((id) => id !== knowledgeId))}
                          >
                            <Trash2Icon />
                          </Button>
                        </div>
                      )})
                    )}
                  </div>
                </ConfigSection>
              ) : null}

              {activeSection === "skills" ? (
                <ConfigSection>
                  <AddRow
                    value={skillToAdd}
                    options={skillOptions.filter((option) => !selectedSkillIds.includes(Number(option.value)))}
                    placeholder={ac(t, "placeholder.selectSkill")}
                    onValueChange={setSkillToAdd}
                    onAdd={() => {
                      addSelected(skillToAdd, selectedSkillIds, setSelectedSkillIds)
                      setSkillToAdd("")
                    }}
                  />
                  <div className="space-y-2">
                    {selectedSkillDefinitions.length === 0 ? (
                      <div className="rounded-md border border-border bg-muted p-4 text-sm text-muted-foreground">
                        {ac(t, "skills.none")}
                      </div>
                    ) : (
                      selectedSkillDefinitions.map((skill) => {
                        const workflowOwnedTools = skill.toolWhitelist.filter((toolCode) =>
                          toolCode.includes("create_ticket") || toolCode.includes("handoff")
                        )
                        return (
                          <div key={skill.id} className="flex flex-col gap-3 rounded-md border p-3 lg:flex-row lg:items-start">
                            <div className="min-w-0 flex-1">
                              <div className="flex flex-wrap items-center gap-2">
                                <span className="text-sm font-medium">{skill.name}</span>
                                <Badge variant={workflowOwnedTools.length > 0 ? "destructive" : "secondary"}>
                                  {workflowOwnedTools.length > 0 ? ac(t, "skills.sideEffectIntercepted") : ac(t, "skills.inNodeSkill")}
                                </Badge>
                                <Badge variant="outline">{ac(t, "skills.toolCount", { count: skill.toolWhitelist.length })}</Badge>
                              </div>
                              {skill.description.trim() ? (
                                <div className="mt-1 text-xs leading-5 text-muted-foreground">{skill.description}</div>
                              ) : null}
                              <div className="mt-2 flex flex-wrap gap-1.5">
                                {skill.toolWhitelist.length === 0 ? (
                                  <Badge variant="outline">{ac(t, "skills.noToolPermission")}</Badge>
                                ) : (
                                  skill.toolWhitelist.slice(0, 4).map((toolCode) => (
                                    <Badge key={toolCode} variant="secondary" className="font-mono text-rhd-xs font-normal">
                                      {toolCode}
                                    </Badge>
                                  ))
                                )}
                              </div>
                            </div>
                            <Button
                              type="button"
                              variant="outline"
                              size="icon-sm"
                              title={ac(t, "skills.removeTitle", { name: skill.name })}
                              onClick={() => setSelectedSkillIds((current) => current.filter((item) => item !== skill.id))}
                            >
                              <Trash2Icon />
                            </Button>
                          </div>
                        )
                      })
                    )}
                  </div>
                </ConfigSection>
              ) : null}

              {activeSection === "tools" ? (
                <ConfigSection>
                  <div className="border-y py-3">
                    <div className="text-sm font-medium">{ac(t, "tools.title")}</div>
                    <div className="mt-1 text-xs leading-5 text-muted-foreground">
                      {ac(t, "tools.helper")}
                    </div>
                  </div>
                  <div className="grid grid-cols-1 gap-3 lg:grid-cols-[220px_minmax(0,1fr)_auto]">
                    <OptionCombobox
                      value={directToolGroupToAdd}
                      options={directToolGroupOptions}
                      placeholder={ac(t, "placeholder.selectToolGroup")}
                      onChange={(value) => {
                        setDirectToolGroupToAdd(value)
                        setDirectToolToAdd("")
                      }}
                    />
                    <OptionCombobox
                      value={directToolToAdd}
                      options={addableDirectToolOptions}
                      placeholder={ac(t, "placeholder.selectTool")}
                      onChange={(value) => {
                        setDirectToolToAdd(value)
                        addDirectTool(value)
                      }}
                    />
                    <Button
                      type="button"
                      variant="outline"
                      disabled={!directToolToAdd}
                      onClick={() => addDirectTool(directToolToAdd)}
                    >
                      {ac(t, "actions.add")}
                    </Button>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {directTools.length === 0 ? (
                      <div className="text-sm text-muted-foreground">{ac(t, "tools.empty")}</div>
                    ) : (
                      directTools.map((tool) => (
                        <Badge key={tool.toolCode} variant="secondary" className="gap-1 pr-1">
                          {tool.title || tool.toolCode}
                          <span className="text-rhd-2xs text-muted-foreground/80">{tool.serverCode || "MCP"}</span>
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="size-5"
                            onClick={() => setDirectTools((current) => current.filter((item) => item.toolCode !== tool.toolCode))}
                          >
                            <Trash2Icon className="size-3" />
                          </Button>
                        </Badge>
                      ))
                    )}
                  </div>
                </ConfigSection>
              ) : null}

              {activeSection === "workflow" ? (
                !workflowEditing ? (
                  <ConfigSection>
                    {workflowBindingPanel}
                  </ConfigSection>
                ) : workflowView === "business" ? (
                  <WorkflowBusinessOverview
                    definition={definition}
                    workflowPublished={workflowPublished}
                    workflowVersionId={agent?.workflowVersionId ?? 0}
                    knowledgeCount={selectedKnowledgeIds.length}
                    skillCount={selectedSkillIds.length}
                    teamCount={selectedTeamIds.length}
                    handoffMode={handoffMode}
                    disabled={savingWorkflow || loading || !currentAgentId}
                    onOpenAdvanced={() => setWorkflowView("advanced")}
                    onValidate={validateWorkflowDraft}
                    onPublish={publishWorkflow}
                  />
                ) : (
                  <div className="flex h-full min-h-0 flex-col">
                    <div className="flex shrink-0 items-center justify-between border-b px-4 py-2">
                      <div>
                        <div className="text-sm font-medium">{ac(t, "advanced.title")}</div>
                      </div>
                      <Button type="button" variant="outline" size="sm" onClick={() => setWorkflowView("business")}>
                        {ac(t, "advanced.backToBusiness")}
                      </Button>
                    </div>
                    <div className="min-h-0 flex-1">
                      <WorkflowEditor
                        key={workflowEditorKey}
                        definition={definition}
                        nodeSpecs={nodeSpecs}
                        onDefinitionChange={setDefinition}
                        onRestoreDefault={restoreDefaultWorkflow}
                        restoreDefaultDisabled={savingWorkflow || loading}
                        onValidate={validateWorkflowDraft}
                        validateDisabled={savingWorkflow || loading || !currentAgentId}
                        onSaveDraft={saveWorkflowDraft}
                        saveDraftDisabled={savingWorkflow || loading || !currentAgentId}
                        onPublish={publishWorkflow}
                        publishDisabled={savingWorkflow || loading || !currentAgentId}
                      />
                    </div>
                  </div>
                )
              ) : null}

              {activeSection === "handoff" ? (
                <ConfigSection>
                  <div className="grid gap-3 border-y py-3 md:grid-cols-2 xl:grid-cols-4">
                    {[ac(t, "handoffTriggers.requiresHuman"), ac(t, "handoffTriggers.lowConfidence"), ac(t, "handoffTriggers.safetyRisk"), ac(t, "handoffTriggers.repeatedDenials")].map((trigger) => (
                      <div key={trigger} className="flex items-center gap-2 text-sm">
                        <CheckCircle2Icon className="size-4 shrink-0 text-muted-foreground" />
                        {trigger}
                      </div>
                    ))}
                  </div>
                  <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                    <FieldBlock label={ac(t, "field.handoffMode")}>
                      <OptionCombobox value={handoffMode} options={handoffModeOptions} placeholder={ac(t, "placeholder.selectHandoffMode")} onChange={setHandoffMode} />
                    </FieldBlock>
                    <FieldBlock label={ac(t, "field.fallbackStrategy")}>
                      <OptionCombobox value={fallbackMode} options={fallbackModeOptions} placeholder={ac(t, "placeholder.selectFallbackStrategy")} onChange={setFallbackMode} />
                    </FieldBlock>
                  </div>
                  <AddRow
                    value={teamToAdd}
                    options={teamOptions.filter((option) => !selectedTeamIds.includes(Number(option.value)))}
                    placeholder={ac(t, "placeholder.selectProductTeam")}
                    onValueChange={setTeamToAdd}
                    onAdd={() => {
                      addSelected(teamToAdd, selectedTeamIds, setSelectedTeamIds)
                      setTeamToAdd("")
                    }}
                  />
                  <BadgeList
                    empty={ac(t, "teams.empty")}
                    items={selectedTeamOptions}
                    onRemove={(id) => setSelectedTeamIds((current) => current.filter((item) => item !== id))}
                  />
                  {handoffMode === String(AIAgentHandoffMode.DefaultTeamPool) && selectedTeamIds.length === 0 ? (
                    <div className="flex items-start gap-2 rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900">
                      <CircleAlertIcon className="mt-0.5 size-4 shrink-0" />
                      {ac(t, "teams.notBound")}
                    </div>
                  ) : null}
                  <FieldBlock label={ac(t, "field.fallbackCopy")}>
                    <Textarea rows={5} value={fallbackMessage} onChange={(event) => setFallbackMessage(event.target.value)} />
                  </FieldBlock>
                </ConfigSection>
              ) : null}

              {activeSection === "versions" ? (
                <ConfigSection>
                  <div className="overflow-hidden rounded-md border">
                    {workflowVersions.length > 0 ? (
                      <Table>
                        <TableHeader className="bg-muted/40">
                          <TableRow>
                            <TableHead className="w-28">{ac(t, "versions.version")}</TableHead>
                            <TableHead>{ac(t, "versions.publishedAt")}</TableHead>
                            <TableHead>{ac(t, "versions.publishedBy")}</TableHead>
                            <TableHead>{ac(t, "versions.status")}</TableHead>
                            <TableHead className="text-right">{ac(t, "versions.definitionHash")}</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {workflowVersions.map((version) => (
                            <TableRow key={version.id}>
                              <TableCell>
                                <div className="flex items-center gap-2">
                                  <span className="font-medium">v{version.version}</span>
                                  {agent?.workflowVersionId === version.id ? (
                                    <Badge variant="secondary">{ac(t, "versions.draftSelected")}</Badge>
                                  ) : null}
                                </div>
                              </TableCell>
                              <TableCell className="text-muted-foreground">
                                {version.publishedAt || version.createdAt || "-"}
                              </TableCell>
                              <TableCell>{version.publishedByName || "-"}</TableCell>
                              <TableCell>
                                <Badge variant={version.status === Status.Ok ? "outline" : "secondary"}>
                                  {version.status === Status.Ok ? ac(t, "versions.enabled") : ac(t, "versions.disabled")}
                                </Badge>
                              </TableCell>
                              <TableCell className="text-right font-mono text-xs text-muted-foreground">
                                {version.definitionHash ? version.definitionHash.slice(0, 8) : "-"}
                              </TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    ) : (
                      <div className="p-4 text-sm text-muted-foreground">{ac(t, "versions.empty")}</div>
                    )}
                  </div>
                </ConfigSection>
              ) : null}

              </>
            )}
          </div>
        </main>
      </div>
    </div>
  )
}

function AIAgentWorkbenchSectionLoading({
  section,
}: {
  section: AIAgentWorkbenchSection
}) {
  const t = useI18n()
  const loadingMeta = {
    basic: { variant: "detail", count: 3, label: ac(t, "loading.basic") },
    model: { variant: "detail", count: 3, label: ac(t, "loading.model") },
    knowledge: { variant: "list", count: 4, label: ac(t, "loading.knowledge") },
    skills: { variant: "list", count: 4, label: ac(t, "loading.skills") },
    tools: { variant: "list", count: 4, label: ac(t, "loading.tools") },
    workflow: { variant: "detail", count: 4, label: ac(t, "loading.workflow") },
    handoff: { variant: "detail", count: 3, label: ac(t, "loading.handoff") },
    versions: { variant: "table", count: 4, label: ac(t, "loading.versions") },
  } satisfies Record<AIAgentWorkbenchSection, {
    variant: "detail" | "list" | "table"
    count: number
    label: string
  }>
  const meta = loadingMeta[section]

  return (
    <ConfigSection>
      <ModuleLoading variant={meta.variant} count={meta.count} label={meta.label} />
    </ConfigSection>
  )
}

function ConfigSection({
  children,
}: {
  children: ReactNode
}) {
  return (
    <section className="space-y-5">
      <div className="space-y-4">{children}</div>
    </section>
  )
}

function FieldBlock({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
    </div>
  )
}

function AddRow({
  value,
  options,
  placeholder,
  onValueChange,
  onAdd,
}: {
  value: string
  options: { value: string; label: string }[]
  placeholder: string
  onValueChange: (value: string) => void
  onAdd: () => void
}) {
  const t = useI18n()
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1">
        <OptionCombobox value={value} options={options} placeholder={placeholder} onChange={onValueChange} />
      </div>
      <Button type="button" variant="outline" disabled={!value} onClick={onAdd}>
        {ac(t, "actions.add")}
      </Button>
    </div>
  )
}

function BadgeList({
  empty,
  items,
  onRemove,
}: {
  empty: string
  items: { value: string; label: string }[]
  onRemove: (id: number) => void
}) {
  if (items.length === 0) {
    return <div className="text-sm text-muted-foreground">{empty}</div>
  }
  return (
    <div className="flex flex-wrap gap-2">
      {items.map((item) => (
        <Badge key={item.value} variant="secondary" className="gap-1 pr-1">
          {item.label}
          <Button type="button" variant="ghost" size="icon" className="size-5" onClick={() => onRemove(Number(item.value))}>
            <Trash2Icon className="size-3" />
          </Button>
        </Badge>
      ))}
    </div>
  )
}
