import { Tag } from 'antd';
import type { TagProps } from 'antd';

export type StatusTagTone = 'blue' | 'success' | 'warning' | 'error' | 'neutral' | 'disabled';

export interface StatusTagProps extends Omit<TagProps, 'color'> {
  tone?: StatusTagTone;
}

export const StatusTag = ({ tone = 'neutral', className, ...props }: StatusTagProps) => <Tag {...props} className={['railops-status-tag', `railops-status-tag-${tone}`, className].filter(Boolean).join(' ')} />;
