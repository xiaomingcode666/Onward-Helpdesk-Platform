import { Breadcrumb, Typography } from 'antd';
import type { BreadcrumbProps } from 'antd';
import type { ReactNode } from 'react';

const { Title, Text } = Typography;

export interface PageShellProps {
  title: string;
  description?: ReactNode;
  breadcrumb?: BreadcrumbProps['items'];
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}

// 仅当面包屑为真实层级（>=2 段）时才渲染：单段面包屑与页面标题同义，叠显会形成重复视觉。
const renderableBreadcrumb = (items: BreadcrumbProps['items']) => (Array.isArray(items) && items.length > 1 ? items : undefined);

export const PageShell = ({ title, description, breadcrumb, actions, children, className }: PageShellProps) => { const crumbItems = renderableBreadcrumb(breadcrumb); return <div className={['railops-page-shell', className].filter(Boolean).join(' ')}><header className="railops-page-shell-header"> <div className="railops-page-shell-heading">{crumbItems && <Breadcrumb items={crumbItems} />}<Title level={2}>{title}</Title>{description && <Text type="secondary">{description}</Text>}</div>{actions && <div className="railops-page-shell-actions">{actions}</div>}</header><main className="railops-page-shell-content">{children}</main></div> };
