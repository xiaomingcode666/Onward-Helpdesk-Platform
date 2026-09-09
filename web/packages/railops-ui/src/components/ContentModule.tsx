import type { ReactNode } from 'react';

export interface ContentModuleProps {
  title?: ReactNode;
  note?: ReactNode;
  extra?: ReactNode;
  children: ReactNode;
  className?: string;
}

export const ContentModule = ({ title, note, extra, children, className }: ContentModuleProps) => <section className={['railops-content-module', className].filter(Boolean).join(' ')}>{(title || note || extra) && <header className="railops-content-module-header"><div className="railops-content-module-title">{title && <strong>{title}</strong>}{note && <span>{note}</span>}</div>{extra}</header>}<div className="railops-content-module-body">{children}</div></section>;
