import type { AppLocale } from "@/i18n/config"
import { customerEntryMessages } from "./customer-entry"
import { dashboardMessages } from "./dashboard"
import { engineerMessages } from "./engineer"
import { enumsMessages } from "./enums"
import { enterpriseMessages } from "./enterprise"
import { exportsMessages } from "./exports"
import { iamMessages } from "./iam"
import { marketingMessages } from "./marketing"
import { meetingRoomMessages } from "./meeting-room"
import { miscTailMessages } from "./misc-tail"
import { opsComponentsMessages } from "./ops-components"
import { orgMessages } from "./org"
import { partnerMessages } from "./partner"
import { platformMessages } from "./platform"
import { portalMessages } from "./portal"
import { railopsMessages } from "./railops"
import { transferMessages } from "./transfer"
import { workflowMessages } from "./workflow"

const extractedMessageGroups = [
  railopsMessages,
  platformMessages,
  enterpriseMessages,
  workflowMessages,
  portalMessages,
  iamMessages,
  partnerMessages,
  orgMessages,
  customerEntryMessages,
  dashboardMessages,
  engineerMessages,
  meetingRoomMessages,
  opsComponentsMessages,
  transferMessages,
  exportsMessages,
  miscTailMessages,
  enumsMessages,
  marketingMessages,
]

export function getExtractedMessage(locale: AppLocale, key: string): unknown {
  for (const group of extractedMessageGroups) {
    const value = getMessageValue(group[locale], key)
    if (value !== undefined) {
      return value
    }
  }
  return undefined
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
