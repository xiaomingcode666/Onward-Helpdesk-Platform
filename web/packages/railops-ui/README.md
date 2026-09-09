# @railops/ui

RailOps spec component package built on React 18, TypeScript, Ant Design 5, and ECharts. It is not a business framework and contains no API, permission, routing, or mock data; it provides unified tokens, theming, component default styles, and extension entry points for complex scenarios.

## Installation

```bash
pnpm add @railops/ui antd @ant-design/icons echarts echarts-for-react
```

RemoteHelpDesk consumes this package as a pnpm workspace member: `package.json` points at `src` and Next compiles it via `transpilePackages`; when publishing a standalone npm/tgz package, run the build and switch to the `dist` output.

Business projects only need to import the theme and styles; the `@railops/ui` root entry only exports components and tokens and does not inject global CSS automatically:

```tsx
import { ConfigProvider } from 'antd';
import { DataTable, PageShell, SearchField, railopsTheme } from '@railops/ui';
import '@railops/ui/styles.css';

export function AppRoot() {
  return <ConfigProvider theme={railopsTheme}>{/* Router and pages */}</ConfigProvider>;
}
```

## Component layers

- Foundation: `railopsPalette`,`railopsTokens`,`typographyTokens`,`spacingTokens`,`shadowTokens`,`railopsTheme`.
- Layout: `RailopsAppLayout`,`SidebarNavigation`,`TopHeader`,`PageShell`,`ContentModule`.
- Actions: `RailopsButton`,`IconButton`,`SearchField`,`FilterTabs`,`UnderlineTabs`,`StatusTag`.
- Forms: `FormField`,`SelectField`,`CheckboxField`,`RadioField`,`FormActions`.
- Data: `StatCard`,`ChartContainer`,`TableToolbar`,`TableFilters`,`DataTable`,`TablePagination`.
- Feedback: `FeedbackAlert`,`StandardModal`,`ConfirmModal`,`DetailDrawer`,`LoadingState`,`EmptyState`,`ErrorState`,`ForbiddenState`.

## Extending for complex scenarios

Common pages should prefer RailOps components; complex business logic can pass through native Ant Design props without building new components:

```tsx
<DataTable
  columns={columns}
  dataSource={rows}
  expandable={{ expandedRowRender: (record) => <SubTable record={record} /> }}
  rowSelection={rowSelection}
  total={total}
  current={page}
  onPageChange={setPage}
/>
```

The package standardizes headers, pagination, search, status, border radius, shadows, and spacing; business code owns columns, data, permissions, and API calls. Using Ant Design directly is allowed, but it must inherit `railopsTheme` — core visual tokens must not be overridden.

## Components not covered by the spec (keep list)

The following shadcn/antd components have no spec replacement and business pages may keep them (the exclusion list for the migration cleanup):

- **Keep shadcn, no spec equivalent**: `Avatar`,`Switch`,`Calendar`,`Popover`,`DropdownMenu`,`Combobox`,`Collapsible`,`Separator`,`Resizable`,`Toggle`/`ToggleGroup`,`Command`,`ContextMenu`,`Menubar`,`ScrollArea`.
- **Keep antd native for decorative scenarios**: `Checkbox` (e.g. display-only checkboxes inside Popover lists).
- Page-level conventions: tables uniformly use `DataTable` (pagination via `total/current/pageSize/onPageChange` or the standalone `TablePagination`; the caller manages state and explicitly preserves the original `pageSize`); page shells use `PageShell` + `useRouteBreadcrumbItems()`; inline skeletons use antd `Skeleton.Node` (width/height via `style`).

## Subpaths

```tsx
import { railopsPalette, shadowTokens } from '@railops/ui/tokens';
import { ChartContainer, railopsChartTheme } from '@railops/ui/charts';
```

## Packaging and delivery

```bash
cd packages/railops-ui
pnpm build
pnpm pack
```

Deliver `*.tgz`, `dist`, `package.json`, and this README; no need to commit `node_modules`. Business projects install it via `pnpm add ./railops-ui-1.0.0.tgz`.
