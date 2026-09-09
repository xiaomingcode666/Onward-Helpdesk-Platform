import { mergeProps } from "@base-ui/react/merge-props"
import { useRender } from "@base-ui/react/use-render"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "group/badge inline-flex h-[22px] w-fit shrink-0 items-center justify-center gap-1 overflow-hidden rounded-full border border-transparent px-2 py-0 text-xs font-semibold leading-5 whitespace-nowrap transition-colors focus-visible:border-[var(--railops-primary)] focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklab,var(--railops-primary)_14%,transparent)] has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 aria-invalid:border-[var(--railops-error)] aria-invalid:ring-[color-mix(in_oklab,var(--railops-error)_18%,transparent)] [&>svg]:pointer-events-none [&>svg]:size-3!",
  {
    variants: {
      variant: {
        default: "bg-[var(--railops-primary-bg)] text-[var(--railops-primary-hover)] [a]:hover:bg-[var(--railops-primary-bg)]",
        secondary:
          "bg-[var(--railops-surface-muted)] text-[var(--railops-text-secondary)] [a]:hover:bg-[var(--railops-surface-muted)]",
        destructive:
          "bg-[var(--railops-error-bg)] text-[var(--railops-error)] focus-visible:ring-[color-mix(in_oklab,var(--railops-error)_18%,transparent)] [a]:hover:bg-[var(--railops-error-bg)]",
        outline:
          "border-[var(--railops-border)] bg-[var(--railops-surface)] text-[var(--railops-text-secondary)] [a]:hover:bg-[var(--railops-primary-bg)] [a]:hover:text-[var(--railops-primary-hover)]",
        ghost:
          "text-[var(--railops-text-secondary)] hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)]",
        link: "text-[var(--railops-primary)] underline-offset-4 hover:text-[var(--railops-primary-hover)] hover:underline",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

function Badge({
  className,
  variant = "default",
  render,
  ...props
}: useRender.ComponentProps<"span"> & VariantProps<typeof badgeVariants>) {
  return useRender({
    defaultTagName: "span",
    props: mergeProps<"span">(
      {
        className: cn(badgeVariants({ variant }), className),
      },
      props
    ),
    render,
    state: {
      slot: "badge",
      variant,
    },
  })
}

export { Badge, badgeVariants }
