import type { ChangeEvent, ReactNode } from 'react';
import { formatRailopsText, useRailopsLocaleText } from './Locale';
import { SearchField } from './SearchField';
import { StatusTag } from './StatusTag';

export interface SideMenuItem {
  key: string;
  label: string;
  count?: number;
  disabled?: boolean;
}

export interface SideMenuWorkspaceProps {
  title?: ReactNode;
  totalLabel?: ReactNode;
  searchValue: string;
  searchPlaceholder?: string;
  onSearchChange: (value: string) => void;
  items: SideMenuItem[];
  selectedKey: string;
  onSelect: (key: string) => void;
  children: ReactNode;
  className?: string;
}

export const SideMenuWorkspace = ({ title, totalLabel, searchValue, searchPlaceholder, onSearchChange, items, selectedKey, onSelect, children, className }: SideMenuWorkspaceProps) => {
  const text = useRailopsLocaleText();
  return <div className={['railops-side-menu-workspace', className].filter(Boolean).join(' ')}><aside className="railops-side-menu"><div className="railops-side-menu-heading"><strong>{title ?? text.product}</strong>{totalLabel && <span>{totalLabel}</span>}</div><SearchField value={searchValue} placeholder={searchPlaceholder ?? text.searchProduct} allowClear onChange={(event: ChangeEvent<HTMLInputElement>) => onSearchChange(event.target.value)} /><div className="railops-side-menu-list" role="list">{items.map((item) => <button type="button" className={`railops-side-menu-item ${selectedKey === item.key ? 'is-active' : ''}`} aria-pressed={selectedKey === item.key} disabled={item.disabled} key={item.key} onClick={() => onSelect(item.key)}><span>{item.label}</span>{item.count !== undefined && <StatusTag tone={item.count > 0 ? 'success' : 'neutral'}>{formatRailopsText(text.unitDevice, { count: item.count })}</StatusTag>}</button>)}</div></aside><section className="railops-side-menu-content">{children}</section></div>;
};
