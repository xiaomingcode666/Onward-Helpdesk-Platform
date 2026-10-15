import { DEFAULT_LOCALE, SUPPORTED_LOCALES, type AppLocale, readStoredLocale } from "@/i18n/config"
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

/**
 * Recognizes text that was rendered from `key` in any supported locale and
 * returns its placeholder values. Use it only for legacy records that stored
 * rendered text instead of a message key, so they can be shown in the current
 * locale without hardcoding source-language patterns in components.
 */
export function matchMessage(
  key: string,
  text: string
): Record<string, string> | null {
  const normalizedText = normalizeMatchText(text)
  for (const locale of SUPPORTED_LOCALES) {
    const template = getMessage(locale, key)
    if (typeof template !== "string") continue
    const names: string[] = []
    const pattern = normalizeMatchText(template)
      .split(/(\{\w+\})/)
      .map((part) => {
        const placeholder = part.match(/^\{(\w+)\}$/)
        if (!placeholder) return part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
        names.push(placeholder[1])
        return "\\s*(.+?)\\s*"
      })
      .join("")
    const match = normalizedText.match(new RegExp(`^${pattern}$`))
    if (match) {
      return Object.fromEntries(names.map((name, index) => [name, match[index + 1]]))
    }
  }
  return null
}

function normalizeMatchText(value: string) {
  return value
    .trim()
    .replace(/：/g, ":")
    .replace(/（/g, "(")
    .replace(/）/g, ")")
    .replace(/\s*([:()])\s*/g, "$1")
    .replace(/[.。]$/, "")
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
