"use client";

import type { ComponentProps } from "react";

import { Checkbox as AntCheckbox } from "antd";
import { cn } from "@/lib/utils";

type SelectionCheckboxProps = ComponentProps<typeof AntCheckbox> & {
  wrapperClassName?: string;
};

export function SelectionCheckbox({
  className,
  wrapperClassName,
  ...props
}: SelectionCheckboxProps) {
  return (
    <span
      className={cn("inline-flex shrink-0", wrapperClassName)}
      onPointerDown={(event) => event.stopPropagation()}
      onClick={(event) => event.stopPropagation()}
    >
      <AntCheckbox className={className} {...props} />
    </span>
  );
}
