export const customerThemeValues = [
  "default",
  "odt-intelligence",
  "clinical-calm",
  "signal-coral",
] as const

export type CustomerTheme = (typeof customerThemeValues)[number]

const customerThemeClasses: Record<CustomerTheme, string> = {
  default: "rhd-customer-theme-default",
  "odt-intelligence": "rhd-customer-theme-odt",
  "clinical-calm": "rhd-customer-theme-clinical",
  "signal-coral": "rhd-customer-theme-coral",
}

export function normalizeCustomerTheme(value?: string | null): CustomerTheme {
  const normalized = value?.trim().toLowerCase()
  return customerThemeValues.find((theme) => theme === normalized) ?? "default"
}

export function getCustomerThemeClassName(value?: string | null) {
  return customerThemeClasses[normalizeCustomerTheme(value)]
}
