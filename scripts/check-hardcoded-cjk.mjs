#!/usr/bin/env node
// 防止新增写死的中文：系统文本必须走多语言资源（后端 internal/pkg/i18nx/locales，前端 web/messages、web/i18n）。
// 统计每个源码文件中「去掉注释后仍含中文的行数」，与基线比较：新增文件或行数增加即失败。
// 用法：node scripts/check-hardcoded-cjk.mjs [--update]
import { readFileSync, readdirSync, statSync, writeFileSync } from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

const CJK = /[㐀-鿿豈-﫿]/
const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..")
const BASELINE_PATH = path.join(ROOT, "scripts", "hardcoded-cjk-baseline.json")

const SCAN_ROOTS = [
  { dir: "cmd", extensions: [".go"] },
  { dir: "internal", extensions: [".go"] },
  { dir: "web/app", extensions: [".ts", ".tsx"] },
  { dir: "web/components", extensions: [".ts", ".tsx"] },
  { dir: "web/lib", extensions: [".ts", ".tsx"] },
  { dir: "web/packages", extensions: [".ts", ".tsx"] },
]

const EXCLUDED_DIRS = new Set(["node_modules", ".next", "dist", "testdata"])
const EXCLUDED_PATHS = [
  "internal/migration/", // 历史迁移按当时数据编写，不再修改
]

export function isScannedFile(relativePath) {
  const normalized = relativePath.split(path.sep).join("/")
  if (EXCLUDED_PATHS.some((prefix) => normalized.startsWith(prefix))) return false
  if (normalized.endsWith("_test.go")) return false
  if (/\.(test|spec)\.tsx?$/.test(normalized)) return false
  if (normalized.endsWith(".d.ts")) return false
  return true
}

// 去掉注释，保留字符串、模板字符串和 JSX 文本。单双引号字符串不跨行，避免文本中的撇号影响后续行。
export function stripComments(source, language) {
  let output = ""
  let state = "code"
  let quote = ""
  for (let index = 0; index < source.length; index += 1) {
    const char = source[index]
    const next = source[index + 1]
    if (state === "line-comment") {
      if (char === "\n") {
        state = "code"
        output += char
      }
      continue
    }
    if (state === "block-comment") {
      if (char === "*" && next === "/") {
        state = "code"
        index += 1
      } else if (char === "\n") {
        output += char
      }
      continue
    }
    if (state === "string") {
      output += char
      const rawString = language === "go" && quote === "`"
      if (char === "\\" && !rawString) {
        if (next !== undefined && next !== "\n") {
          output += next
          index += 1
        }
        continue
      }
      if (char === quote || (char === "\n" && quote !== "`")) {
        state = "code"
      }
      continue
    }
    if (char === "/" && next === "/") {
      state = "line-comment"
      index += 1
      continue
    }
    if (char === "/" && next === "*") {
      state = "block-comment"
      index += 1
      continue
    }
    if (char === '"' || char === "`" || char === "'") {
      state = "string"
      quote = char
    }
    output += char
  }
  return output
}

export function countHardcodedCjkLines(source, language) {
  return stripComments(source, language)
    .split("\n")
    .filter((line) => CJK.test(line)).length
}

function languageOf(filePath) {
  return filePath.endsWith(".go") ? "go" : "ts"
}

function walk(directory, extensions, files) {
  let entries
  try {
    entries = readdirSync(directory)
  } catch {
    return
  }
  for (const entry of entries) {
    if (EXCLUDED_DIRS.has(entry)) continue
    const fullPath = path.join(directory, entry)
    const stats = statSync(fullPath)
    if (stats.isDirectory()) {
      walk(fullPath, extensions, files)
    } else if (extensions.some((extension) => entry.endsWith(extension))) {
      files.push(fullPath)
    }
  }
}

export function collectCounts(root = ROOT) {
  const counts = {}
  for (const { dir, extensions } of SCAN_ROOTS) {
    const files = []
    walk(path.join(root, dir), extensions, files)
    for (const file of files) {
      const relativePath = path.relative(root, file).split(path.sep).join("/")
      if (!isScannedFile(relativePath)) continue
      const count = countHardcodedCjkLines(readFileSync(file, "utf8"), languageOf(file))
      if (count > 0) counts[relativePath] = count
    }
  }
  return Object.fromEntries(Object.entries(counts).sort(([left], [right]) => left.localeCompare(right)))
}

export function compareWithBaseline(counts, baseline) {
  const increased = []
  const decreased = []
  for (const [file, count] of Object.entries(counts)) {
    const allowed = baseline[file] ?? 0
    if (count > allowed) increased.push({ file, count, allowed })
    else if (count < allowed) decreased.push({ file, count, allowed })
  }
  for (const [file, allowed] of Object.entries(baseline)) {
    if (!(file in counts) && allowed > 0) decreased.push({ file, count: 0, allowed })
  }
  return { increased, decreased }
}

function readBaseline() {
  try {
    return JSON.parse(readFileSync(BASELINE_PATH, "utf8"))
  } catch {
    return {}
  }
}

function main(argv) {
  const counts = collectCounts()
  if (argv.includes("--update")) {
    writeFileSync(BASELINE_PATH, `${JSON.stringify(counts, null, 2)}\n`)
    const total = Object.values(counts).reduce((sum, count) => sum + count, 0)
    console.log(`Updated ${path.relative(ROOT, BASELINE_PATH)}: ${Object.keys(counts).length} files, ${total} lines.`)
    return 0
  }
  const { increased, decreased } = compareWithBaseline(counts, readBaseline())
  if (increased.length > 0) {
    console.error("Hardcoded Chinese text increased. Move system text into locale resources:")
    console.error("  backend: internal/pkg/i18nx/locales/*.yml (i18nx.Text / errorsx I18n errors)")
    console.error("  frontend: web/messages/*.json or web/i18n")
    for (const { file, count, allowed } of increased) {
      console.error(`  ${file}: ${count} lines (baseline ${allowed})`)
    }
    return 1
  }
  if (decreased.length > 0) {
    console.log(`Hardcoded Chinese text decreased in ${decreased.length} files. Run with --update to tighten the baseline.`)
  }
  console.log("No new hardcoded Chinese text.")
  return 0
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exitCode = main(process.argv.slice(2))
}
