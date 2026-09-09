import { CheckOutlined, FilterOutlined } from '@ant-design/icons';
import { Button, Checkbox, Dropdown, Form, Radio, Select, Space, Tag } from 'antd';
import type { MenuProps, RadioGroupProps, SelectProps } from 'antd';
import type { ReactNode } from 'react';
import { useEffect, useState } from 'react';
import { RailopsButton } from './Button';
import { useRailopsLocaleText } from './Locale';

export type TableFilterValue = string | string[] | boolean | undefined;
export type TableFilterValues = Record<string, TableFilterValue>;
export type TableFilterFieldType = 'select' | 'multiSelect' | 'radio' | 'checkbox';

export interface TableFilterField {
  key: string;
  label: string;
  type?: TableFilterFieldType;
  options?: SelectProps['options'];
}

export interface TableFiltersProps {
  fields: TableFilterField[];
  value: TableFilterValues;
  onChange: (value: TableFilterValues) => void;
  onApply?: (value: TableFilterValues) => void;
  onReset?: () => void;
  triggerText?: ReactNode;
  className?: string;
}

const hasValue = (value: TableFilterValue) => Array.isArray(value) ? value.length > 0 : value !== undefined && value !== '' && value !== false;

export const TableFilters = ({ fields, value, onChange, onApply, onReset, triggerText, className }: TableFiltersProps) => {
  const text = useRailopsLocaleText();
  const [draft, setDraft] = useState<TableFilterValues>(value);
  useEffect(() => setDraft(value), [value]);
  const count = fields.filter((field) => hasValue(value[field.key])).length;
  const update = (key: string, nextValue: TableFilterValue) => setDraft((current) => ({ ...current, [key]: nextValue }));
  const apply = () => { onChange(draft); onApply?.(draft); };
  const reset = () => { setDraft({}); onChange({}); onReset?.(); };
  const content = <div className="railops-table-filter-menu"><Form layout="vertical">{fields.map((field) => <Form.Item label={field.label} key={field.key}>{field.type === 'checkbox' ? <Checkbox checked={Boolean(draft[field.key])} onChange={(event) => update(field.key, event.target.checked)}>{text.yes}</Checkbox> : field.type === 'radio' ? <Radio.Group value={draft[field.key]} options={field.options as RadioGroupProps['options']} onChange={(event) => update(field.key, event.target.value)} /> : <Select allowClear mode={field.type === 'multiSelect' ? 'multiple' : undefined} value={draft[field.key] as string | string[] | undefined} options={field.options} onChange={(nextValue) => update(field.key, nextValue)} style={{ width: '100%' }} />}</Form.Item>)}</Form><div className="railops-table-filter-actions"><RailopsButton onClick={reset}>{text.reset}</RailopsButton><RailopsButton variant="primary" onClick={apply} icon={<CheckOutlined />}>{text.applyFilters}</RailopsButton></div></div>;
  return <Dropdown trigger={['click']} popupRender={() => content}><Button className={['railops-table-filter-trigger', className].filter(Boolean).join(' ')} icon={<FilterOutlined />}>{triggerText ?? text.filter}{count > 0 && <Tag className="railops-table-filter-count">{count}</Tag>}</Button></Dropdown>;
};

export interface ActiveFilterTagsProps { fields: TableFilterField[]; value: TableFilterValues; onChange: (value: TableFilterValues) => void; onReset?: () => void; }

export const ActiveFilterTags = ({ fields, value, onChange, onReset }: ActiveFilterTagsProps) => {
  const text = useRailopsLocaleText();
  const active = fields.filter((field) => hasValue(value[field.key]));
  if (active.length === 0) return null;
  const remove = (key: string) => { const next = { ...value }; delete next[key]; onChange(next); };
  return <div className="railops-active-filter-tags"><span>{text.filtered}</span>{active.map((field) => <Tag closable key={field.key} onClose={() => remove(field.key)}>{field.label}</Tag>)}{onReset && <Button type="link" size="small" onClick={onReset}>{text.resetAll}</Button>}</div>;
};

export type TableFilterMenuItem = NonNullable<MenuProps['items']>[number];
