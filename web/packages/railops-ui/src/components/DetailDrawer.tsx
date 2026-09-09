import { Drawer } from 'antd';
import type { DrawerProps } from 'antd';

export const DetailDrawer = ({ className, rootClassName, width = '40vw', ...props }: DrawerProps) => <Drawer {...props} width={width} className={['railops-detail-drawer', className].filter(Boolean).join(' ')} rootClassName={['railops-detail-drawer-root', rootClassName].filter(Boolean).join(' ')} />;
