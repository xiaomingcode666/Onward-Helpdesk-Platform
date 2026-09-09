"use client"

import { useEffect, useState } from "react"
import { PaletteIcon } from "lucide-react"

import { useI18n } from "@/i18n/provider"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

type PaletteMode = "plain"

const PALETTE_STORAGE_KEY = "remote_helpdesk_palette"
const LEGACY_PALETTE_STORAGE_KEY = "dashboard_palette"
const DEFAULT_PALETTE: PaletteMode = "plain"

const paletteOptions: Array<{
  value: PaletteMode
  labelKey: string
  swatch: string
}> = [
  {
    value: "plain",
    labelKey: "palette.plain",
    swatch: "bg-[#3B82F6]",
  },
]

function readPalette(): PaletteMode {
  return DEFAULT_PALETTE
}

function applyPalette() {
  document.documentElement.dataset.palette = DEFAULT_PALETTE
  window.localStorage.setItem(PALETTE_STORAGE_KEY, DEFAULT_PALETTE)
  window.localStorage.removeItem(LEGACY_PALETTE_STORAGE_KEY)
}

export function PaletteToggle() {
  const t = useI18n()
  const [palette, setPalette] = useState<PaletteMode>(DEFAULT_PALETTE)

  useEffect(() => {
    const storedPalette = readPalette()
    // RailOps uses one canonical color system; legacy stored palettes are normalized after hydration.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setPalette(storedPalette)
    applyPalette()
  }, [])

  function handleChange() {
    setPalette(DEFAULT_PALETTE)
    applyPalette()
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<Button variant="outline" size="sm" />}
        aria-label={t("palette.toggle")}
      >
        <PaletteIcon />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52 min-w-52">
        <DropdownMenuRadioGroup value={palette} onValueChange={handleChange}>
          {paletteOptions.map((option) => (
            <DropdownMenuRadioItem key={option.value} value={option.value}>
              <span className={`size-2.5 rounded-full ${option.swatch}`} />
              <span className="flex-1">{t(option.labelKey)}</span>
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
