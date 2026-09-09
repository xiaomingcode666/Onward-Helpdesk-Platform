"use client"

import { CheckIcon, LanguagesIcon } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { createPortal } from "react-dom"

import { Button } from "@/components/ui/button"
import { SELECTABLE_LOCALES, normalizeSelectableLocale, type SelectableLocale } from "@/i18n/config"
import { useAppLocale } from "@/i18n/provider"
import { cn } from "@/lib/utils"

const mobileLocaleShortLabels: Record<SelectableLocale, string> = {
  "zh-CN": "中",
  "en-US": "EN",
}

type MenuPosition = {
  top: number
  right: number
  minWidth: number
}

export function MobileLanguageSwitch({
  tone = "light",
  className,
}: {
  tone?: "light" | "dark"
  className?: string
}) {
  const { locale, setLocale, t } = useAppLocale()
  const currentLocale = normalizeSelectableLocale(locale)
  const [open, setOpen] = useState(false)
  const [menuPosition, setMenuPosition] = useState<MenuPosition | null>(null)
  const rootRef = useRef<HTMLDivElement | null>(null)
  const menuRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (!open) {
      setMenuPosition(null)
      return
    }

    function updateMenuPosition() {
      const rect = rootRef.current?.getBoundingClientRect()
      if (!rect) return
      const menuHeight = 104
      const belowTop = rect.bottom + 8
      const aboveTop = rect.top - menuHeight - 8
      setMenuPosition({
        top: belowTop + menuHeight > window.innerHeight ? Math.max(8, aboveTop) : belowTop,
        right: Math.max(8, window.innerWidth - rect.right),
        minWidth: Math.max(144, rect.width),
      })
    }

    function handlePointerDown(event: PointerEvent) {
      const target = event.target as Node | null
      if (!target) return
      if (rootRef.current?.contains(target) || menuRef.current?.contains(target)) {
        return
      }
      setOpen(false)
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false)
      }
    }

    updateMenuPosition()
    document.addEventListener("pointerdown", handlePointerDown)
    document.addEventListener("keydown", handleKeyDown)
    window.addEventListener("resize", updateMenuPosition)
    window.addEventListener("scroll", updateMenuPosition, true)
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown)
      document.removeEventListener("keydown", handleKeyDown)
      window.removeEventListener("resize", updateMenuPosition)
      window.removeEventListener("scroll", updateMenuPosition, true)
    }
  }, [open])

  const menu = open && menuPosition && typeof document !== "undefined"
    ? createPortal(
      <div
        ref={menuRef}
        id="mobile-language-switch-menu"
        role="menu"
        aria-label={t("account.locale")}
        className={cn(
          "fixed z-[1000] overflow-hidden rounded-md border shadow-[var(--railops-dropdown-shadow)]",
          tone === "dark"
            ? "border-white/15 bg-[#101827] text-white"
            : "border-[var(--railops-border)] bg-[var(--railops-surface)] text-[var(--railops-text)]",
        )}
        style={{
          top: menuPosition.top,
          right: menuPosition.right,
          minWidth: menuPosition.minWidth,
        }}
      >
        {SELECTABLE_LOCALES.map((item) => {
          const selected = item === currentLocale
          return (
            <button
              key={item}
              type="button"
              role="menuitemradio"
              aria-checked={selected}
              className={cn(
                "flex w-full items-center justify-between gap-3 px-3 py-2.5 text-left text-rhd-xs transition",
                tone === "dark"
                  ? "hover:bg-white/10"
                  : "hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)]",
                selected && (tone === "dark" ? "bg-white/10" : "bg-[var(--railops-primary-bg)]"),
              )}
              onClick={() => {
                setLocale(normalizeSelectableLocale(item))
                setOpen(false)
              }}
            >
              <span>{t(`locale.${item}`)}</span>
              {selected ? <CheckIcon className="size-4 shrink-0" /> : null}
            </button>
          )
        })}
      </div>,
      document.body,
    )
    : null

  return (
    <div ref={rootRef} className={cn("relative inline-flex shrink-0", className)}>
      <Button
        type="button"
        variant="outline"
        size="sm"
        className={cn(
          "h-9 shrink-0 gap-1.5 rounded-full px-2.5 text-rhd-xs font-semibold shadow-none",
          tone === "dark"
            ? "border-white/20 bg-white/10 text-white hover:bg-white/20 hover:text-white"
            : "border-[#dce2ea] bg-white text-[#465266] hover:bg-[#f4f6f8] hover:text-[#172033]",
        )}
        aria-label={t("account.languageSettings")}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? "mobile-language-switch-menu" : undefined}
        title={t("account.languageSettings")}
        onClick={() => setOpen((current) => !current)}
      >
        <LanguagesIcon className="size-4" />
        <span>{mobileLocaleShortLabels[currentLocale]}</span>
      </Button>
      {menu}
    </div>
  )
}
