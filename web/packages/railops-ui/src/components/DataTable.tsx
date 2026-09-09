import { Empty, Pagination, Table, Typography } from 'antd';
import type { PaginationProps, TableProps } from 'antd';
import type { ReactNode } from 'react';
import { formatRailopsText, type RailopsLocaleText, useRailopsLocaleText } from './Locale';

const { Text } = Typography;

/** 统一表格每页条数：所有使用 DataTable / TablePagination 的页面默认一页仅显示该条数，保持整体样式一致。 */
export const DEFAULT_PAGE_SIZE = 10;

export interface DataTableProps<RecordType extends object> extends TableProps<RecordType> {
  total?: number;
  current?: number;
  pageSize?: number;
  onPageChange?: (page: number, pageSize: number) => void;
  paginationProps?: Omit<PaginationProps, 'current' | 'pageSize' | 'total' | 'onChange'>;
  footerNote?: ReactNode;
  emptyDescription?: ReactNode;
  localeText?: Partial<RailopsLocaleText>;
  className?: string;
}

export const DataTable = <RecordType extends object>({ total, current = 1, pageSize = DEFAULT_PAGE_SIZE, onPageChange, paginationProps, footerNote, emptyDescription, localeText, className, pagination, locale, ...props }: DataTableProps<RecordType>) => {
  const text = useRailopsLocaleText(localeText);
  return <div className="railops-data-table"><Table<RecordType> {...props} className={['railops-table', className].filter(Boolean).join(' ')} pagination={pagination ?? false} locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyDescription ?? text.emptyData} />, ...locale }} />{total !== undefined && <TablePagination current={current} pageSize={pageSize} total={total} onChange={onPageChange} footerNote={footerNote} localeText={localeText} {...paginationProps} />}</div>;
};

export interface TablePaginationProps extends Omit<PaginationProps, 'current' | 'pageSize' | 'total' | 'onChange'> {
  current?: number;
  pageSize?: number;
  total: number;
  onChange?: (page: number, pageSize: number) => void;
  footerNote?: ReactNode;
  localeText?: Partial<RailopsLocaleText>;
}

export const TablePagination = ({ current = 1, pageSize = DEFAULT_PAGE_SIZE, total, onChange, footerNote, localeText, ...props }: TablePaginationProps) => {
  const text = useRailopsLocaleText(localeText);
  return <footer className="railops-data-table-footer"><Text type="secondary">{footerNote ?? formatRailopsText(text.paginationTotal, { total })}</Text><Pagination size="small" current={current} pageSize={pageSize} total={total} showSizeChanger={false} onChange={onChange} {...props} /></footer>;
};
