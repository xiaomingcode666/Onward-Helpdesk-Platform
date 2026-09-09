import { Checkbox, Form, Radio, Select } from 'antd';
import type { CheckboxProps, FormItemProps, RadioGroupProps, SelectProps } from 'antd';
import type { ReactNode } from 'react';
import { RailopsButton } from './Button';
import { type RailopsLocaleText, useRailopsLocaleText } from './Locale';

export interface FormFieldProps extends FormItemProps {
  children?: ReactNode;
}

export const FormField = ({ className, children, ...props }: FormFieldProps) => (
  <Form.Item {...props} className={['railops-form-field', className].filter(Boolean).join(' ')}>
    {children}
  </Form.Item>
);

export interface SelectFieldProps extends Omit<FormItemProps, 'children'> {
  selectProps?: SelectProps;
}

export const SelectField = ({ className, selectProps, ...itemProps }: SelectFieldProps) => (
  <Form.Item {...itemProps} className={['railops-form-field', className].filter(Boolean).join(' ')}>
    <Select {...selectProps} />
  </Form.Item>
);

export interface CheckboxFieldProps extends Omit<FormItemProps, 'children'> {
  checkboxProps?: CheckboxProps;
  children?: ReactNode;
}

export const CheckboxField = ({ className, checkboxProps, children, ...itemProps }: CheckboxFieldProps) => (
  <Form.Item {...itemProps} className={['railops-form-field', className].filter(Boolean).join(' ')}>
    <Checkbox {...checkboxProps}>{children}</Checkbox>
  </Form.Item>
);

export interface RadioFieldProps extends Omit<FormItemProps, 'children'> {
  radioProps?: RadioGroupProps;
}

export const RadioField = ({ className, radioProps, ...itemProps }: RadioFieldProps) => (
  <Form.Item {...itemProps} className={['railops-form-field', className].filter(Boolean).join(' ')}>
    <Radio.Group {...radioProps} />
  </Form.Item>
);

export interface FormActionsProps {
  onCancel?: () => void;
  cancelText?: ReactNode;
  submitText?: ReactNode;
  loading?: boolean;
  disabled?: boolean;
  align?: 'start' | 'end';
  children?: ReactNode;
  localeText?: Partial<RailopsLocaleText>;
  className?: string;
}

export const FormActions = ({ onCancel, cancelText, submitText, loading, disabled, align = 'end', children, localeText, className }: FormActionsProps) => {
  const text = useRailopsLocaleText(localeText);
  return (
    <div className={['railops-form-actions', `railops-form-actions-${align}`, className].filter(Boolean).join(' ')}>
      {children}
      {onCancel && <RailopsButton onClick={onCancel}>{cancelText ?? text.cancel}</RailopsButton>}
      <RailopsButton variant="primary" htmlType="submit" loading={loading} disabled={disabled}>{submitText ?? text.submit}</RailopsButton>
    </div>
  );
};
