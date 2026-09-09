"use client";

import type * as React from "react";
import { useState } from "react";

import { cn } from "@/lib/utils";
import type { DialogRoot } from "@base-ui/react/dialog";
import { StandardModal } from "@railops/ui";
import { Button } from "@/components/ui/button";
import { Maximize2Icon, Minimize2Icon } from "lucide-react";
import { useI18n } from "@/i18n/provider";

const dialogSizeWidth = {
  sm: 448, // max-w-md
  md: 576, // max-w-xl
  lg: 672, // max-w-2xl
  xl: 896, // max-w-4xl
  xxl: 1024, // max-w-5xl
} as const;

type ProjectDialogSize = keyof typeof dialogSizeWidth;

type ProjectDialogProps = DialogRoot.Props & {
  title: React.ReactNode;
  description?: React.ReactNode;
  size?: ProjectDialogSize;
  children: React.ReactNode;
  footer?: React.ReactNode;
  contentClassName?: string;
  headerClassName?: string;
  bodyClassName?: string;
  footerClassName?: string;
  showCloseButton?: boolean;
  closeOnEsc?: boolean;
  allowFullscreen?: boolean;
  defaultFullscreen?: boolean;
  bodyScrollable?: boolean;
};

function ProjectDialog({
  open,
  onOpenChange,
  title,
  description: _description,
  size = "md",
  children,
  footer,
  contentClassName,
  headerClassName,
  bodyClassName,
  footerClassName,
  showCloseButton = true,
  closeOnEsc = true,
  allowFullscreen = false,
  defaultFullscreen = false,
  bodyScrollable = true,
}: ProjectDialogProps) {
  const t = useI18n();
  const [fullscreen, setFullscreen] = useState(defaultFullscreen);

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) {
      setFullscreen(defaultFullscreen);
    }
    onOpenChange?.(nextOpen, undefined as never);
  }

  return (
    <StandardModal
      open={open}
      onCancel={() => handleOpenChange(false)}
      keyboard={closeOnEsc}
      width={fullscreen ? "calc(100vw - 16px)" : dialogSizeWidth[size]}
      style={
        fullscreen
          ? { top: 8, maxWidth: "calc(100vw - 16px)", height: "calc(100vh - 16px)" }
          : { maxHeight: "calc(100vh - 2rem)" }
      }
      className={cn(
        "rhd-railops-project-dialog",
        fullscreen && "rhd-railops-project-dialog-fullscreen",
        contentClassName,
      )}
      closeIcon={showCloseButton ? undefined : null}
      title={(
        <div className={cn("flex min-w-0 items-center gap-2", (showCloseButton || allowFullscreen) && "pr-16", headerClassName)}>
          <span className="min-w-0 flex-1">{title}</span>
          {allowFullscreen ? (
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              onClick={() => setFullscreen((value) => !value)}
            >
              {fullscreen ? <Minimize2Icon /> : <Maximize2Icon />}
              <span className="sr-only">
                {fullscreen ? t("common.exitFullscreen") : t("common.fullscreen")}
              </span>
            </Button>
          ) : null}
        </div>
      )}
      footer={footer ? (
        <div className={cn(
          "flex flex-col-reverse items-center gap-2 border-t border-[var(--railops-border-light)] bg-[var(--railops-surface)] px-6 py-4 sm:flex-row sm:justify-end",
          footerClassName,
        )}>
          {footer}
        </div>
      ) : null}
    >
      <style jsx>{`
        .project-dialog-native-scrollbar {
          scrollbar-width: thin;
          scrollbar-color: hsl(var(--border)) transparent;
        }

        .project-dialog-native-scrollbar::-webkit-scrollbar {
          width: 10px;
        }

        .project-dialog-native-scrollbar::-webkit-scrollbar-track {
          background: transparent;
        }

        .project-dialog-native-scrollbar::-webkit-scrollbar-thumb {
          background: hsl(var(--border));
          border: 2px solid transparent;
          border-radius: 9999px;
          background-clip: content-box;
        }

        .project-dialog-native-scrollbar::-webkit-scrollbar-thumb:hover {
          background: color-mix(
            in srgb,
            hsl(var(--border)) 80%,
            hsl(var(--foreground))
          );
          border: 2px solid transparent;
          background-clip: content-box;
        }
      `}</style>
      {bodyScrollable ? (
        <div
          className={cn(
            "project-dialog-native-scrollbar min-h-0 flex-1 overflow-y-auto",
            bodyClassName,
          )}
        >
          <div className="space-y-4 p-6 w-full">{children}</div>
        </div>
      ) : (
        <div className={cn("min-h-0 flex-1", bodyClassName)}>{children}</div>
      )}
    </StandardModal>
  );
}

export { ProjectDialog };
