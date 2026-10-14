"use client"

import { tm } from "@/components/marketing/marketing-i18n"
import { useI18n } from "@/i18n/provider"

import { type FormEvent, useState } from "react"
import { CheckCircle2Icon, LoaderCircleIcon, SendIcon } from "lucide-react"

import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { submitDemoRequest } from "@/lib/api/marketing"

type DemoRequestDialogProps = {
  className?: string
  label?: string
}

const fieldClassName = "h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none transition placeholder:text-slate-400 focus:border-emerald-600 focus:ring-2 focus:ring-emerald-100"

export function DemoRequestDialog({ className, label = tm("marketing.demoRequestDialog.t017") }: DemoRequestDialogProps) {
  useI18n()
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [error, setError] = useState("")
  const [form, setForm] = useState({ company: "", contactName: "", email: "", mobile: "", countryRegion: "", requirements: "", website: "" })

  const update = (field: keyof typeof form, value: string) => setForm((current) => ({ ...current, [field]: value }))
  const changeOpen = (nextOpen: boolean) => {
    setOpen(nextOpen)
    if (!nextOpen) {
      setError("")
      setSubmitted(false)
    }
  }

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setSubmitting(true)
    setError("")
    try {
      await submitDemoRequest(form)
      setSubmitted(true)
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : tm("marketing.demoRequestDialog.t018"))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <button type="button" className={className} onClick={() => setOpen(true)}>{label}</button>
      <Dialog open={open} onOpenChange={changeOpen}>
        <DialogContent className="max-h-[calc(100vh-2rem)] overflow-y-auto p-0 sm:max-w-[580px]" showCloseButton={!submitting}>
          {submitted ? (
            <div className="px-7 py-12 text-center sm:px-10">
              <CheckCircle2Icon className="mx-auto size-10 text-emerald-600" />
              <DialogTitle className="mt-5 text-xl">{tm("marketing.demoRequestDialog.t005")}</DialogTitle>
              <DialogDescription className="mt-3 text-sm leading-6">{tm("marketing.demoRequestDialog.t006")}</DialogDescription>
              <button type="button" className="mt-7 inline-flex h-10 items-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-700" onClick={() => changeOpen(false)}>{tm("marketing.demoRequestDialog.t007")}</button>
            </div>
          ) : (
            <form onSubmit={submit}>
              <DialogHeader className="border-b border-slate-200 px-6 pb-5 pt-6 sm:px-7">
                <DialogTitle className="text-xl">{tm("marketing.demoRequestDialog.t008")}</DialogTitle>
                <DialogDescription className="mt-1 text-sm leading-6">{tm("marketing.demoRequestDialog.t009")}</DialogDescription>
              </DialogHeader>
              <div className="grid gap-4 px-6 py-6 sm:grid-cols-2 sm:px-7">
                <label className="grid gap-1.5 text-sm font-medium text-slate-700 sm:col-span-2">{tm("marketing.demoRequestDialog.t010")}<input required value={form.company} onChange={(event) => update("company", event.target.value)} className={fieldClassName} placeholder={tm("marketing.demoRequestDialog.t001")} /></label>
                <label className="grid gap-1.5 text-sm font-medium text-slate-700">{tm("marketing.demoRequestDialog.t011")}<input required value={form.contactName} onChange={(event) => update("contactName", event.target.value)} className={fieldClassName} placeholder={tm("marketing.demoRequestDialog.t002")} /></label>
                <label className="grid gap-1.5 text-sm font-medium text-slate-700">{tm("marketing.demoRequestDialog.t012")}<input required type="email" value={form.email} onChange={(event) => update("email", event.target.value)} className={fieldClassName} placeholder="name@company.com" /></label>
                <label className="grid gap-1.5 text-sm font-medium text-slate-700">{tm("marketing.demoRequestDialog.t013")}<input value={form.mobile} onChange={(event) => update("mobile", event.target.value)} className={fieldClassName} placeholder={tm("marketing.demoRequestDialog.t003")} /></label>
                <label className="grid gap-1.5 text-sm font-medium text-slate-700">{tm("marketing.demoRequestDialog.t014")}<input value={form.countryRegion} onChange={(event) => update("countryRegion", event.target.value)} className={fieldClassName} placeholder={tm("marketing.demoRequestDialog.t003")} /></label>
                <label className="grid gap-1.5 text-sm font-medium text-slate-700 sm:col-span-2">{tm("marketing.demoRequestDialog.t015")}<textarea value={form.requirements} onChange={(event) => update("requirements", event.target.value)} className="min-h-28 w-full resize-y rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 outline-none transition placeholder:text-slate-400 focus:border-emerald-600 focus:ring-2 focus:ring-emerald-100" placeholder={tm("marketing.demoRequestDialog.t004")} /></label>
                <label className="absolute -left-[9999px]" aria-hidden="true">Website<input tabIndex={-1} autoComplete="off" value={form.website} onChange={(event) => update("website", event.target.value)} /></label>
                {error ? <p className="sm:col-span-2 text-sm text-red-600">{error}</p> : null}
              </div>
              <div className="flex items-center justify-end gap-3 border-t border-slate-200 px-6 py-4 sm:px-7">
                <button type="button" className="h-10 rounded-md px-3 text-sm font-medium text-slate-600 transition hover:bg-slate-100" onClick={() => changeOpen(false)} disabled={submitting}>{tm("marketing.demoRequestDialog.t016")}</button>
                <button type="submit" className="inline-flex h-10 items-center gap-2 rounded-md bg-blue-600 px-4 text-sm font-semibold text-white transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" disabled={submitting}>{submitting ? <LoaderCircleIcon className="size-4 animate-spin" /> : <SendIcon className="size-4" />}{submitting ? tm("marketing.demoRequestDialog.t019") : tm("marketing.demoRequestDialog.t020")}</button>
              </div>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </>
  )
}
