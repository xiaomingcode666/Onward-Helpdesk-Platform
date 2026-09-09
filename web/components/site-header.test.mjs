import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { describe, it } from "node:test";

const source = await readFile(new URL("./site-header.tsx", import.meta.url), "utf8");

describe("site header preferences", () => {
  it("renders the palette control without locale or theme switchers", () => {
    assert.doesNotMatch(source, /LocaleSwitcher/);
    assert.match(source, /import \{ PaletteToggle \} from "@\/components\/palette-toggle"/);
    assert.doesNotMatch(source, /ThemeToggle/);
    assert.match(source, /<PaletteToggle \/>/);
  });

  it("uses the shared header breadcrumb treatment", () => {
    assert.match(source, /import \{ HeaderCrumbs \} from "@\/components\/layout\/header-crumbs"/);
    assert.match(source, /getPageBreadcrumbKeys/);
    assert.doesNotMatch(source, /BreadcrumbPage/);
  });
});
