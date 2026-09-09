import { MobileServiceClient } from "./mobile-service-client"
import { notFound } from "next/navigation"
import { isStaticExportRouteParam, STATIC_EXPORT_PARAM } from "@/lib/static-export-route"

export async function generateStaticParams() {
  return [{ serviceCode: STATIC_EXPORT_PARAM }]
}

type CustomerServiceEntryPageProps = {
  params: Promise<{
    serviceCode?: string
  }>
}

function normalizeServiceCodeParam(value?: string) {
  const raw = value?.trim() ?? ""
  if (!raw) {
    return ""
  }
  try {
    return decodeURIComponent(raw)
  } catch {
    return raw
  }
}

export default async function CustomerServiceEntryPage({
  params,
}: CustomerServiceEntryPageProps) {
  const { serviceCode } = await params

  if (isStaticExportRouteParam(serviceCode)) {
    notFound()
  }

  return (
    <MobileServiceClient
      serviceCode={normalizeServiceCodeParam(serviceCode)}
    />
  )
}
