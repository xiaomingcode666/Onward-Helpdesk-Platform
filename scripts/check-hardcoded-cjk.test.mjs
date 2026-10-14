import assert from "node:assert/strict"
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import path from "node:path"
import test from "node:test"

import {
  collectCounts,
  compareWithBaseline,
  countHardcodedCjkLines,
  isScannedFile,
  stripComments,
} from "./check-hardcoded-cjk.mjs"

test("ignores Chinese in Go comments but counts string literals", () => {
  const source = [
    "// 中文注释",
    "/* 多行",
    "   注释 */",
    'name := "技术维护组" // 尾注释',
    "raw := `原始\\字符串`",
    "url := \"https://example.com\" // 地址",
    "rune := '中'",
  ].join("\n")

  assert.equal(countHardcodedCjkLines(source, "go"), 3)
  assert.doesNotMatch(stripComments(source, "go"), /注释|地址/)
})

test("counts TS strings, template literals and JSX text but not comments", () => {
  const source = [
    "// 注释",
    'const title = "通知中心"',
    "const message = `共 ${count} 条`",
    "return <p>Don't 显示</p>",
    "const next = 1 /* 块注释 */",
    "/**",
    " * 文档注释",
    " */",
  ].join("\n")

  assert.equal(countHardcodedCjkLines(source, "ts"), 3)
})

test("an apostrophe in JSX text does not swallow later lines", () => {
  const source = ["<p>Don't</p>", "// 注释", 'const label = "标签"'].join("\n")

  assert.equal(countHardcodedCjkLines(source, "ts"), 1)
})

test("excludes tests, migrations and declaration files", () => {
  assert.equal(isScannedFile("internal/services/sla_service.go"), true)
  assert.equal(isScannedFile("internal/services/sla_service_test.go"), false)
  assert.equal(isScannedFile("internal/migration/000123_enable.go"), false)
  assert.equal(isScannedFile("web/lib/api/client.test.ts"), false)
  assert.equal(isScannedFile("web/lib/types.d.ts"), false)
  assert.equal(isScannedFile("web/app/page.tsx"), true)
})

test("fails on new files or increased counts and reports decreases", () => {
  const { increased, decreased } = compareWithBaseline(
    { "a.go": 3, "b.tsx": 1, "c.go": 1 },
    { "a.go": 2, "b.tsx": 4, "d.go": 2 },
  )

  assert.deepEqual(increased, [
    { file: "a.go", count: 3, allowed: 2 },
    { file: "c.go", count: 1, allowed: 0 },
  ])
  assert.deepEqual(decreased, [
    { file: "b.tsx", count: 1, allowed: 4 },
    { file: "d.go", count: 0, allowed: 2 },
  ])
})

test("collects counts from scanned roots only", () => {
  const root = mkdtempSync(path.join(tmpdir(), "cjk-check-"))
  try {
    mkdirSync(path.join(root, "internal/services"), { recursive: true })
    mkdirSync(path.join(root, "internal/migration"), { recursive: true })
    mkdirSync(path.join(root, "web/lib/node_modules/pkg"), { recursive: true })
    writeFileSync(path.join(root, "internal/services/team.go"), 'package services\nconst name = "技术维护组"\n')
    writeFileSync(path.join(root, "internal/services/team_test.go"), 'package services\nconst want = "技术维护组"\n')
    writeFileSync(path.join(root, "internal/migration/001.go"), 'package migration\nconst old = "旧数据"\n')
    writeFileSync(path.join(root, "web/lib/label.ts"), 'export const label = "标签"\n// 注释\n')
    writeFileSync(path.join(root, "web/lib/node_modules/pkg/index.ts"), 'export const x = "依赖"\n')

    assert.deepEqual(collectCounts(root), {
      "internal/services/team.go": 1,
      "web/lib/label.ts": 1,
    })
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})
