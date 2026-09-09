import { DEFAULT_LOCALE, type AppLocale, readStoredLocale } from "@/i18n/config"
import { getExtractedMessage } from "@/i18n/extracted"
import enUSMessages from "@/messages/en-US.json"
import esESMessages from "@/messages/es-ES.json"
import zhCNMessages from "@/messages/zh-CN.json"

const messages = {
  "zh-CN": zhCNMessages,
  "en-US": enUSMessages,
  "es-ES": esESMessages,
} satisfies Record<AppLocale, typeof zhCNMessages>

const FALLBACK_LOCALE: AppLocale = "en-US"

export function translateMessage(
  locale: AppLocale,
  key: string,
  values?: Record<string, string | number>
): string {
  const value = getMessage(locale, key)
  if (typeof value === "string") {
    return formatMessage(value, values)
  }
  if (locale !== FALLBACK_LOCALE) {
    const fallback = getMessage(FALLBACK_LOCALE, key)
    if (typeof fallback === "string") {
      return formatMessage(fallback, values)
    }
  }
  if (locale !== DEFAULT_LOCALE) {
    const fallback = getMessage(DEFAULT_LOCALE, key)
    if (typeof fallback === "string") {
      return formatMessage(fallback, values)
    }
  }
  return key
}

export function translateCurrentMessage(
  key: string,
  values?: Record<string, string | number>
): string {
  return translateMessage(readStoredLocale(), key, values)
}

function getMessage(locale: AppLocale, key: string): unknown {
  const value = getMessageValue(messages[locale], key)
  return value === undefined ? getExtractedMessage(locale, key) : value
}

function getMessageValue(source: unknown, key: string): unknown {
  let current = source
  for (const part of key.split(".")) {
    if (!current || typeof current !== "object" || !(part in current)) {
      return undefined
    }
    current = (current as Record<string, unknown>)[part]
  }
  return current
}

function formatMessage(
  message: string,
  values?: Record<string, string | number>
): string {
  if (!values) {
    return message
  }
  return message.replace(/\{(\w+)\}/g, (match, key) =>
    Object.prototype.hasOwnProperty.call(values, key) ? String(values[key]) : match
  )
}
