import * as React from "react"
import { Input as InputPrimitive } from "@base-ui/react/input"

import { cn } from "@/lib/utils"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <InputPrimitive
      type={type}
      data-slot="input"
      className={cn(
        "h-8 w-full min-w-0 rounded-[6px] border border-[var(--railops-border)] bg-[var(--railops-surface)] px-2.5 py-1 text-xs text-[var(--railops-text)] transition-colors outline-none file:inline-flex file:h-6 file:border-0 file:bg-transparent file:text-xs file:font-medium file:text-[var(--railops-text)] placeholder:text-[var(--railops-text-tertiary)] focus-visible:border-[var(--railops-primary)] focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklab,var(--railops-primary)_12%,transparent)] disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-[var(--railops-surface-muted)] disabled:opacity-60 aria-invalid:border-[var(--railops-error)] aria-invalid:ring-2 aria-invalid:ring-[color-mix(in_oklab,var(--railops-error)_18%,transparent)]",
        className
      )}
      {...props}
    />
  )
}

export { Input }
