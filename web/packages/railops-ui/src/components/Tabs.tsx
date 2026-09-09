import { Button } from 'antd';
import type { ReactNode } from 'react';
import { useRailopsLocaleText } from './Locale';

export interface RailopsTabItem {
  label: string;
  value: string;
  icon?: ReactNode;
  count?: number;
  disabled?: boolean;
}

export interface FilterTabsProps {
  items: RailopsTabItem[];
  value: string;
  onChange: (value: string) => void;
  ariaLabel?: string;
}

export const FilterTabs = ({ items, value, onChange, ariaLabel }: FilterTabsProps) => {
  const text = useRailopsLocaleText();
  return <div className="railops-filter-tabs" role="tablist" aria-label={ariaLabel ?? text.filterOptions}>{items.map((item) => <Button key={item.value} type="text" role="tab" aria-selected={value === item.value} disabled={item.disabled} className={`railops-filter-tab ${value === item.value ? 'is-active' : ''}`} icon={item.icon} onClick={() => onChange(item.value)}>{item.label}{item.count !== undefined && <span className="railops-tab-count">{item.count}</span>}</Button>)}</div>;
};

export interface UnderlineTabsProps extends FilterTabsProps {
  animated?: boolean;
}

export const UnderlineTabs = ({ items, value, onChange, ariaLabel, animated = true }: UnderlineTabsProps) => {
  const text = useRailopsLocaleText();
  const activeIndex = Math.max(0, items.findIndex((item) => item.value === value));
  return <div className={`railops-underline-tabs ${animated ? '' : 'is-static'}`} role="tablist" aria-label={ariaLabel ?? text.moduleTabs} style={{ '--railops-tab-count': items.length, '--railops-active-index': activeIndex } as React.CSSProperties}><div className="railops-underline-tab-list">{items.map((item) => <Button key={item.value} type="text" role="tab" aria-selected={value === item.value} disabled={item.disabled} className={`railops-underline-tab ${value === item.value ? 'is-active' : ''}`} icon={item.icon} onClick={() => onChange(item.value)}>{item.label}{item.count !== undefined && <span className="railops-tab-count">{item.count}</span>}</Button>)}</div><span className="railops-underline-tab-indicator" aria-hidden="true" /></div>;
};
