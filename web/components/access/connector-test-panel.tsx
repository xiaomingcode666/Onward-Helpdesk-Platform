"use client"

import { useState } from "react"
import {
  CheckCircle2Icon,
  XCircleIcon,
  Loader2Icon,
  ActivityIcon,
  AlertTriangleIcon,
  RefreshCwIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useI18n } from "@/i18n/provider"

export interface TestResult {
  success: boolean
  latencyMs: number
  errorMessage?: string
  errorDetail?: string
  testedAt: string
}

interface ConnectorTestPanelProps {
  connectorId: string
  onTest: () => Promise<TestResult>
  lastTestResult?: TestResult | null
}

function formatDuration(ms: number): string {
  if (ms < 1000) {
    return `${ms}ms`
  }
  return `${(ms / 1000).toFixed(2)}s`
}

function getLatencyBadge(latencyMs: number) {
  if (latencyMs < 500) {
    return { variant: "default" as const, labelKey: "access.latencyFast" }
  }
  if (latencyMs < 2000) {
    return { variant: "secondary" as const, labelKey: "access.latencyNormal" }
  }
  if (latencyMs < 5000) {
    return { variant: "outline" as const, labelKey: "access.latencySlow" }
  }
  return { variant: "destructive" as const, labelKey: "access.latencyTimeout" }
}

export function ConnectorTestPanel({
  connectorId,
  onTest,
  lastTestResult,
}: ConnectorTestPanelProps) {
  const t = useI18n()
  const [testing, setTesting] = useState(false)
  const [result, setResult] = useState<TestResult | null>(
    lastTestResult ?? null
  )
  const [showDetail, setShowDetail] = useState(false)
  const latencyBadge = result ? getLatencyBadge(result.latencyMs) : null

  async function handleTest() {
    setTesting(true)
    setResult(null)
    try {
      const testResult = await onTest()
      setResult(testResult)
    } catch (err) {
      setResult({
        success: false,
        latencyMs: 0,
        errorMessage:
          err instanceof Error
            ? err.message
            : t("access.testUnknownError"),
        testedAt: new Date().toISOString(),
      })
    } finally {
      setTesting(false)
    }
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
        <CardTitle className="text-base font-semibold">
          {t("access.testConnection")}
        </CardTitle>
        <Button
          type="button"
          variant={result?.success === false ? "destructive" : "outline"}
          size="sm"
          onClick={handleTest}
          disabled={testing}
        >
          {testing ? (
            <>
              <Loader2Icon className="size-4 animate-spin" />
              {t("access.testing")}
            </>
          ) : (
            <>
              <RefreshCwIcon className="size-4" />
              {t("access.testNow")}
            </>
          )}
        </Button>
      </CardHeader>
      <CardContent className="space-y-3">
        {testing && (
          <div className="flex items-center gap-3 rounded-md border bg-muted/30 px-4 py-3">
            <Loader2Icon className="size-5 animate-spin text-primary" />
            <div className="text-sm">
              <p className="font-medium">{t("access.testInProgress")}</p>
            </div>
          </div>
        )}

        {result && !testing && (
          <div
            className={`rounded-md border px-4 py-3 ${
              result.success
                ? "border-primary/20 bg-primary/10"
                : "border-destructive/30 bg-destructive/5"
            }`}
          >
            <div className="flex items-start gap-3">
              {result.success ? (
                <CheckCircle2Icon className="mt-0.5 size-5 shrink-0 text-primary" />
              ) : (
                <XCircleIcon className="mt-0.5 size-5 shrink-0 text-destructive" />
              )}
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">
                  {result.success
                    ? t("access.testSuccess")
                    : t("access.testFailed")}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {t("access.testedAt")}:{" "}
                  {new Date(result.testedAt).toLocaleString()}
                </p>

                {latencyBadge ? (
                  <div className="mt-2 flex flex-wrap gap-3">
                    <div className="flex items-center gap-1.5 text-xs">
                      <ActivityIcon className="size-3.5 text-muted-foreground" />
                      <span className="text-muted-foreground">
                        {t("access.latency")}:
                      </span>
                      <Badge
                        variant={latencyBadge.variant}
                        className="text-rhd-2xs"
                      >
                        {formatDuration(result.latencyMs)}
                      </Badge>
                      <span className="text-rhd-2xs text-muted-foreground">
                        ({t(latencyBadge.labelKey)})
                      </span>
                    </div>
                  </div>
                ) : null}

                {!result.success && result.errorMessage && (
                  <div className="mt-3 space-y-2">
                    <div className="flex items-center gap-1.5 text-xs text-destructive">
                      <AlertTriangleIcon className="size-3.5" />
                      <span className="font-medium">
                        {t("access.errorMessage")}:
                      </span>
                      {result.errorMessage}
                    </div>
                    {result.errorDetail && (
                      <>
                        <button
                          type="button"
                          onClick={() => setShowDetail(!showDetail)}
                          className="text-xs text-muted-foreground underline underline-offset-2 hover:text-foreground"
                        >
                          {showDetail
                            ? t("common.hideDetail")
                            : t("common.showDetail")}
                        </button>
                        {showDetail && (
                          <pre className="overflow-auto rounded-md bg-muted p-3 text-xs leading-relaxed">
                            {result.errorDetail}
                          </pre>
                        )}
                      </>
                    )}
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        {!result && !testing && (
          <div className="flex items-center gap-2 rounded-md border border-dashed px-4 py-3 text-sm text-muted-foreground">
            <ActivityIcon className="size-4" />
            {t("access.testNotRun")}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
