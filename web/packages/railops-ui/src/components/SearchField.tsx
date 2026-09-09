import { SearchOutlined } from '@ant-design/icons';
import { Input } from 'antd';
import type { InputProps, InputRef } from 'antd';
import { forwardRef } from 'react';

export type SearchFieldProps = Omit<InputProps, 'prefix'>;

export const SearchField = forwardRef<InputRef, SearchFieldProps>(function SearchField(props, ref) {
  const { className, ...rest } = props;
  return <Input ref={ref} {...rest} className={['railops-search-field', className].filter(Boolean).join(' ')} prefix={<SearchOutlined aria-hidden="true" />} />;
});
