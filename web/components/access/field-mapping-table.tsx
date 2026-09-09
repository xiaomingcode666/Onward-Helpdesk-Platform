"use client"

import { useState } from "react"
import { PlusIcon, Trash2Icon, AlertCircleIcon, GripVerticalIcon } from "lucide-react"

import { SelectField } from "@railops/ui"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"
import { useI18n } from "@/i18n/provider"

export interface FieldMapping {
  id: string
  sourceField: string
  sourceExpression: string
  targetField: string
  targetObject: string
  transformType: "direct" | "jsonpath" | "enum" | "formula"
  status: "active" | "inactive" | "error"
}

interface FieldMappingTableProps {
  mappings: FieldMapping[]
  onChange: (mappings: FieldMapping[]) => void
  readonly?: boolean
}

const STANDARD_OBJECTS = [
  "customer",
  "device",
  "warranty",
  "serviceHistory",
  "sparePart",
  "faultCode",
  "contractEntitlement",
  "deviceTelemetry",
  "deviceEvent",
]

const TRANSFORM_TYPES = [
  { value: "direct", label: "Direct" },
  { value: "jsonpath", label: "JSONPath" },
  { value: "enum", label: "Enum Map" },
  { value: "formula", label: "Formula" },
] as const

let mappingIdCounter = 100

function generateMappingId(): string {
  mappingIdCounter += 1
  return `map_${mappingIdCounter}`
}

function isValidExpression(type: string, expr: string): boolean {
  if (type === "jsonpath") {
    return expr.startsWith("$.") || expr.startsWith("$[")
  }
  return expr.trim().length > 0
}

export function FieldMappingTable({
  mappings,
  onChange,
  readonly = false,
}: FieldMappingTableProps) {
  const t = useI18n()
  const [errors, setErrors] = useState<Record<string, string>>({})

  function handleAddRow() {
    const newMapping: FieldMapping = {
      id: generateMappingId(),
      sourceField: "",
      sourceExpression: "$.",
      targetField: "",
      targetObject: "device",
      transformType: "direct",
      status: "active",
    }
    onChange([...mappings, newMapping])
  }

  function handleRemoveRow(id: string) {
    onChange(mappings.filter((m) => m.id !== id))
    const newErrors = { ...errors }
    delete newErrors[id]
    setErrors(newErrors)
  }

  function handleFieldChange(
    id: string,
    field: keyof FieldMapping,
    value: string
  ) {
    const updated = mappings.map((m) =>
      m.id === id ? { ...m, [field]: value } : m
    )
    onChange(updated)

    // Validate
    const mapping = mappings.find((m) => m.id === id)
    if (mapping) {
      const updatedMapping = { ...mapping, [field]: value }
      if (
        field === "sourceExpression" &&
        !isValidExpression(updatedMapping.transformType, value)
      ) {
        setErrors((prev) => ({
          ...prev,
          [id]: "Invalid expression format",
        }))
      } else if (
        field === "transformType" &&
        !isValidExpression(value, mapping.sourceExpression)
      ) {
        setErrors((prev) => ({
          ...prev,
          [id]: "Expression does not match selected type",
        }))
      } else {
        const newErrors = { ...errors }
        delete newErrors[id]
        setErrors(newErrors)
      }
    }
  }

  function getStatusBadgeVariant(status: FieldMapping["status"]) {
    switch (status) {
      case "active":
        return "default" as const
      case "inactive":
        return "secondary" as const
      case "error":
        return "destructive" as const
    }
  }

  return (
    <div className="space-y-4">
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              {!readonly && <TableHead className="w-10" />}
              <TableHead className="min-w-[140px]">
                {t("access.sourceField") || "Source Field"}
              </TableHead>
              <TableHead className="min-w-[200px]">
                {t("access.sourceExpression") || "Expression / JSONPath"}
              </TableHead>
              <TableHead className="min-w-[120px]">
                {t("access.targetObject") || "Target Object"}
              </TableHead>
              <TableHead className="min-w-[140px]">
                {t("access.targetField") || "Target Field"}
              </TableHead>
              <TableHead className="w-[120px]">
                {t("access.transformType") || "Transform"}
              </TableHead>
              <TableHead className="w-[90px]">
                {t("common.status") || "Status"}
              </TableHead>
              {!readonly && <TableHead className="w-12" />}
            </TableRow>
          </TableHeader>
          <TableBody>
            {mappings.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={readonly ? 6 : 8}
                  className="h-24 text-center text-sm text-muted-foreground"
                >
                  {t("access.noMappings") || "No field mappings configured. Add one to start."}
                </TableCell>
              </TableRow>
            )}
            {mappings.map((mapping) => (
              <TableRow key={mapping.id}>
                {!readonly && (
                  <TableCell className="p-2">
                    <GripVerticalIcon className="size-4 cursor-grab text-muted-foreground" />
                  </TableCell>
                )}
                <TableCell>
                  {readonly ? (
                    <span className="text-sm font-medium">
                      {mapping.sourceField}
                    </span>
                  ) : (
                    <Input
                      value={mapping.sourceField}
                      onChange={(e) =>
                        handleFieldChange(mapping.id, "sourceField", e.target.value)
                      }
                      placeholder="e.g. serial_no"
                      className="h-8 text-sm"
                    />
                  )}
                </TableCell>
                <TableCell>
                  {readonly ? (
                    <code className="rounded bg-muted px-1.5 py-0.5 text-xs">
                      {mapping.sourceExpression}
                    </code>
                  ) : (
                    <div className="space-y-1">
                      <Input
                        value={mapping.sourceExpression}
                        onChange={(e) =>
                          handleFieldChange(
                            mapping.id,
                            "sourceExpression",
                            e.target.value
                          )
                        }
                        placeholder="$.data.serialNo"
                        className={`h-8 font-mono text-xs ${
                          errors[mapping.id] ? "border-destructive" : ""
                        }`}
                      />
                      {errors[mapping.id] && (
                        <p className="flex items-center gap-1 text-xs text-destructive">
                          <AlertCircleIcon className="size-3" />
                          {errors[mapping.id]}
                        </p>
                      )}
                    </div>
                  )}
                </TableCell>
                <TableCell>
                  {readonly ? (
                    <Badge variant="outline">{mapping.targetObject}</Badge>
                  ) : (
	                    <SelectField
	                      style={{ marginBottom: 0 }}
	                      selectProps={{
	                        value: mapping.targetObject,
	                        onChange: (v) => {
	                          if (v) handleFieldChange(mapping.id, "targetObject", v)
	                        },
	                        options: STANDARD_OBJECTS.map((obj) => ({ value: obj, label: obj })),
	                        style: { width: "100%" },
	                      }}
	                    />
                  )}
                </TableCell>
                <TableCell>
                  {readonly ? (
                    <span className="text-sm">{mapping.targetField}</span>
                  ) : (
                    <Input
                      value={mapping.targetField}
                      onChange={(e) =>
                        handleFieldChange(mapping.id, "targetField", e.target.value)
                      }
                      placeholder="e.g. serialNo"
                      className="h-8 text-sm"
                    />
                  )}
                </TableCell>
                <TableCell>
                  {readonly ? (
                    <Badge variant="secondary">
                      {mapping.transformType}
                    </Badge>
                  ) : (
                    <SelectField
                      style={{ marginBottom: 0 }}
                      selectProps={{
                        value: mapping.transformType,
                        onChange: (v) =>
                          handleFieldChange(
                            mapping.id,
                            "transformType",
                            v as FieldMapping["transformType"]
                          ),
                        options: TRANSFORM_TYPES.map((type) => ({ value: type.value, label: type.label })),
                        style: { width: "100%" },
                      }}
                    />
                  )}
                </TableCell>
                <TableCell>
                  <Badge variant={getStatusBadgeVariant(mapping.status)}>
                    {mapping.status}
                  </Badge>
                </TableCell>
                {!readonly && (
                  <TableCell className="p-2">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-7 text-muted-foreground hover:text-destructive"
                      onClick={() => handleRemoveRow(mapping.id)}
                    >
                      <Trash2Icon className="size-4" />
                    </Button>
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {!readonly && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={handleAddRow}
        >
          <PlusIcon className="size-4" />
          {t("access.addMapping") || "Add Mapping"}
        </Button>
      )}
    </div>
  )
}
