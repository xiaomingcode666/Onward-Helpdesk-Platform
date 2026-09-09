export type CsvCell = boolean | number | string | null | undefined
export type CsvRow = CsvCell[]

function formatCsvCell(value: CsvCell) {
  const raw = value === null || value === undefined ? "" : String(value)
  const safe = /^[=+\-@]/.test(raw) ? `'${raw}` : raw
  return `"${safe.replace(/"/g, '""')}"`
}

export function downloadCsv(filename: string, rows: CsvRow[]) {
  const csv = `\uFEFF${rows.map((row) => row.map(formatCsvCell).join(",")).join("\n")}`
  const blob = new Blob([csv], { type: "text/csv;charset=utf-8" })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement("a")
  anchor.href = url
  anchor.download = filename
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
}
