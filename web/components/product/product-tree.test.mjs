import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./product-tree.tsx", import.meta.url), "utf8")
const styles = await readFile(new URL("../../styles/main.scss", import.meta.url), "utf8")

test("directory pinned scopes align with the product directory root", () => {
  assert.match(source, /ent-product-tree-group-row ent-product-tree-pinned-row/)
  assert.match(source, /ent-product-tree-toggle ent-product-tree-toggle-spacer/)
  assert.match(source, /className=\{cn\("ent-product-tree-group-button", item\.selected && "active"\)\}/)
  assert.match(source, /ent-product-tree-pinned-meta/)
})

test("product tree uses a visible display label instead of raw product.name", () => {
  assert.match(source, /export function getProductTreeDisplayName\(product: ProductListItem\)/)
  assert.match(source, /const productLabel = getProductTreeDisplayName\(product\)/)
  assert.doesNotMatch(source, /<span className="ent-product-list-name">\{product\.name\}<\/span>/)
})

test("product tree metadata cannot squeeze product names out of view", () => {
  assert.match(styles, /\.ent-product-tree-children \.ent-product-list-name\s*\{[\s\S]*?grid-column: 1;/)
  assert.match(styles, /\.ent-product-tree-children \.ent-product-list-meta,[\s\S]*?\.ent-product-tree-children \.kb-product-code\s*\{[\s\S]*?grid-column: 1;/)
  assert.match(styles, /\.ent-product-tree-children \.ent-product-list-badge\s*\{[\s\S]*?grid-column: 2;[\s\S]*?grid-row: 1 \/ span 2;/)
})
