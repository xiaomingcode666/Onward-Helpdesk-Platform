import Component from "./page-client"
import { notFound } from "next/navigation"
import { isStaticExportRouteParam, STATIC_EXPORT_PARAM } from "@/lib/static-export-route"

/**
 * Product detail is a database-driven dynamic route: products keep growing after deployment,
 * so they cannot be enumerated at build time.
 *
 * - SaaS admin build (default): server-render on demand; any real product ID can be opened and refreshed directly.
 * - Static export build (NEXT_STATIC_EXPORT=1): emit a static-export sentinel to satisfy static export constraints;
 *   the embedded SPA fallback takes over real product IDs at runtime.
 */
export function generateStaticParams() {
  return [{ productId: STATIC_EXPORT_PARAM }]
}

type PageProps = {
  params: Promise<{ productId: string }>
}

export default async function Page({ params }: PageProps) {
  const { productId } = await params
  if (isStaticExportRouteParam(productId)) {
    notFound()
  }
  return <Component />
}
