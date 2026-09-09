"use client"

import { Checkbox as CheckboxPrimitive } from "@base-ui/react/checkbox"

import { cn } from "@/lib/utils"
import { CheckIcon } from "lucide-react"

function Checkbox({ className, ...props }: CheckboxPrimitive.Root.Props) {
  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      className={cn(
        "peer relative flex size-4 shrink-0 items-center justify-center rounded-[4px] border border-[var(--railops-border)] bg-[var(--railops-surface)] text-white transition-colors outline-none group-has-disabled/field:opacity-60 after:absolute after:-inset-x-3 after:-inset-y-2 focus-visible:border-[var(--railops-primary)] focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklab,var(--railops-primary)_14%,transparent)] disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-[var(--railops-error)] aria-invalid:ring-2 aria-invalid:ring-[color-mix(in_oklab,var(--railops-error)_18%,transparent)] aria-invalid:aria-checked:border-[var(--railops-primary)] data-checked:border-[var(--railops-primary)] data-checked:bg-[var(--railops-primary)] data-checked:text-white",
        className
      )}
      {...props}
    >
      <CheckboxPrimitive.Indicator
        data-slot="checkbox-indicator"
        className="grid place-content-center text-current transition-none [&>svg]:size-3.5"
      >
        <CheckIcon
        />
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  )
}

export { Checkbox }
