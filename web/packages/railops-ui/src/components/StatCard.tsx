import { Statistic } from 'antd';
import type { ReactNode } from 'react';

export type StatCardTone = 'blue' | 'success' | 'warning' | 'error' | 'neutral';

export interface StatCardProps {
  label: ReactNode;
  value: string | number;
  note?: ReactNode;
  icon?: ReactNode;
  tone?: StatCardTone;
  className?: string;
}

export const StatCard = ({ label, value, note, icon, tone = 'blue', className }: StatCardProps) => <section className={['railops-stat-card', `railops-stat-card-${tone}`, className].filter(Boolean).join(' ')}><div className="railops-stat-card-label"><span>{label}</span>{icon && <span className="railops-stat-card-icon">{icon}</span>}</div><Statistic value={value} /><div className="railops-stat-card-note">{note}</div></section>;

export interface StatGridProps {
  children: ReactNode;
  className?: string;
}

export const StatGrid = ({ children, className }: StatGridProps) => <div className={['railops-stat-grid', className].filter(Boolean).join(' ')}>{children}</div>;
