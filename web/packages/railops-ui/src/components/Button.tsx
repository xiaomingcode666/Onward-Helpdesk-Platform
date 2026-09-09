import { Button, Tooltip } from 'antd';
import type { ButtonProps } from 'antd';
import type { ReactNode } from 'react';

export type RailopsButtonVariant = 'primary' | 'default' | 'dashed' | 'text' | 'link';

export interface RailopsButtonProps extends Omit<ButtonProps, 'type' | 'variant'> {
  variant?: RailopsButtonVariant;
}

export const RailopsButton = ({ variant = 'default', className, ...props }: RailopsButtonProps) => (
  <Button
    {...props}
    type={variant}
    className={['railops-button', `railops-button-${variant}`, className].filter(Boolean).join(' ')}
  />
);

export interface IconButtonProps extends Omit<RailopsButtonProps, 'children' | 'icon'> {
  icon: ReactNode;
  tooltip?: ReactNode;
  'aria-label': string;
}

export const IconButton = ({ icon, tooltip, className, 'aria-label': ariaLabel, ...props }: IconButtonProps) => {
  const button = <RailopsButton {...props} aria-label={ariaLabel} icon={icon} className={['railops-icon-button', className].filter(Boolean).join(' ')} />;
  return tooltip ? <Tooltip title={tooltip}>{button}</Tooltip> : button;
};
