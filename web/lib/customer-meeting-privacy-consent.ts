export const CUSTOMER_MEETING_PRIVACY_NOTICE_VERSION = "2026-08-12-meeting-v1"

const CUSTOMER_MEETING_PRIVACY_STORAGE_PREFIX = "remotehelpdesk:customer-meeting-privacy-consent"

type CustomerMeetingPrivacyScope = {
  tenantId: number
  userId: number
}

type StoredCustomerMeetingPrivacyConsent = {
  policyVersion: string
  acceptedAt: string
}

function customerMeetingPrivacyStorageKey(scope: CustomerMeetingPrivacyScope) {
  if (scope.tenantId <= 0 || scope.userId <= 0) return ""
  return `${CUSTOMER_MEETING_PRIVACY_STORAGE_PREFIX}:${scope.tenantId}:${scope.userId}`
}

export function hasRememberedCustomerMeetingPrivacyConsent(scope: CustomerMeetingPrivacyScope) {
  if (typeof window === "undefined") return false
  const storageKey = customerMeetingPrivacyStorageKey(scope)
  if (!storageKey) return false

  try {
    const raw = window.localStorage.getItem(storageKey)
    if (!raw) return false
    const consent = JSON.parse(raw) as StoredCustomerMeetingPrivacyConsent
    return consent.policyVersion === CUSTOMER_MEETING_PRIVACY_NOTICE_VERSION && Boolean(consent.acceptedAt)
  } catch {
    window.localStorage.removeItem(storageKey)
    return false
  }
}

export function rememberCustomerMeetingPrivacyConsent(
  scope: CustomerMeetingPrivacyScope,
  accepted: boolean,
) {
  if (typeof window === "undefined") return
  const storageKey = customerMeetingPrivacyStorageKey(scope)
  if (!storageKey) return

  if (!accepted) {
    window.localStorage.removeItem(storageKey)
    return
  }

  const consent: StoredCustomerMeetingPrivacyConsent = {
    policyVersion: CUSTOMER_MEETING_PRIVACY_NOTICE_VERSION,
    acceptedAt: new Date().toISOString(),
  }
  window.localStorage.setItem(storageKey, JSON.stringify(consent))
}
