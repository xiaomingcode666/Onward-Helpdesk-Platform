"use client"

import { useState } from "react"
import type { Node } from "@xyflow/react"

import { useI18n } from "@/i18n/provider"
import { Checkbox as AntCheckbox } from "antd"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { OptionCombobox } from "@/components/option-combobox"
import { VariableSelector } from "./variable-selector"
import {
  formatWorkflowVariableName,
  formatWorkflowVariableType,
  workflowExtractText,
  type WorkflowConditionBranch,
  type WorkflowNodeSpec,
  type WorkflowNodeConfig,
  type WorkflowVariableRef,
  type WorkflowVariableSpec,
  type WorkflowVariableSelector,
} from "./workflow-utils"

type WorkflowNodeData = Record<string, unknown> & {
  nodeType?: string
  name?: string
  title?: string
  config?: WorkflowNodeConfig
  inputs?: Record<string, WorkflowVariableSelector>
  errorTargetNodeId?: string
}

export type WorkflowBranchSummary = {
  branchId: string
  targetNodeId: string
  targetName: string
  conditionLabel: string
  isDefault: boolean
}

export type WorkflowBranchTargetOption = {
  value: string
  label: string
}

export function NodeConfigPanel({
  node,
  nodeSpec,
  availableVariables,
  branchSummaries = [],
  branchTargetOptions = [],
  onChange,
}: {
  node: Node<WorkflowNodeData> | null
  nodeSpec?: WorkflowNodeSpec
  availableVariables: WorkflowVariableRef[]
  branchSummaries?: WorkflowBranchSummary[]
  branchTargetOptions?: WorkflowBranchTargetOption[]
  onChange: (nodeId: string, data: WorkflowNodeData) => void
}) {
  const t = useI18n()
  if (!node) {
    return (
      <div className="flex h-full items-center justify-center px-4 text-sm text-muted-foreground">
        {workflowExtractText(t, "editor.nodeConfig.empty", "Select a node to configure input mappings and review output variables.")}
      </div>
    )
  }

  return (
    <NodeConfigForm
      key={node.id}
      node={node}
      nodeSpec={nodeSpec}
      availableVariables={availableVariables}
      branchSummaries={branchSummaries}
      branchTargetOptions={branchTargetOptions}
      onChange={onChange}
    />
  )
}

function NodeConfigForm({
  node,
  nodeSpec,
  availableVariables,
  branchSummaries,
  branchTargetOptions,
  onChange,
}: {
  node: Node<WorkflowNodeData>
  nodeSpec?: WorkflowNodeSpec
  availableVariables: WorkflowVariableRef[]
  branchSummaries: WorkflowBranchSummary[]
  branchTargetOptions: WorkflowBranchTargetOption[]
  onChange: (nodeId: string, data: WorkflowNodeData) => void
}) {
  const t = useI18n()
  const [name, setName] = useState(node.data.name ?? "")
  const [configText, setConfigText] = useState(JSON.stringify(node.data.config ?? {}, null, 2))
  const [inputs, setInputs] = useState<Record<string, WorkflowVariableSelector>>(
    node.data.inputs ?? {}
  )
  const [error, setError] = useState("")
  const inputSchema = nodeSpec?.inputSchema ?? []
  const outputSchema = nodeSpec?.outputSchema ?? []
  const isConditionNode = node.data.nodeType === "condition"
  const supportsErrorTarget = !["start", "condition", "end"].includes(node.data.nodeType ?? "")
  const fallbackNodeName = nodeSpec?.title || node.data.title || node.data.nodeType || node.id
  const panelTitle = name.trim() || node.data.name?.trim() || fallbackNodeName

  const commitChange = (next: Partial<WorkflowNodeData>) => {
    onChange(node.id, {
      ...node.data,
      name: name.trim() || fallbackNodeName,
      config: node.data.config ?? {},
      inputs,
      ...next,
    })
  }

  const commitConfig = (patch: Partial<WorkflowNodeConfig>) => {
    const nextConfig = { ...(node.data.config ?? {}), ...patch }
    setConfigText(JSON.stringify(nextConfig, null, 2))
    commitChange({ config: nextConfig })
  }

  const handleApply = () => {
    try {
      const parsed = JSON.parse(configText || "{}") as Record<string, unknown>
      setError("")
      commitChange({ config: parsed })
    } catch {
      setError(workflowExtractText(t, "editor.nodeConfig.advancedJsonInvalid", "Advanced config must be valid JSON."))
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-4 p-4">
      <div>
        <div className="text-sm font-medium">{panelTitle}</div>
        <div className="mt-1 text-xs text-muted-foreground">
          {node.data.nodeType && node.data.nodeType !== panelTitle
            ? `${node.id} · ${node.data.nodeType}`
            : node.id}
        </div>
      </div>
      <div className="space-y-2">
        <Label htmlFor="workflow-node-name">{workflowExtractText(t, "editor.nodeConfig.nodeName", "Node name")}</Label>
        <Input
          id="workflow-node-name"
          value={name}
          onChange={(event) => setName(event.target.value)}
          onBlur={() => commitChange({ name: name.trim() || node.data.nodeType || node.id })}
        />
      </div>
      {isConditionNode ? (
        <ConditionNodePanel
          branches={node.data.config?.branches ?? []}
          branchSummaries={branchSummaries}
          branchTargetOptions={branchTargetOptions}
          availableVariables={availableVariables}
          outputSchema={outputSchema}
          onChange={(branches) => commitChange({ config: { ...(node.data.config ?? {}), branches } })}
        />
      ) : (
        <>
          {inputSchema.length > 0 ? (
            <div className="space-y-3">
              <div className="text-sm font-medium">{workflowExtractText(t, "editor.nodeConfig.inputMapping", "Input mapping")}</div>
              {availableVariables.length === 0 ? (
                <div className="rounded-md border border-dashed p-2 text-xs text-muted-foreground">
                  {workflowExtractText(t, "editor.nodeConfig.noVariables", "No variables available")}
                </div>
              ) : null}
              {inputSchema.map((input) => (
                <div key={input.name} className="space-y-1.5">
                  <div className="flex items-center justify-between gap-2">
                    <Label className="text-xs">
                      {formatWorkflowVariableName(input.name, t)}
                      {input.required ? <span className="text-destructive"> *</span> : null}
                    </Label>
                    <span className="text-xs text-muted-foreground">{formatWorkflowVariableType(input.type, t)}</span>
                  </div>
                  <VariableSelector
                    value={inputs[input.name]}
                    variables={availableVariables}
                    onChange={(value) => {
                      const nextInputs = {
                        ...inputs,
                        [input.name]: value,
                      }
                      setInputs(nextInputs)
                      commitChange({
                        inputs: nextInputs,
                      })
                    }}
                  />
                  {inputs[input.name] ? (
                    <div className="text-xs text-muted-foreground">
                      {workflowExtractText(t, "editor.nodeConfig.selectedVariable", "Selected: {node}.{field}", {
                        node: inputs[input.name].nodeId,
                        field: formatWorkflowVariableName(inputs[input.name].field, t),
                      })}
                    </div>
                  ) : null}
                  {input.description ? (
                    <div className="text-xs text-muted-foreground">{input.description}</div>
                  ) : null}
                </div>
              ))}
            </div>
          ) : null}
          <RuntimeNodeConfigFields
            nodeId={node.id}
            nodeType={node.data.nodeType ?? ""}
            config={node.data.config ?? {}}
            availableVariables={availableVariables}
            onChange={commitConfig}
          />
          {supportsErrorTarget ? (
            <div className="space-y-1.5 border-y py-3">
              <Label>{workflowExtractText(t, "editor.nodeConfig.errorTarget", "On failure go to")}</Label>
              <OptionCombobox
                value={node.data.errorTargetNodeId || "__stop__"}
                options={[
                  { value: "__stop__", label: workflowExtractText(t, "editor.nodeConfig.stopOnError", "Stop the flow and record the error") },
                  ...branchTargetOptions,
                ]}
                placeholder={workflowExtractText(t, "editor.nodeConfig.errorTargetPlaceholder", "Select failure exit")}
                searchPlaceholder={workflowExtractText(t, "editor.nodeConfig.searchTargetNode", "Search target nodes")}
                emptyText={workflowExtractText(t, "editor.nodeConfig.noTargetNodes", "No target nodes available")}
                onChange={(value) => commitChange({
                  errorTargetNodeId: value === "__stop__" ? undefined : value,
                })}
              />
              <p className="text-xs leading-5 text-muted-foreground">
                {workflowExtractText(t, "editor.nodeConfig.errorTargetHelp", "After choosing a fallback node, connect the current node to it. Normal execution will not use that edge.")}
              </p>
            </div>
          ) : null}
          <details className="rounded-md border bg-background p-3">
            <summary className="cursor-pointer text-sm font-medium">{workflowExtractText(t, "editor.nodeConfig.advancedJson", "Advanced config JSON")}</summary>
            <div className="mt-3 space-y-2">
              <Textarea
                id="workflow-node-config"
                className="h-40 font-mono text-xs"
                value={configText}
                onChange={(event) => setConfigText(event.target.value)}
              />
              {error ? <div className="text-xs text-destructive">{error}</div> : null}
              <Button type="button" variant="outline" size="sm" onClick={handleApply}>
                {workflowExtractText(t, "editor.nodeConfig.saveAdvanced", "Save advanced config")}
              </Button>
            </div>
          </details>
          {outputSchema.length > 0 ? (
            <div className="space-y-2">
              <div className="text-sm font-medium">{workflowExtractText(t, "editor.nodeConfig.outputVariables", "Output variables")}</div>
              <div className="space-y-1 rounded-md border bg-background p-2">
                {outputSchema.map((output) => (
                  <div key={output.name} className="space-y-0.5 rounded-sm px-1 py-0.5">
                    <div className="flex items-center justify-between gap-2 text-xs">
                      <span className="truncate font-medium">{formatWorkflowVariableName(output.name, t)}</span>
                      <span className="shrink-0 text-muted-foreground">{formatWorkflowVariableType(output.type, t)}</span>
                    </div>
                    {output.description ? (
                      <div className="text-xs text-muted-foreground">{output.description}</div>
                    ) : null}
                  </div>
                ))}
              </div>
            </div>
          ) : null}
        </>
      )}
    </div>
  )
}

function RuntimeNodeConfigFields({
  nodeId,
  nodeType,
  config,
  availableVariables,
  onChange,
}: {
  nodeId: string
  nodeType: string
  config: WorkflowNodeConfig
  availableVariables: WorkflowVariableRef[]
  onChange: (patch: Partial<WorkflowNodeConfig>) => void
}) {
  const t = useI18n()
  if (nodeType === "entry_context") {
    return (
      <div className="space-y-1.5">
        <Label>{workflowExtractText(t, "editor.nodeConfig.entryUnboundMode", "When device is not recognized")}</Label>
        <OptionCombobox
          value={config.unboundMode ?? "quick_ai"}
          options={[
            { value: "quick_ai", label: workflowExtractText(t, "editor.nodeConfig.quickAi", "Go directly to Q&A") },
            { value: "product_context", label: workflowExtractText(t, "editor.nodeConfig.productContextFirst", "Collect product or device details first") },
          ]}
          placeholder={workflowExtractText(t, "editor.nodeConfig.entryModePlaceholder", "Select entry handling mode")}
          onChange={(unboundMode) => onChange({ unboundMode: unboundMode as "quick_ai" | "product_context" })}
        />
      </div>
    )
  }

  if (nodeType === "service_access_policy") {
    const accessOptions = [
      { value: "device_only", label: workflowExtractText(t, "editor.nodeConfig.accessDeviceOnly", "Only open to bound devices") },
      { value: "inherit", label: workflowExtractText(t, "editor.nodeConfig.accessInherit", "Follow reception config service mode") },
      { value: "always", label: workflowExtractText(t, "editor.nodeConfig.accessAlways", "Open to all entries") },
      { value: "never", label: workflowExtractText(t, "editor.nodeConfig.accessNever", "Always closed") },
    ]
    return (
      <div className="space-y-3">
        <div className="space-y-1.5">
          <Label>{workflowExtractText(t, "editor.nodeConfig.handoffPermission", "Human handoff permission")}</Label>
          <OptionCombobox
            value={config.humanHandoffMode ?? "device_only"}
            options={accessOptions}
            placeholder={workflowExtractText(t, "editor.nodeConfig.handoffRulePlaceholder", "Select human handoff rule")}
            onChange={(humanHandoffMode) => onChange({ humanHandoffMode: humanHandoffMode as WorkflowNodeConfig["humanHandoffMode"] })}
          />
        </div>
        <div className="space-y-1.5">
          <Label>{workflowExtractText(t, "editor.nodeConfig.ticketPermission", "Ticket creation permission")}</Label>
          <OptionCombobox
            value={config.ticketAccessMode ?? "device_only"}
            options={accessOptions}
            placeholder={workflowExtractText(t, "editor.nodeConfig.ticketRulePlaceholder", "Select ticket rule")}
            onChange={(ticketAccessMode) => onChange({ ticketAccessMode: ticketAccessMode as WorkflowNodeConfig["ticketAccessMode"] })}
          />
        </div>
        <p className="text-xs leading-5 text-muted-foreground">{workflowExtractText(t, "editor.nodeConfig.accessPolicyHelp", "Access rules still depend on the flow's actual capabilities. If human handoff or ticket nodes are not arranged, no fake entry will be shown.")}</p>
      </div>
    )
  }

  if (nodeType === "knowledge_retrieve") {
    return (
      <div className="space-y-3">
        <div className="space-y-1.5">
          <Label>{workflowExtractText(t, "editor.nodeConfig.knowledgeBindingMethod", "Knowledge binding method")}</Label>
          <OptionCombobox
            value={config.bindingMethod ?? config.scope ?? "public_product"}
            options={knowledgeBindingMethodOptions(t)}
            placeholder={workflowExtractText(t, "editor.nodeConfig.knowledgeBindingPlaceholder", "Select knowledge binding method")}
            searchPlaceholder={workflowExtractText(t, "editor.nodeConfig.searchKnowledgeBinding", "Search knowledge binding methods")}
            emptyText={workflowExtractText(t, "editor.nodeConfig.noKnowledgeBinding", "No knowledge binding methods available")}
            onChange={(bindingMethod) => onChange({ bindingMethod })}
          />
        </div>
        <div className="grid grid-cols-2 gap-2">
          <div className="space-y-1.5">
            <Label htmlFor="workflow-knowledge-top-k">{workflowExtractText(t, "editor.nodeConfig.topK", "Recall count")}</Label>
            <Input
              id="workflow-knowledge-top-k"
              type="number"
              min={1}
              max={50}
              value={config.topK ?? 8}
              onChange={(event) => onChange({ topK: clampInteger(event.target.value, 1, 50) })}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="workflow-knowledge-score">{workflowExtractText(t, "editor.nodeConfig.scoreThreshold", "Minimum score")}</Label>
            <Input
              id="workflow-knowledge-score"
              type="number"
              min={0}
              max={1}
              step={0.05}
              value={config.scoreThreshold ?? 0.3}
              onChange={(event) => onChange({ scoreThreshold: clampNumber(event.target.value, 0, 1) })}
            />
          </div>
        </div>
      </div>
    )
  }

  if (nodeType === "knowledge_merge") {
    const sources = config.sources ?? []
    const candidates = availableVariables.filter((item) => item.field === "items" && item.type === "array<object>")
    return (
      <div className="space-y-3">
        <div className="space-y-2">
          <Label>{workflowExtractText(t, "editor.nodeConfig.mergeSources", "Merge sources")}</Label>
          {candidates.map((candidate) => {
            const checked = sources.some((source) => source.nodeId === candidate.nodeId && source.field === candidate.field)
            return (
              <label key={`${candidate.nodeId}.${candidate.field}`} className="flex items-center gap-2 text-sm">
                <AntCheckbox
                  checked={checked}
                  onChange={(event) => onChange({
                    sources: event.target.checked
                      ? [...sources, { nodeId: candidate.nodeId, field: candidate.field }]
                      : sources.filter((source) => source.nodeId !== candidate.nodeId || source.field !== candidate.field),
                  })}
                />
                <span className="min-w-0 truncate">{candidate.nodeName}.{formatWorkflowVariableName(candidate.field, t)}</span>
              </label>
            )
          })}
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="workflow-knowledge-max-items">{workflowExtractText(t, "editor.nodeConfig.maxItems", "Maximum items")}</Label>
          <Input
            id="workflow-knowledge-max-items"
            type="number"
            min={1}
            max={50}
            value={config.maxItems ?? 10}
            onChange={(event) => onChange({ maxItems: clampInteger(event.target.value, 1, 50) })}
          />
        </div>
      </div>
    )
  }

  if (nodeType === "answerability_gate") {
    return (
      <div className="space-y-3">
        <div className="grid grid-cols-2 gap-2">
          <div className="space-y-1.5">
            <Label htmlFor="workflow-answerability-min-score">{workflowExtractText(t, "editor.nodeConfig.minRelevanceScore", "Minimum relevance score")}</Label>
            <Input
              id="workflow-answerability-min-score"
              type="number"
              min={0}
              max={1}
              step={0.05}
              value={config.minScore ?? 0.35}
              onChange={(event) => onChange({ minScore: clampNumber(event.target.value, 0, 1) })}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="workflow-answerability-strong-score">{workflowExtractText(t, "editor.nodeConfig.strongRelevanceScore", "Strong relevance score")}</Label>
            <Input
              id="workflow-answerability-strong-score"
              type="number"
              min={0}
              max={1}
              step={0.05}
              value={config.strongScore ?? 0.72}
              onChange={(event) => onChange({ strongScore: clampNumber(event.target.value, 0, 1) })}
            />
          </div>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="workflow-answerability-matched-terms">{workflowExtractText(t, "editor.nodeConfig.minMatchedTerms", "Minimum matched terms")}</Label>
          <Input
            id="workflow-answerability-matched-terms"
            type="number"
            min={1}
            max={20}
            value={config.minMatchedTerms ?? 1}
            onChange={(event) => onChange({ minMatchedTerms: clampInteger(event.target.value, 1, 20) })}
          />
        </div>
      </div>
    )
  }

  if (nodeType === "subflow") {
    return <WorkflowVersionField value={config.workflowVersionId} onChange={(workflowVersionId) => onChange({ workflowVersionId })} />
  }

  if (nodeType === "loop") {
    const until = config.until ?? {
      left: { nodeId, field: "iteration" },
      operator: "gte",
      right: config.maxIterations ?? 3,
    }
    return (
      <div className="space-y-3">
        <WorkflowVersionField value={config.workflowVersionId} onChange={(workflowVersionId) => onChange({ workflowVersionId })} />
        <div className="grid grid-cols-2 gap-2">
          <div className="space-y-1.5">
            <Label htmlFor="workflow-loop-max">{workflowExtractText(t, "editor.nodeConfig.maxIterations", "Maximum iterations")}</Label>
            <Input
              id="workflow-loop-max"
              type="number"
              min={1}
              max={10}
              value={config.maxIterations ?? 3}
              onChange={(event) => onChange({ maxIterations: clampInteger(event.target.value, 1, 10) })}
            />
          </div>
          <div className="space-y-1.5">
            <Label>{workflowExtractText(t, "editor.nodeConfig.failurePolicy", "Failure policy")}</Label>
            <OptionCombobox
              value={config.failurePolicy ?? "fail"}
              options={[
                { value: "fail", label: workflowExtractText(t, "editor.nodeConfig.failureStop", "Stop") },
                { value: "continue", label: workflowExtractText(t, "editor.nodeConfig.failureContinue", "Continue") },
              ]}
              placeholder={workflowExtractText(t, "editor.nodeConfig.failurePolicyPlaceholder", "Select failure policy")}
              onChange={(value) => onChange({ failurePolicy: value as "fail" | "continue" })}
            />
          </div>
        </div>
        <div className="grid grid-cols-[1fr_1fr] gap-2">
          <div className="space-y-1.5">
            <Label>{workflowExtractText(t, "editor.nodeConfig.exitField", "Exit field")}</Label>
            <OptionCombobox
              value={until.left?.field ?? "iteration"}
              options={[
                { value: "iteration", label: formatWorkflowVariableName("iteration", t) },
                { value: "status", label: formatWorkflowVariableName("status", t) },
                { value: "replyText", label: formatWorkflowVariableName("replyText", t) },
              ]}
              placeholder={workflowExtractText(t, "editor.nodeConfig.exitFieldPlaceholder", "Select exit field")}
              onChange={(field) => onChange({ until: { ...until, left: { nodeId, field } } })}
            />
          </div>
          <div className="space-y-1.5">
            <Label>{workflowExtractText(t, "editor.nodeConfig.conditionOperator", "Condition operator")}</Label>
            <OptionCombobox
              value={until.operator ?? "gte"}
              options={conditionOperatorOptions(t)}
              placeholder={workflowExtractText(t, "editor.nodeConfig.conditionOperatorPlaceholder", "Select condition operator")}
              onChange={(operator) => onChange({ until: { ...until, operator } })}
            />
          </div>
        </div>
        {!conditionOperatorWithoutRight(until.operator ?? "gte") ? (
          <div className="space-y-1.5">
            <Label htmlFor="workflow-loop-right">{workflowExtractText(t, "editor.nodeConfig.compareValue", "Compare value")}</Label>
            <Input
              id="workflow-loop-right"
              value={until.right === undefined ? "" : String(until.right)}
              onChange={(event) => onChange({ until: { ...until, right: normalizeConditionRight(event.target.value) } })}
            />
          </div>
        ) : null}
      </div>
    )
  }

  return null
}

function WorkflowVersionField({ value, onChange }: { value?: number; onChange: (value: number) => void }) {
  const t = useI18n()
  return (
    <div className="space-y-1.5">
      <Label htmlFor="workflow-version-id">{workflowExtractText(t, "editor.nodeConfig.workflowVersionId", "Published version ID")}</Label>
      <Input
        id="workflow-version-id"
        type="number"
        min={1}
        value={value ?? ""}
        onChange={(event) => onChange(Math.max(0, Number.parseInt(event.target.value, 10) || 0))}
      />
    </div>
  )
}

function clampInteger(value: string, min: number, max: number) {
  return Math.min(max, Math.max(min, Number.parseInt(value, 10) || min))
}

function clampNumber(value: string, min: number, max: number) {
  return Math.min(max, Math.max(min, Number.parseFloat(value) || min))
}

function knowledgeBindingMethodOptions(t: ReturnType<typeof useI18n>) {
  return [
    { value: "public_product", label: workflowExtractText(t, "editor.nodeConfig.knowledgePublicProduct", "Product public knowledge") },
    { value: "product_context", label: workflowExtractText(t, "editor.nodeConfig.knowledgeProductContext", "Product context knowledge") },
    { value: "agent_default", label: workflowExtractText(t, "editor.nodeConfig.knowledgeAgentDefault", "Reception config default knowledge") },
    { value: "internal_product", label: workflowExtractText(t, "editor.nodeConfig.knowledgeInternalProduct", "Product internal knowledge") },
  ]
}

function ConditionNodePanel({
  branches,
  branchSummaries,
  branchTargetOptions,
  availableVariables,
  outputSchema,
  onChange,
}: {
  branches: WorkflowConditionBranch[]
  branchSummaries: WorkflowBranchSummary[]
  branchTargetOptions: WorkflowBranchTargetOption[]
  availableVariables: WorkflowVariableRef[]
  outputSchema: WorkflowVariableSpec[]
  onChange: (branches: WorkflowConditionBranch[]) => void
}) {
  const t = useI18n()
  const summariesByBranchID = new Map(branchSummaries.map((item) => [item.branchId, item]))
  const commitBranch = (branchId: string, patch: Partial<WorkflowConditionBranch>) => {
    onChange(branches.map((branch) => (
      branch.id === branchId ? normalizeBranch({ ...branch, ...patch }) : branch
    )))
  }
  const addBranch = () => {
    const index = branches.length + 1
    onChange([
      ...branches,
      {
        id: `branch_${index}`,
        name: workflowExtractText(t, "editor.nodeConfig.branchNameDefault", "Branch {index}", { index }),
        targetNodeId: branchTargetOptions[0]?.value ?? "",
        condition: { operator: "eq" },
      },
    ])
  }
  const deleteBranch = (branchId: string) => {
    onChange(branches.filter((branch) => branch.id !== branchId))
  }
  const markDefault = (branchId: string) => {
    onChange(branches.map((branch) => normalizeBranch({
      ...branch,
      default: branch.id === branchId,
      condition: branch.id === branchId ? undefined : branch.condition ?? { operator: "eq" },
    })))
  }

  return (
    <>
      <div className="space-y-2">
        <div className="flex items-center justify-between gap-2">
          <div className="text-sm font-medium">{workflowExtractText(t, "editor.nodeConfig.branches", "Branches")}</div>
          <Button type="button" variant="outline" size="sm" onClick={addBranch}>
            {workflowExtractText(t, "editor.nodeConfig.addBranch", "Add branch")}
          </Button>
        </div>
        {branches.length > 0 ? (
          <div className="space-y-2">
            {branches.map((branch, index) => {
              const summary = summariesByBranchID.get(branch.id)
              const condition = branch.condition ?? {}
              const conditionRight = condition.right === undefined || condition.right === null
                ? ""
                : String(condition.right)
              return (
                <div key={branch.id} className="space-y-3 rounded-md border bg-background p-3">
                  <div className="flex items-center justify-between gap-2 text-xs">
                    <span className="min-w-0 truncate font-medium">
                      {branch.default
                        ? workflowExtractText(t, "editor.nodeConfig.elseLabel", "Else")
                        : index === 0
                          ? workflowExtractText(t, "editor.nodeConfig.ifLabel", "If")
                          : workflowExtractText(t, "editor.nodeConfig.elseIfLabel", "Else if")}
                    </span>
                    <span className="shrink-0 rounded-sm bg-muted px-1.5 py-0.5 text-muted-foreground">
                      {branch.default
                        ? workflowExtractText(t, "editor.nodeConfig.defaultBranch", "Default")
                        : workflowExtractText(t, "editor.nodeConfig.conditionBranch", "Condition")}
                    </span>
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs">{workflowExtractText(t, "editor.nodeConfig.branchName", "Branch name")}</Label>
                    <Input
                      value={branch.name ?? ""}
                      onChange={(event) => commitBranch(branch.id, { name: event.target.value })}
                      placeholder={workflowExtractText(t, "editor.nodeConfig.branchNamePlaceholder", "For example: Needs human handoff")}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs">{workflowExtractText(t, "editor.nodeConfig.targetNode", "Target node")}</Label>
                    <OptionCombobox
                      value={branch.targetNodeId}
                      options={branchTargetOptions}
                      placeholder={workflowExtractText(t, "editor.nodeConfig.targetNodePlaceholder", "Select target node")}
                      searchPlaceholder={workflowExtractText(t, "editor.nodeConfig.searchTargetNode", "Search target nodes")}
                      emptyText={workflowExtractText(t, "editor.nodeConfig.connectDownstreamFirst", "Connect downstream nodes from the condition node first")}
                      onChange={(value) => commitBranch(branch.id, { targetNodeId: value })}
                    />
                  </div>
                  {branch.default ? (
                    <div className="rounded-md border border-dashed p-2 text-xs text-muted-foreground">
                      {workflowExtractText(t, "editor.nodeConfig.defaultTargetHelp", "When no condition above matches, go to: {target}", {
                        target: summary?.targetName ?? (branch.targetNodeId || workflowExtractText(t, "editor.nodeConfig.noTargetSelected", "No target selected")),
                      })}
                    </div>
                  ) : (
                    <div className="space-y-3 rounded-md border bg-muted/20 p-2">
                      <div className="space-y-1.5">
                        <Label className="text-xs">{workflowExtractText(t, "editor.nodeConfig.conditionVariable", "Condition variable")}</Label>
                        <VariableSelector
                          value={condition.left}
                          variables={availableVariables}
                          onChange={(value) => commitBranch(branch.id, {
                            condition: { ...condition, left: value },
                          })}
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label className="text-xs">{workflowExtractText(t, "editor.nodeConfig.conditionOperator", "Condition operator")}</Label>
                        <OptionCombobox
                          value={condition.operator ?? "eq"}
                          options={conditionOperatorOptions(t)}
                          placeholder={workflowExtractText(t, "editor.nodeConfig.conditionOperatorPlaceholder", "Select condition operator")}
                          searchPlaceholder={workflowExtractText(t, "editor.nodeConfig.searchConditionOperator", "Search condition operators")}
                          emptyText={workflowExtractText(t, "editor.nodeConfig.noConditionOperators", "No condition operators available")}
                          onChange={(value) => commitBranch(branch.id, {
                            condition: { ...condition, operator: value },
                          })}
                        />
                      </div>
                      {!conditionOperatorWithoutRight(condition.operator ?? "eq") ? (
                        <div className="space-y-1.5">
                          <Label className="text-xs">{workflowExtractText(t, "editor.nodeConfig.compareValue", "Compare value")}</Label>
                          <Input
                            value={conditionRight}
                            onChange={(event) => commitBranch(branch.id, {
                              condition: {
                                ...condition,
                                right: normalizeConditionRight(event.target.value),
                              },
                            })}
                            placeholder={workflowExtractText(t, "editor.nodeConfig.compareValuePlaceholder", "Enter compare value")}
                          />
                        </div>
                      ) : null}
                    </div>
                  )}
                  <div className="flex flex-wrap gap-2">
                    {!branch.default ? (
                      <Button type="button" size="sm" variant="outline" onClick={() => markDefault(branch.id)}>
                        {workflowExtractText(t, "editor.nodeConfig.markDefault", "Set as default")}
                      </Button>
                    ) : null}
                    <Button type="button" size="sm" variant="outline" onClick={() => deleteBranch(branch.id)}>
                      {workflowExtractText(t, "editor.nodeConfig.delete", "Delete")}
                    </Button>
                  </div>
                  <div className="line-clamp-2 text-xs text-muted-foreground">
                    {summary?.conditionLabel ?? workflowExtractText(t, "editor.nodeConfig.branchIncomplete", "Branch configuration is not complete")}
                  </div>
                </div>
              )
            })}
          </div>
        ) : (
          <div className="rounded-md border border-dashed p-2 text-xs text-muted-foreground">
            {workflowExtractText(t, "editor.nodeConfig.noBranches", "No branches yet")}
          </div>
        )}
      </div>
      {outputSchema.length > 0 ? (
        <div className="space-y-2">
          <div className="text-sm font-medium">{workflowExtractText(t, "editor.nodeConfig.outputVariables", "Output variables")}</div>
          <div className="space-y-1 rounded-md border bg-background p-2">
            {outputSchema.map((output) => (
              <div key={output.name} className="space-y-0.5 rounded-sm px-1 py-0.5">
                <div className="flex items-center justify-between gap-2 text-xs">
                  <span className="truncate font-medium">{formatWorkflowVariableName(output.name, t)}</span>
                  <span className="shrink-0 text-muted-foreground">{formatWorkflowVariableType(output.type, t)}</span>
                </div>
                {output.description ? (
                  <div className="text-xs text-muted-foreground">{output.description}</div>
                ) : null}
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </>
  )
}

function conditionOperatorOptions(t: ReturnType<typeof useI18n>) {
  return [
    { value: "eq", label: workflowExtractText(t, "editor.conditionOperators.eq", "Equals") },
    { value: "neq", label: workflowExtractText(t, "editor.conditionOperators.neq", "Does not equal") },
    { value: "contains", label: workflowExtractText(t, "editor.conditionOperators.contains", "Contains") },
    { value: "exists", label: workflowExtractText(t, "editor.conditionOperators.exists", "Exists") },
    { value: "not_exists", label: workflowExtractText(t, "editor.conditionOperators.notExists", "Does not exist") },
    { value: "truthy", label: workflowExtractText(t, "editor.conditionOperators.truthy", "Is true") },
    { value: "falsy", label: workflowExtractText(t, "editor.conditionOperators.falsy", "Is false") },
    { value: "gt", label: workflowExtractText(t, "editor.conditionOperators.gt", "Greater than") },
    { value: "gte", label: workflowExtractText(t, "editor.conditionOperators.gte", "Greater than or equal to") },
    { value: "lt", label: workflowExtractText(t, "editor.conditionOperators.lt", "Less than") },
    { value: "lte", label: workflowExtractText(t, "editor.conditionOperators.lte", "Less than or equal to") },
  ]
}

function conditionOperatorWithoutRight(operator: string) {
  return ["exists", "not_exists", "truthy", "falsy"].includes(operator)
}

function normalizeConditionRight(value: string) {
  const trimmed = value.trim()
  if (trimmed === "true") return true
  if (trimmed === "false") return false
  if (trimmed !== "" && !Number.isNaN(Number(trimmed))) return Number(trimmed)
  return trimmed
}

function normalizeBranch(branch: WorkflowConditionBranch): WorkflowConditionBranch {
  if (branch.default) {
    const normalized = { ...branch }
    delete normalized.condition
    return normalized
  }
  return {
    ...branch,
    condition: branch.condition ?? { operator: "eq" },
  }
}
