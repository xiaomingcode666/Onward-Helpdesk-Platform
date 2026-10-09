"use client"

import type { UseFormRegisterReturn } from "react-hook-form"

import { Button } from "@/components/ui/button"
import { Field, FieldContent, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import type { ChannelSecretStatus } from "@/lib/api/admin"

type Translate = (key: string, values?: Record<string, string | number>) => string

type WhatsAppSecretFieldProps = {
  id: string
  label: string
  status?: ChannelSecretStatus
  cleared: boolean
  hasReference: boolean
  error?: { message?: string }
  registration: UseFormRegisterReturn
  onToggleClear: () => void
  t: Translate
}

// Write-only secret input: the stored value is never prefilled or shown.
export function WhatsAppSecretField({ id, label, status, cleared, hasReference, error, registration, onToggleClear, t }: WhatsAppSecretFieldProps) {
  const configured = Boolean(status?.configured) && !cleared
  const placeholder = cleared
    ? t("channel.secretWillClear")
    : configured
      ? t("channel.secretConfigured", { suffix: status?.fingerprintSuffix ?? "" })
      : t("channel.secretNotConfigured")
  return (
    <Field data-invalid={!!error}>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <FieldContent>
        <div className="flex gap-2">
          <Input id={id} type="password" autoComplete="new-password" spellCheck={false} placeholder={placeholder} {...registration} />
          {status?.configured ? (
            <Button type="button" variant="outline" size="sm" onClick={onToggleClear}>
              {cleared ? t("channel.secretUndoClear") : t("channel.secretClear")}
            </Button>
          ) : null}
        </div>
        {configured && hasReference ? <p className="text-xs text-muted-foreground">{t("channel.secretPageOverridesRef")}</p> : null}
        <FieldError errors={error ? [error] : []} />
      </FieldContent>
    </Field>
  )
}
