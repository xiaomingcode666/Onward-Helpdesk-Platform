import Image from "next/image"

import { translateCurrentMessage } from "@/i18n/messages"
import { cn } from "@/lib/utils"

type AppLogoMarkProps = {
  alt?: string
  className?: string
  imageClassName?: string
  priority?: boolean
}

export function AppLogoMark({
  alt = translateCurrentMessage("miscTailExtract.uiShared.brandAlt"),
  className,
  imageClassName,
  priority = false,
}: AppLogoMarkProps) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center justify-center overflow-hidden rounded-md bg-white shadow-sm ring-1 ring-slate-200/80",
        className
      )}
    >
      <Image
        src="/images/logo-mark.webp"
        alt={alt}
        width={512}
        height={512}
        priority={priority}
        unoptimized
        className={cn("h-full w-full object-contain p-1", imageClassName)}
      />
    </span>
  )
}
