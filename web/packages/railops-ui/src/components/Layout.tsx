import { MenuFoldOutlined, MenuUnfoldOutlined } from '@ant-design/icons';
import { Breadcrumb, Button, Layout as AntLayout, Menu, Typography } from 'antd';
import type { BreadcrumbProps, MenuProps } from 'antd';
import type { ReactNode } from 'react';
import { IconButton } from './Button';
import { useRailopsLocaleText } from './Locale';

const { Header, Sider, Content } = AntLayout;

export interface SidebarNavigationProps {
  items: MenuProps['items'];
  selectedKeys?: string[];
  openKeys?: string[];
  collapsed?: boolean;
  onCollapsedChange?: (collapsed: boolean) => void;
  onClick?: MenuProps['onClick'];
  onOpenChange?: MenuProps['onOpenChange'];
  brand?: ReactNode;
  footer?: ReactNode;
  width?: number;
  collapsedWidth?: number;
  className?: string;
}

export const SidebarNavigation = ({ items, selectedKeys, openKeys, collapsed = false, onCollapsedChange, onClick, onOpenChange, brand, footer, width = 240, collapsedWidth = 64, className }: SidebarNavigationProps) => {
  const text = useRailopsLocaleText();
  const collapseLabel = collapsed ? text.expandSidebar : text.collapseSidebar;
  return (
    <Sider width={width} collapsedWidth={collapsedWidth} collapsed={collapsed} trigger={null} className={['railops-sidebar', className].filter(Boolean).join(' ')}>
      {brand && <div className="railops-sidebar-brand">{brand}</div>}
      <div className="railops-sidebar-menu"><Menu mode="inline" inlineIndent={16} triggerSubMenuAction="click" subMenuOpenDelay={0} subMenuCloseDelay={0} items={items} selectedKeys={selectedKeys} openKeys={collapsed ? [] : openKeys} onClick={onClick} onOpenChange={onOpenChange} /></div>
      <div className="railops-sidebar-footer">
        {onCollapsedChange && <IconButton icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />} tooltip={collapseLabel} aria-label={collapseLabel} onClick={() => onCollapsedChange(!collapsed)} />}
        {footer && !collapsed && <div className="railops-sidebar-footer-content">{footer}</div>}
      </div>
    </Sider>
  );
};

export interface TopHeaderProps {
  breadcrumb?: BreadcrumbProps['items'];
  actions?: ReactNode;
  children?: ReactNode;
  className?: string;
}

export const TopHeader = ({ breadcrumb, actions, children, className }: TopHeaderProps) => (
  <Header className={['railops-top-header', className].filter(Boolean).join(' ')}>
    <div className="railops-top-header-left">{breadcrumb && <Breadcrumb items={breadcrumb} />}{children}</div>
    {actions && <div className="railops-top-header-actions">{actions}</div>}
  </Header>
);

export const PageBreadcrumb = (props: BreadcrumbProps) => <Breadcrumb {...props} className={['railops-page-breadcrumb', props.className].filter(Boolean).join(' ')} />;

export interface RailopsAppLayoutProps {
  sidebar: Omit<SidebarNavigationProps, 'className'>;
  header?: Omit<TopHeaderProps, 'className'>;
  children: ReactNode;
  className?: string;
}

export const RailopsAppLayout = ({ sidebar, header, children, className }: RailopsAppLayoutProps) => (
  <AntLayout className={['railops-app-layout', className].filter(Boolean).join(' ')}>
    <SidebarNavigation {...sidebar} />
    <AntLayout>
      {header && <TopHeader {...header} />}
      <Content className="railops-app-content">{children}</Content>
    </AntLayout>
  </AntLayout>
);

export const LayoutNote = ({ children }: { children: ReactNode }) => <Typography.Text type="secondary">{children}</Typography.Text>;
