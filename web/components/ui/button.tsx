"use client"

import { Button as ButtonPrimitive } from "@base-ui/react/button"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center rounded-[6px] border border-transparent bg-clip-padding text-xs font-semibold whitespace-nowrap transition-colors outline-none select-none focus-visible:border-[var(--railops-primary)] focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklab,var(--railops-primary)_14%,transparent)] disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-[var(--railops-error)] aria-invalid:ring-2 aria-invalid:ring-[color-mix(in_oklab,var(--railops-error)_18%,transparent)] [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: "bg-[var(--railops-primary)] text-white hover:bg-[var(--railops-primary-hover)]",
        outline:
          "border-[var(--railops-border)] bg-[var(--railops-surface)] text-[var(--railops-text)] hover:border-[var(--railops-primary)] hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)] aria-expanded:bg-[var(--railops-primary-bg)] aria-expanded:text-[var(--railops-primary-hover)]",
        secondary:
          "bg-[var(--railops-surface-muted)] text-[var(--railops-text)] hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)] aria-expanded:bg-[var(--railops-primary-bg)] aria-expanded:text-[var(--railops-primary-hover)]",
        ghost:
          "text-[var(--railops-text-secondary)] hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)] aria-expanded:bg-[var(--railops-primary-bg)] aria-expanded:text-[var(--railops-primary-hover)]",
        destructive:
          "bg-[var(--railops-error-bg)] text-[var(--railops-error)] hover:bg-[color-mix(in_oklab,var(--railops-error)_14%,white)] focus-visible:border-[var(--railops-error)] focus-visible:ring-[color-mix(in_oklab,var(--railops-error)_18%,transparent)]",
        link: "h-auto min-h-0 px-0 text-[var(--railops-primary)] underline-offset-4 hover:text-[var(--railops-primary-hover)] hover:underline",
      },
      size: {
        default:
          "h-8 gap-1.5 px-3 has-data-[icon=inline-end]:pr-2.5 has-data-[icon=inline-start]:pl-2.5",
        xs: "h-6 gap-1 rounded-md px-2 text-xs in-data-[slot=button-group]:rounded-md has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-7 gap-1 rounded-md px-2.5 text-xs in-data-[slot=button-group]:rounded-md has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3.5",
        lg: "h-9 gap-1.5 px-4 text-rhd-md has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3",
        icon: "size-8",
        "icon-xs":
          "size-6 rounded-md in-data-[slot=button-group]:rounded-md [&_svg:not([class*='size-'])]:size-3",
        "icon-sm":
          "size-6 rounded-md in-data-[slot=button-group]:rounded-md",
        "icon-lg": "size-10",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  nativeButton,
  render,
  variant = "default",
  size = "default",
  ...props
}: ButtonPrimitive.Props & VariantProps<typeof buttonVariants>) {
  return (
    <ButtonPrimitive
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      nativeButton={nativeButton ?? (render ? false : true)}
      render={render}
      {...props}
    />
  )
}

export { Button, buttonVariants }
