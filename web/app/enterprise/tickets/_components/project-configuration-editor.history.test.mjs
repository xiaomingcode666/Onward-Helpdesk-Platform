import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./project-configuration-editor.tsx", import.meta.url), "utf8")
const serviceSource = await readFile(new URL("../../../../../internal/services/project_configuration_service.go", import.meta.url), "utf8")

test("configuration history loads ten versions first and continues on scroll", () => {
  assert.match(serviceSource, /query\.Order\("id DESC"\)\.Limit\(10\)/)
  assert.match(source, /const historySentinel = useRef<HTMLDivElement \| null>\(null\)/)
  assert.match(source, /new IntersectionObserver\(/)
  assert.match(source, /getProjectConfiguration\(beforeID\)/)
  assert.match(source, /setView\(current => \{/)
  assert.doesNotMatch(source, /onClick=\{\(\) => void loadOlder\(\)\}/)
})

test("system-written versions render from their origin in the current locale", async () => {
  const messages = Object.fromEntries(await Promise.all(["zh-CN", "en-US", "es-ES"].map(async (locale) => [
    locale,
    JSON.parse(await readFile(new URL(`../../../../messages/${locale}.json`, import.meta.url), "utf8")).projectConfiguration,
  ])))
  assert.match(serviceSource, /Origin: projectConfigVersionOrigin\(v\)/)
  assert.match(source, /version\.origin === "migration_baseline"\) return pc\("migrationBaselineNote"\)/)
  assert.match(source, /version\.origin === "initial_deployment"\) return pc\("initialDeploymentNote"\)/)
  assert.match(source, /\{pc\("createdBy"\)\}: \{versionCreator\(version\)\}/)
  assert.doesNotMatch(source, /version\.created_by === 0 \? pc\("migrationBaseline"\)/)
  for (const key of ["initialDeployment", "migrationBaselineNote", "initialDeploymentNote", "deploymentTool"]) {
    for (const locale of ["en-US", "es-ES"]) {
      assert.ok(messages[locale][key], `${locale} ${key}`)
      assert.doesNotMatch(messages[locale][key], /[一-鿿]/, `${locale} ${key} contains Chinese`)
    }
    assert.ok(messages["zh-CN"][key], `zh-CN ${key}`)
  }
})
