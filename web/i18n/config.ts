export const SUPPORTED_LOCALES = ["zh-CN", "en-US", "es-ES"] as const
export type AppLocale = (typeof SUPPORTED_LOCALES)[number]
export const SELECTABLE_LOCALES = ["zh-CN", "en-US"] as const satisfies readonly AppLocale[]
export type SelectableLocale = (typeof SELECTABLE_LOCALES)[number]

export const DEFAULT_LOCALE: AppLocale = "zh-CN"

const LOCALE_ALIASES: Record<string, AppLocale> = {
  zh: "zh-CN",
  "zh-cn": "zh-CN",
  zh_cn: "zh-CN",
  "zh-hans": "zh-CN",
  en: "en-US",
  "en-us": "en-US",
  en_us: "en-US",
  es: "es-ES",
  "es-es": "es-ES",
  es_es: "es-ES",
  "es-mx": "es-ES",
  es_mx: "es-ES",
}

export function normalizeLocale(value: string | null | undefined): AppLocale {
  return normalizeSupportedLocale(value) ?? DEFAULT_LOCALE
}

export function normalizeSelectableLocale(
  value: string | null | undefined
): SelectableLocale {
  const locale = normalizeLocale(value)
  return SELECTABLE_LOCALES.includes(locale as SelectableLocale)
    ? (locale as SelectableLocale)
    : "zh-CN"
}

function normalizeSupportedLocale(
  value: string | null | undefined
): AppLocale | null {
  if (!value) {
    return null
  }
  const key = value.trim().toLowerCase()
  return LOCALE_ALIASES[key] ?? null
}

export function isSupportedLocale(
  value: string | null | undefined
): value is AppLocale {
  if (!value) {
    return false
  }
  return SUPPORTED_LOCALES.includes(value as AppLocale)
}

let configuredLocale: AppLocale = DEFAULT_LOCALE

export function readStoredLocale(): AppLocale {
  return configuredLocale
}

export function configureLocale(locale: string | null | undefined): AppLocale {
  configuredLocale = normalizeLocale(locale)
  return configuredLocale
}
