import type { ReactNode } from 'react';

export interface TableToolbarProps {
  tabs?: ReactNode;
  search?: ReactNode;
  filters?: ReactNode;
  actions?: ReactNode;
  activeFilters?: ReactNode;
  className?: string;
}

export const TableToolbar = ({ tabs, search, filters, actions, activeFilters, className }: TableToolbarProps) => <div className={['railops-table-toolbar-wrap', className].filter(Boolean).join(' ')}><div className="railops-table-toolbar">{tabs && <div className="railops-table-toolbar-tabs">{tabs}</div>}<div className="railops-table-toolbar-actions">{search}{filters}{actions}</div></div>{activeFilters && <div className="railops-table-toolbar-active-filters">{activeFilters}</div>}</div>;
