"use client"

import { cn } from "@/lib/utils"

import {
  CONTENT_MODE_OPTIONS,
  type ContentMode,
} from "./types"

type EditorModeSwitchProps = {
  value: ContentMode
  allowedModes?: ReadonlyArray<ContentMode>
  disabled?: boolean
  onChange: (nextMode: ContentMode) => void
}

const MODE_OPTIONS: Array<{ value: ContentMode; label: string }> = [
  { value: "markdown", label: "Markdown" },
  { value: "html", label: "HTML" },
]

export function EditorModeSwitch({
  value,
  allowedModes = CONTENT_MODE_OPTIONS,
  disabled = false,
  onChange,
}: EditorModeSwitchProps) {
  const options = MODE_OPTIONS.filter((option) => allowedModes.includes(option.value))

  if (options.length <= 1) {
    return null
  }

  return (
    <div className="mx-0.5 rounded-[3px] border border-border bg-muted/40 p-0">
      <div className="flex items-center">
        {options.map((option) => {
          const active = option.value === value
          return (
            <button
              key={option.value}
              type="button"
              disabled={disabled}
              onClick={() => onChange(option.value)}
              className={cn(
                "px-1.5 py-0 text-rhd-sm leading-6 whitespace-nowrap transition-colors",
                "hover:bg-muted disabled:cursor-not-allowed disabled:opacity-60",
                active ? "bg-background text-foreground shadow-sm" : "bg-transparent text-muted-foreground"
              )}
            >
              {option.label}
            </button>
          )
        })}
      </div>
    </div>
  )
}
