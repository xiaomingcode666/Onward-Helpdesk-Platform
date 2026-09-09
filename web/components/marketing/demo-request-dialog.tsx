"use client"

import { type FormEvent, useState } from "react"
import { CheckCircle2Icon, LoaderCircleIcon, SendIcon } from "lucide-react"

import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { submitDemoRequest } from "@/lib/api/marketing"

type DemoRequestDialogProps = {
  className?: string
  label?: string
}

const fieldClassName = "h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none transition placeholder:text-slate-400 focus:border-emerald-600 focus:ring-2 focus:ring-emerald-100"

export function DemoRequestDialog({ className, label = "申请 Demo" }: DemoRequestDialogProps) {
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
      setError(submitError instanceof Error ? submitError.message : "提交失败，请稍后重试")
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
              <DialogTitle className="mt-5 text-xl">申请已提交</DialogTitle>
              <DialogDescription className="mt-3 text-sm leading-6">我们已收到你的 Demo 申请，会通过你留下的联系方式与您沟通。</DialogDescription>
              <button type="button" className="mt-7 inline-flex h-10 items-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-700" onClick={() => changeOpen(false)}>返回首页</button>
            </div>
          ) : (
            <form onSubmit={submit}>
              <DialogHeader className="border-b border-slate-200 px-6 pb-5 pt-6 sm:px-7">
                <DialogTitle className="text-xl">申请产品 Demo</DialogTitle>
                <DialogDescription className="mt-1 text-sm leading-6">留下企业与服务场景信息，我们将为你准备更贴近业务的演示。</DialogDescription>
              </DialogHeader>
              <div className="grid gap-4 px-6 py-6 sm:grid-cols-2 sm:px-7">
                <label className="grid gap-1.5 text-sm font-medium text-slate-700 sm:col-span-2">企业名称<input required value={form.company} onChange={(event) => update("company", event.target.value)} className={fieldClassName} placeholder="例如：XX 工业设备有限公司" /></label>
                <label className="grid gap-1.5 text-sm font-medium text-slate-700">联系人<input required value={form.contactName} onChange={(event) => update("contactName", event.target.value)} className={fieldClassName} placeholder="你的姓名" /></label>
                <label className="grid gap-1.5 text-sm font-medium text-slate-700">工作邮箱<input required type="email" value={form.email} onChange={(event) => update("email", event.target.value)} className={fieldClassName} placeholder="name@company.com" /></label>
                <label className="grid gap-1.5 text-sm font-medium text-slate-700">联系电话<input value={form.mobile} onChange={(event) => update("mobile", event.target.value)} className={fieldClassName} placeholder="选填" /></label>
                <label className="grid gap-1.5 text-sm font-medium text-slate-700">国家或地区<input value={form.countryRegion} onChange={(event) => update("countryRegion", event.target.value)} className={fieldClassName} placeholder="选填" /></label>
                <label className="grid gap-1.5 text-sm font-medium text-slate-700 sm:col-span-2">希望解决的问题<textarea value={form.requirements} onChange={(event) => update("requirements", event.target.value)} className="min-h-28 w-full resize-y rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 outline-none transition placeholder:text-slate-400 focus:border-emerald-600 focus:ring-2 focus:ring-emerald-100" placeholder="例如：海外设备报修、服务派单、远程专家协作等" /></label>
                <label className="absolute -left-[9999px]" aria-hidden="true">Website<input tabIndex={-1} autoComplete="off" value={form.website} onChange={(event) => update("website", event.target.value)} /></label>
                {error ? <p className="sm:col-span-2 text-sm text-red-600">{error}</p> : null}
              </div>
              <div className="flex items-center justify-end gap-3 border-t border-slate-200 px-6 py-4 sm:px-7">
                <button type="button" className="h-10 rounded-md px-3 text-sm font-medium text-slate-600 transition hover:bg-slate-100" onClick={() => changeOpen(false)} disabled={submitting}>取消</button>
                <button type="submit" className="inline-flex h-10 items-center gap-2 rounded-md bg-blue-600 px-4 text-sm font-semibold text-white transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" disabled={submitting}>{submitting ? <LoaderCircleIcon className="size-4 animate-spin" /> : <SendIcon className="size-4" />}{submitting ? "提交中" : "提交申请"}</button>
              </div>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </>
  )
}
