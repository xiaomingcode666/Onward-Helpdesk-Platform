import { Alert, Empty, Modal, Spin } from 'antd';
import type { AlertProps, ModalProps } from 'antd';
import { ExclamationCircleFilled, LockOutlined } from '@ant-design/icons';
import type { ReactNode } from 'react';
import { RailopsButton } from './Button';
import { type RailopsLocaleText, useRailopsLocaleText } from './Locale';
import { StatePanel } from './StatePanel';

export const FeedbackAlert = ({ className, ...props }: AlertProps) => (
  <Alert {...props} showIcon={props.showIcon ?? true} className={['railops-feedback-alert', className].filter(Boolean).join(' ')} />
);

/** 状态面板统一 props：title/description 为文案插槽，action/secondaryAction 为渲染好的按钮节点（消费方可注入 Link 等）。 */
export interface RailopsStateProps {
  title?: ReactNode;
  description?: ReactNode;
  action?: ReactNode;
  secondaryAction?: ReactNode;
  localeText?: Partial<RailopsLocaleText>;
  className?: string;
}

const stateBody = (children: ReactNode, className?: string) => (
  <div className={['railops-state-panel', className].filter(Boolean).join(' ')}><div className="railops-state-body">{children}</div></div>
);

export const LoadingState = ({ description, localeText, className }: { description?: ReactNode; localeText?: Partial<RailopsLocaleText>; className?: string }) => {
  const text = useRailopsLocaleText(localeText);
  return <div className={['railops-state-panel', 'railops-loading-state', className].filter(Boolean).join(' ')}><div className="railops-loading-copy"><Spin /><span>{description ?? text.loading}</span></div></div>;
};

export const EmptyState = ({ title, description, action, secondaryAction, localeText, className }: RailopsStateProps) => {
  const text = useRailopsLocaleText(localeText);
  return stateBody(
    <>
      <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={false} />
      {title ? <div className="railops-state-title">{title}</div> : null}
      {(description ?? text.emptyData) ? <div className="railops-state-description">{description ?? text.emptyData}</div> : null}
      {(action || secondaryAction) ? <div className="railops-state-actions">{action}{secondaryAction}</div> : null}
    </>,
    className,
  );
};

export const ErrorState = ({ title, description, onRetry, action, secondaryAction, localeText, className }: RailopsStateProps & { onRetry?: () => void }) => {
  const text = useRailopsLocaleText(localeText);
  return stateBody(
    <>
      <ExclamationCircleFilled className="railops-state-icon railops-state-icon-error" />
      <div className="railops-state-title">{title ?? text.loadFailed}</div>
      <div className="railops-state-description">{description ?? text.loadFailedDescription}</div>
      {(action || secondaryAction || onRetry) ? <div className="railops-state-actions">{action}{secondaryAction}{onRetry ? <RailopsButton variant="primary" onClick={onRetry}>{text.retry}</RailopsButton> : null}</div> : null}
    </>,
    className,
  );
};

export const ForbiddenState = ({ title, description, action, secondaryAction, localeText, className }: RailopsStateProps) => {
  const text = useRailopsLocaleText(localeText);
  return stateBody(
    <>
      <LockOutlined className="railops-state-icon railops-state-icon-forbidden" />
      <div className="railops-state-title">{title ?? text.forbidden}</div>
      <div className="railops-state-description">{description ?? text.forbiddenDescription}</div>
      {(action || secondaryAction) ? <div className="railops-state-actions">{action}{secondaryAction}</div> : null}
    </>,
    className,
  );
};

export interface StandardModalProps extends ModalProps {
  width?: number | string;
}

export const StandardModal = ({ className, width = 520, ...props }: StandardModalProps) => (
  <Modal {...props} width={width} className={['railops-standard-modal', className].filter(Boolean).join(' ')} />
);

export interface ConfirmModalProps extends StandardModalProps {
  confirmText?: ReactNode;
  cancelText?: ReactNode;
  localeText?: Partial<RailopsLocaleText>;
  danger?: boolean;
}

export const ConfirmModal = ({ confirmText, cancelText, localeText, danger, okText, okButtonProps, cancelButtonProps, ...props }: ConfirmModalProps) => {
  const text = useRailopsLocaleText(localeText);
  return (
    <StandardModal
      {...props}
      okText={okText ?? confirmText ?? text.confirm}
      cancelText={cancelText ?? text.cancel}
      okButtonProps={{ danger, ...okButtonProps }}
      cancelButtonProps={cancelButtonProps}
    />
  );
};

export { StatePanel };
