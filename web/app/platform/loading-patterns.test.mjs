import assert from "node:assert/strict"
import { readdir, readFile } from "node:fs/promises"
import { extname, join } from "node:path"
import test from "node:test"
import { fileURLToPath } from "node:url"

const webRoot = fileURLToPath(new URL("../../", import.meta.url))
const ignoredDirectories = new Set([
  ".next",
  "android",
  "coverage",
  "ios",
  "node_modules",
  "out",
])
const checkedExtensions = new Set([".js", ".jsx", ".mjs", ".ts", ".tsx"])

async function collectSourceFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true })
  const files = []
  for (const entry of entries) {
    if (entry.isDirectory()) {
      if (!ignoredDirectories.has(entry.name)) {
        files.push(...await collectSourceFiles(join(directory, entry.name)))
      }
      continue
    }
    if (entry.isFile() && checkedExtensions.has(extname(entry.name)) && !entry.name.includes(".test.")) {
      files.push(join(directory, entry.name))
    }
  }
  return files
}

const loadingFlag = String.raw`(?:loading|isLoading|isPending|[A-Za-z_$][\w$]*Loading)`
const dataPath = String.raw`data(?:\.[A-Za-z_$][\w$]*)*`
const loadingDataGatePatterns = [
  new RegExp(String.raw`${loadingFlag}\s*&&\s*!data\b`),
  new RegExp(String.raw`!data\b\s*&&\s*${loadingFlag}`),
  new RegExp(String.raw`${loadingFlag}\s*&&\s*\(?\s*${dataPath}\s*={2,3}\s*(?:null|undefined)`),
  new RegExp(String.raw`${dataPath}\s*={2,3}\s*(?:null|undefined)\s*\)?\s*&&\s*${loadingFlag}`),
  new RegExp(String.raw`${loadingFlag}\s*&&\s*${dataPath}\.length\s*={2,3}\s*0`),
  new RegExp(String.raw`${dataPath}\.length\s*={2,3}\s*0\s*&&\s*${loadingFlag}`),
]

test("frontend avoids data-gated whole-page loading", async () => {
  const files = await collectSourceFiles(webRoot)
  const matches = []
  for (const file of files) {
    const source = await readFile(file, "utf8")
    for (const pattern of loadingDataGatePatterns) {
      if (pattern.test(source)) {
        matches.push(file)
        break
      }
    }
  }
  assert.deepEqual(matches, [])
})

test("frontend avoids the legacy platform page loader", async () => {
  const files = await collectSourceFiles(webRoot)
  const legacyName = "Platform" + "Loading"
  const matches = []
  for (const file of files) {
    const source = await readFile(file, "utf8")
    if (source.includes(legacyName)) {
      matches.push(file)
    }
  }
  assert.deepEqual(matches, [])
})
