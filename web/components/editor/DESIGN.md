# Unified Editor Design (single TipTap core + dual-format storage)

## 1. Background and goals

The project currently has two kinds of editing needs:

- `markdown`: structured content such as knowledge documents
- `html` rich text: WYSIWYG editing, IM messages, etc.

Today different scenarios use forked implementations, which is costly to maintain and gives an inconsistent interaction model.
The goal of this design is to unify the frontend editor system without breaking the existing backend interface (`contentType + content`).

### Goals

- Unified editing core: all frontend editing scenarios use TipTap/ProseMirror where possible
- Dual-format compatibility: keep `contentType = "markdown" | "html"`
- Keep it evolvable: later extensions such as image upload, drafts, shortcuts, and a pluggable toolbar
- Lower migration risk: replace in phases, never rewrite every page at once

### Non-goals

- No pursuit of absolutely lossless two-way conversion between markdown and html
- No full collaborative editing (OT/CRDT) in the first phase

---

## 2. Core conclusion

TipTap can serve as the single editing core, but markdown/html should not be reduced to "fully equivalent formats".

- TipTap's internal model is a ProseMirror document, not a markdown AST
- markdown and html differ semantically; lossless two-way conversion of complex structures is lossy in practice
- The right approach: **unified core + typed storage + controlled conversion**

---

## 3. Overall architecture

Proposed layout (centered on `web/components/editor`):

```text
web/components/editor/
  index.tsx                # unified entry component UnifiedEditor
  html.tsx                 # HtmlEditor (TipTap configuration)
  markdown.tsx             # MarkdownEditor (TipTap markdown mode)
  viewer.tsx               # unified read-only render entry (optional)
  toolbar.tsx              # shared toolbar (sized by capabilities)
  schema.ts                # TipTap extensions and capability groups
  convert.ts               # markdown/html <-> editor doc conversion wrapper
  sanitize.ts              # HTML allowlist sanitizing (before render)
  types.ts                 # unified type definitions
  DESIGN.md                # this document
```

---

## 4. Data model and interface

## 4.1 Type definitions (proposed)

```ts
export type EditorMode = "markdown" | "html"

export type EditorValue = {
  mode: EditorMode
  raw: string
}

export type UnifiedEditorProps = {
  value: EditorValue
  onChange: (next: EditorValue) => void
  placeholder?: string
  disabled?: boolean
  features?: {
    image?: boolean
    link?: boolean
    table?: boolean
    codeBlock?: boolean
  }
}
```

Notes:

- `raw` holds the final persisted content (markdown text or html string)
- Callers no longer care "which editor implementation is used", they only handle `value/onChange`

## 4.2 Backward compatibility

Stays consistent with the current backend interface:

- `contentType` <- `value.mode`
- `content` <- `value.raw`

No backend data structure changes required.

---

## 5. Mode strategy (key section)

## 5.1 html mode

- Import: `setContent(html)`
- Editing: regular TipTap rich text
- Export: `editor.getHTML()`

## 5.2 markdown mode

- Import: `markdown -> editor doc`
- Editing: still uses the TipTap core (with a markdown-friendly toolbar option)
- Export: `editor doc -> markdown`

## 5.3 Mode switching

When the user manually switches `markdown/html`:

1. Show a confirmation prompt: warn that formatting may be lost
2. The user can choose:
   - Switch mode only (keep the original content, no conversion)
   - Run conversion (attempt markdown/html two-way conversion)
3. On conversion failure, fall back and show the error reason

---

## 6. Conversion and boundary rules

## 6.1 Stable conversion subset

In the first phase, only guarantee the following elements convert stably:

- Paragraphs, headings (h1-h3)
- Bold, italic, strikethrough
- Unordered/ordered lists
- Blockquotes
- Inline code, code blocks
- Links
- Images (basic attributes)

## 6.2 Explicitly lossy boundaries

The following capabilities are not promised to round-trip losslessly (may be hinted in the UI):

- Complex tables
- Custom HTML attributes and inline styles
- Arbitrary nested blocks and third-party embedded nodes

---

## 7. Security policy

All HTML rendering must go through sanitize before reaching `dangerouslySetInnerHTML`.

Suggested allowlist:

- Tags: `p`, `br`, `strong`, `em`, `del`, `blockquote`, `ul`, `ol`, `li`, `code`, `pre`, `a`, `img`, `h1`, `h2`, `h3`
- Attributes:
  - `a`: `href`, `target`, `rel`
  - `img`: `src`, `alt`, `title`

Key points:

- Forbid dangerous tags such as `script`, `style`, `iframe`
- Filter event attributes (e.g. `onclick`)
- Restrict `href/src` protocols (e.g. only `http`, `https`, and `data:image/*` as needed)

---

## 8. Suggested component layering

Keep the following layers so editing logic does not leak into pages:

- `UnifiedEditor`: mode dispatch, shared props, unified events
- `HtmlEditor` / `MarkdownEditor`: each implementation's details
- `EditorToolbar`: buttons toggled by the `features` prop
- `convert.ts`: content conversion only, no UI mixed in
- `viewer.tsx`: read-only rendering, unified sanitize + styling

---

## 9. Phased implementation plan

## Phase 1 (low-risk unified entry)

- Add minimal implementations of `index.tsx`, `html.tsx`, `markdown.tsx` under `web/components/editor`
- First migrate the knowledge document editing page to `UnifiedEditor`
- Leave IM scenarios untouched for now to avoid one large change

Acceptance criteria:

- Business pages no longer branch directly between `Textarea` and `RichTextEditor`
- Saved results are fully compatible with the current interface

## Phase 2 (converge rich-text capabilities)

- Abstract the TipTap schema/toolbar into reusable capabilities
- Migrate the IM editor to the unified core configuration (keeping its send shortcut and image upload behavior)

Acceptance criteria:

- Shared core extensions and styling strategy
- Scenario differences are controlled by the `features` toggle

## Phase 3 (experience and reliability)

- Introduce auto-save drafts (localStorage or server-side drafts)
- Unify shortcuts, word count, and paste rules
- Harden the sanitize policy and regression tests

---

## 10. Testing suggestions

Cover at least the following scenarios:

- markdown/html editing and saving independently
- Mode-switch prompt and conversion-failure fallback
- Image upload placeholder replacement success/failure
- HTML render safety (XSS cases)
- Key shortcut behavior (Enter/Shift+Enter/Cmd+B)

---

## 11. Risks and trade-offs

- Risk: pursuing "lossless conversion of all formats" would sharply increase implementation complexity
- Trade-off: first define a "stably supported syntax subset"; handle the rest with hints plus graceful degradation
- Payoff: with a unified core, later features (drafts, plugins, statistics, theming) are built once and reused everywhere

---

## 12. Direct mapping to the current project

Prioritize replacing the branching logic in the knowledge document editing page (markdown textarea vs html rich text) with a single `UnifiedEditor` integration.
The IM editor can migrate to the same core configuration in the next phase, without breaking the existing send interaction.
