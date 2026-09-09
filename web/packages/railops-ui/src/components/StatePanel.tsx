import { Button, Empty, Result, Spin } from 'antd';
import type { ReactNode } from 'react';
import { type RailopsLocaleText, useRailopsLocaleText } from './Locale';

export type DataState = 'loading' | 'empty' | 'error' | 'forbidden';

export interface StatePanelProps {
  state: DataState;
  emptyDescription?: ReactNode;
  errorDescription?: ReactNode;
  forbiddenDescription?: ReactNode;
  onRetry?: () => void;
  children?: ReactNode;
  localeText?: Partial<RailopsLocaleText>;
  className?: string;
}

export const StatePanel = ({ state, emptyDescription, errorDescription, forbiddenDescription, onRetry, children, localeText, className }: StatePanelProps) => {
  const text = useRailopsLocaleText(localeText);
  if (state === 'loading') return <div className={['railops-state-panel', 'railops-loading-state', className].filter(Boolean).join(' ')}><div className="railops-loading-copy"><Spin /><span>{text.loading}</span></div></div>;
  if (state === 'empty') return <div className={['railops-state-panel', className].filter(Boolean).join(' ')}><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyDescription ?? text.emptyData} /></div>;
  if (state === 'forbidden') return <div className={['railops-state-panel', className].filter(Boolean).join(' ')}><Result status="403" title={text.forbidden} subTitle={forbiddenDescription ?? text.forbiddenDescription} /></div>;
  if (state === 'error') return <div className={['railops-state-panel', className].filter(Boolean).join(' ')}><Result status="error" title={text.loadFailed} subTitle={errorDescription ?? text.loadFailedDescription} extra={onRetry && <Button type="primary" onClick={onRetry}>{text.retry}</Button>} /></div>;
  return <>{children}</>;
};
