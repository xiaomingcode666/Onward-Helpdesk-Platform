"use client"

import { OptionCombobox } from "@/components/option-combobox"

import {
  formatWorkflowVariableName,
  formatWorkflowVariableType,
  workflowExtractText,
  type WorkflowVariableRef,
  type WorkflowVariableSelector,
} from "./workflow-utils"
import { useI18n } from "@/i18n/provider"

export function VariableSelector({
  value,
  variables,
  onChange,
}: {
  value?: WorkflowVariableSelector
  variables: WorkflowVariableRef[]
  onChange: (value: WorkflowVariableSelector) => void
}) {
  const t = useI18n()
  const options = variables.map((item) => ({
    value: `${item.nodeId}.${item.field}`,
    label: `${item.nodeName}.${formatWorkflowVariableName(item.field, t)} · ${formatWorkflowVariableType(item.type, t)}`,
  }))
  const selectedValue = value?.nodeId && value.field ? `${value.nodeId}.${value.field}` : ""

  return (
    <OptionCombobox
      value={selectedValue}
      options={options}
      placeholder={workflowExtractText(t, "editor.variableSelector.placeholder", "Select variable")}
      searchPlaceholder={workflowExtractText(t, "editor.variableSelector.searchPlaceholder", "Search variables")}
      emptyText={workflowExtractText(t, "editor.variableSelector.emptyText", "No upstream variables available")}
      onChange={(nextValue) => {
        const [nodeId, ...fieldParts] = nextValue.split(".")
        onChange({ nodeId, field: fieldParts.join(".") })
      }}
    />
  )
}
