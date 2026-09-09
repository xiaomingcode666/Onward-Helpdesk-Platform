import { ChevronRightIcon } from "lucide-react"
import { Fragment, type ReactNode } from "react"

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { cn } from "@/lib/utils"

type HeaderCrumbsProps = {
  children?: ReactNode
  className?: string
  size?: "page" | "sm"
}

export function HeaderCrumbs({
  children,
  className,
  size = "sm",
}: HeaderCrumbsProps) {
  if (children === null || children === undefined || children === false) {
    return null
  }

  if (typeof children !== "string") {
    return <div className={cn("min-w-0", className)}>{children}</div>
  }

  const segments = children
    .split(/\s*\/\s*/)
    .map((segment) => segment.trim())
    .filter(Boolean)
  const visibleSegments = segments.length > 2
    ? [segments[0], segments[segments.length - 1]]
    : segments

  if (segments.length === 0) {
    return null
  }

  const textSizeClass = size === "page" ? "text-rhd-3xl leading-6" : "text-rhd-xl leading-5"
  const separatorIconClass = size === "page" ? "size-4" : "size-3.5"
  const itemMaxWidthClass = size === "page" ? "max-w-[min(22rem,58vw)]" : "max-w-[min(14rem,42vw)]"

  return (
    <Breadcrumb className={cn("min-w-0 overflow-hidden", className)}>
      <BreadcrumbList
        className={cn(
          "flex min-w-0 flex-nowrap items-center gap-1.5 overflow-hidden whitespace-nowrap font-semibold text-muted-foreground",
          textSizeClass
        )}
      >
        {visibleSegments.map((segment, index) => {
          const isCurrent = index === visibleSegments.length - 1

          return (
            <Fragment key={`${segment}-${index}`}>
              <BreadcrumbItem className={cn("min-w-0 shrink", itemMaxWidthClass)}>
                <BreadcrumbPage
                  className={cn(
                    "block truncate",
                    isCurrent ? "font-bold text-foreground" : "font-semibold text-muted-foreground"
                  )}
                >
                  {segment}
                </BreadcrumbPage>
              </BreadcrumbItem>
              {!isCurrent ? (
                <BreadcrumbSeparator className="shrink-0 text-border">
                  <ChevronRightIcon className={separatorIconClass} />
                </BreadcrumbSeparator>
              ) : null}
            </Fragment>
          )
        })}
      </BreadcrumbList>
    </Breadcrumb>
  )
}
