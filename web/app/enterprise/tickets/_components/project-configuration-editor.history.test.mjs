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
