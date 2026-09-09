import * as React from "react"

import { cn } from "@/lib/utils"

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        "flex field-sizing-content min-h-20 w-full rounded-[6px] border border-[var(--railops-border)] bg-[var(--railops-surface)] px-2.5 py-2 text-xs leading-[18px] text-[var(--railops-text)] transition-colors outline-none placeholder:text-[var(--railops-text-tertiary)] focus-visible:border-[var(--railops-primary)] focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklab,var(--railops-primary)_12%,transparent)] disabled:cursor-not-allowed disabled:bg-[var(--railops-surface-muted)] disabled:opacity-60 aria-invalid:border-[var(--railops-error)] aria-invalid:ring-2 aria-invalid:ring-[color-mix(in_oklab,var(--railops-error)_18%,transparent)]",
        className
      )}
      {...props}
    />
  )
}

export { Textarea }
