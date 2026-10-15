#!/usr/bin/env node
// 校验前端多语言资源：代码里写死的翻译 key 必须在 zh-CN 和 en-US 中都存在，且两种语言的 key 集合一致。
// 缺失的 key 会在界面上显示 key 原文，或在英文界面回退成中文。
// 用法：node scripts/check-i18n-keys.mjs
import { readFileSync, readdirSync, statSync } from "node:fs"
import path from "node:path"
import vm from "node:vm"
import { createRequire } from "node:module"
import { fileURLToPath } from "node:url"

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..")
const WEB = path.join(ROOT, "web")
const REQUIRED_LOCALES = ["zh-CN", "en-US"]
const SCAN_DIRS = ["app", "components", "lib", "hooks"]

function flatten(source, prefix = "", output = new Map()) {
  if (source && typeof source === "object" && !Array.isArray(source)) {
    for (const [key, value] of Object.entries(source)) {
      flatten(value, prefix ? `${prefix}.${key}` : key, output)
    }
  } else {
    output.set(prefix, source)
  }
  return output
}

function loadExtractedGroups() {
  const ts = createRequire(path.join(WEB, "package.json"))("typescript")
  const directory = path.join(WEB, "i18n", "extracted")
  const groups = []
  for (const entry of readdirSync(directory).sort()) {
    if (!entry.endsWith(".ts") || entry === "index.ts" || entry === "types.ts") continue
    const compiled = ts.transpileModule(readFileSync(path.join(directory, entry), "utf8"), {
      compilerOptions: { target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.CommonJS },
      fileName: entry,
    }).outputText
    const sandbox = { module: { exports: {} }, require: () => ({}) }
    sandbox.exports = sandbox.module.exports
    vm.runInNewContext(compiled, sandbox)
    for (const value of Object.values(sandbox.module.exports)) {
      if (value && typeof value === "object" && "zh-CN" in value) groups.push({ name: entry, messages: value })
    }
  }
  return groups
}

export function loadCatalogs() {
  const catalogs = {}
  const sources = [{ name: "messages", messages: {} }, ...loadExtractedGroups()]
  for (const locale of ["zh-CN", "en-US", "es-ES"]) {
    sources[0].messages[locale] = JSON.parse(readFileSync(path.join(WEB, "messages", `${locale}.json`), "utf8"))
  }
  for (const locale of REQUIRED_LOCALES) {
    const keys = new Map()
    for (const source of sources) {
      for (const [key, value] of flatten(source.messages[locale])) {
        if (!keys.has(key)) keys.set(key, { value, source: source.name })
      }
    }
    catalogs[locale] = keys
  }
  return catalogs
}

function prefixesOf(keys) {
  const prefixes = new Set()
  for (const key of keys) {
    const parts = key.split(".")
    for (let index = 1; index < parts.length; index += 1) prefixes.add(parts.slice(0, index).join("."))
  }
  return prefixes
}

const KEY = String.raw`[A-Za-z][\w-]*(?:\.[\w-]+)*`
const DIRECT_TRANSLATORS = ["t", "translate", "translateCurrentMessage", "translateMessage"]

// 找到文件内形如 `prefix.${key}` 的前缀包装函数，返回 函数名 → 前缀。
export function findScopedTranslators(source) {
  const scoped = new Map()
  const declaration = /(?:function\s+([A-Za-z_$][\w$]*)\s*\(|(?:const|let)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:useCallback\(\s*)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)\s*(?::[^=]*)?=>)/g
  for (const match of source.matchAll(declaration)) {
    const name = match[1] || match[2]
    const body = source.slice(match.index, match.index + 400)
    const template = body.match(new RegExp("`(" + KEY + ")\\.\\$\\{key\\}`"))
    if (template) scoped.set(name, template[1])
  }
  return scoped
}

function lineOf(source, index) {
  return source.slice(0, index).split("\n").length
}

export function collectKeyReferences(source) {
  const references = []
  const scoped = findScopedTranslators(source)
  const direct = new RegExp(String.raw`(?<![\w$.])(?:${DIRECT_TRANSLATORS.join("|")})\(\s*(?:[A-Za-z_$][\w$]*\s*,\s*)?["'](${KEY}\.[\w-]+)["']`, "g")
  // `t` 等常用名在文件内被重新定义成前缀包装函数时，按前缀拼出完整 key。
  for (const match of source.matchAll(direct)) {
    const name = match[0].match(/^[\w$]+/)[0]
    const prefix = scoped.get(name)
    references.push({ key: prefix ? `${prefix}.${match[1]}` : match[1], line: lineOf(source, match.index) })
  }
  for (const [name, prefix] of scoped) {
    if (DIRECT_TRANSLATORS.includes(name)) continue
    const call = new RegExp(String.raw`(?<![\w$.])${name.replace(/\$/g, "\\$")}\(\s*(?:[A-Za-z_$][\w$]*\s*,\s*)?["'](${KEY})["']`, "g")
    for (const match of source.matchAll(call)) {
      references.push({ key: `${prefix}.${match[1]}`, line: lineOf(source, match.index) })
    }
  }
  return references
}

function walk(directory, files) {
  for (const entry of readdirSync(directory)) {
    if (entry === "node_modules" || entry.startsWith(".next")) continue
    const fullPath = path.join(directory, entry)
    if (statSync(fullPath).isDirectory()) walk(fullPath, files)
    else if (/\.tsx?$/.test(entry) && !/\.(test|spec)\.tsx?$/.test(entry) && !entry.endsWith(".d.ts")) files.push(fullPath)
  }
  return files
}

function main() {
  const catalogs = loadCatalogs()
  const problems = []

  const zhKeys = catalogs["zh-CN"]
  const enKeys = catalogs["en-US"]
  for (const key of zhKeys.keys()) if (!enKeys.has(key)) problems.push(`en-US missing key present in zh-CN: ${key} (${zhKeys.get(key).source})`)
  for (const key of enKeys.keys()) if (!zhKeys.has(key)) problems.push(`zh-CN missing key present in en-US: ${key} (${enKeys.get(key).source})`)

  const known = REQUIRED_LOCALES.map((locale) => {
    const keys = new Set(catalogs[locale].keys())
    return { locale, keys, prefixes: prefixesOf(keys) }
  })
  for (const file of SCAN_DIRS.flatMap((directory) => walk(path.join(WEB, directory), []))) {
    const source = readFileSync(file, "utf8")
    const relative = path.relative(WEB, file).split(path.sep).join("/")
    for (const { key, line } of collectKeyReferences(source)) {
      for (const { locale, keys, prefixes } of known) {
        if (!keys.has(key) && !prefixes.has(key)) problems.push(`${relative}:${line} ${locale} missing key: ${key}`)
      }
    }
  }

  if (problems.length > 0) {
    console.error(`Missing i18n messages (${problems.length}). Add them to web/messages/*.json or web/i18n/extracted:`)
    for (const problem of problems) console.error(`  ${problem}`)
    process.exitCode = 1
    return
  }
  console.log("i18n keys OK")
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main()
}
