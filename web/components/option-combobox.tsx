"use client"

import { useState, type ReactNode } from "react"
import { CheckIcon, ChevronsUpDownIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { cn } from "@/lib/utils"
import { useI18n } from "@/i18n/provider"

export type ComboboxOption = {
  value: string
  label: string
  icon?: ReactNode
}

type OptionComboboxProps = {
  value: string
  options: ComboboxOption[]
  placeholder: string
  searchPlaceholder?: string
  emptyText?: string
  disabled?: boolean
  searchValue?: string
  onSearchChange?: (value: string) => void
  shouldFilter?: boolean
  onChange: (value: string) => void
  renderOptionAction?: (option: ComboboxOption) => ReactNode
}

export function OptionCombobox({
  value,
  options,
  placeholder,
  searchPlaceholder,
  emptyText,
  disabled = false,
  searchValue,
  onSearchChange,
  shouldFilter,
  onChange,
  renderOptionAction,
}: OptionComboboxProps) {
  const t = useI18n()
  const [open, setOpen] = useState(false)
  const selectedOption = options.find((option) => option.value === value)
  const selectedLabel = selectedOption?.label ?? placeholder

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <Button
            variant="outline"
            role="combobox"
            className="m-0 w-full justify-between font-normal"
            disabled={disabled}
          />
        }
      >
        <span className="flex min-w-0 items-center gap-2">
          {selectedOption?.icon ? (
            <span className="shrink-0 text-muted-foreground">{selectedOption.icon}</span>
          ) : null}
          <span className="truncate">{selectedLabel}</span>
        </span>
        <ChevronsUpDownIcon className="ml-2 size-4 shrink-0 opacity-50" />
      </PopoverTrigger>
      <PopoverContent className="w-(--radix-popover-trigger-width) p-0" align="start">
        <Command shouldFilter={shouldFilter}>
          <CommandInput
            placeholder={searchPlaceholder ?? t("common.searchKeyword")}
            value={searchValue}
            onValueChange={onSearchChange}
          />
          <CommandList>
            <CommandEmpty>{emptyText ?? t("common.emptyOptions")}</CommandEmpty>
            <CommandGroup>
              {options.map((option) => (
                <CommandItem
                  key={option.value}
                  value={`${option.label} ${option.value}`}
                  onSelect={() => {
                    onChange(option.value)
                    setOpen(false)
                  }}
                >
                  <div className="flex min-w-0 flex-1 items-center justify-between gap-2">
                    <div className="flex min-w-0 items-center">
                      <CheckIcon
                        className={cn(
                          "mr-2 size-4 shrink-0",
                          option.value === value ? "opacity-100" : "opacity-0"
                        )}
                      />
                      {option.icon ? (
                        <span className="mr-2 shrink-0 text-muted-foreground">{option.icon}</span>
                      ) : null}
                      <span className="truncate">{option.label}</span>
                    </div>
                    {renderOptionAction ? (
                      <div
                        className="shrink-0"
                        onMouseDown={(event) => event.preventDefault()}
                        onClick={(event) => event.stopPropagation()}
                      >
                        {renderOptionAction(option)}
                      </div>
                    ) : null}
                  </div>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
