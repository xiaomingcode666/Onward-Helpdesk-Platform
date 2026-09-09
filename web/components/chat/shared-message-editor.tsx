"use client"

import { useEffect, useRef, useState, type ChangeEvent } from "react"
import Placeholder from "@tiptap/extension-placeholder"
import { EditorContent, useEditor } from "@tiptap/react"
import StarterKit from "@tiptap/starter-kit"
import {
  ImageIcon,
  MessageSquareTextIcon,
  PaperclipIcon,
  SendHorizonalIcon,
  SendIcon,
} from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { VoiceRecorderButton } from "@/components/chat/voice-recorder-button"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
  buildSendableEditorHTML,
  hasUploadingEditorImages,
  markEditorImageUploadedByTitle,
  MessageImageExtension,
  removeEditorImageByTitle,
  revokeEditorObjectUrl,
  revokeEditorObjectUrls,
  setEditorImageUploadingByTitle,
  type UploadedEditorImage,
} from "@/lib/im-editor-image"
import { generateUUID } from "@/lib/utils"
import { useI18n } from "@/i18n/provider"

export type UploadedMessageEditorImage = UploadedEditorImage

export type MessageEditorQuickReply = {
  id: number
  groupName?: string
  title: string
  content: string
}

type SharedMessageEditorVariant = "customer" | "agent"

type SharedMessageEditorProps = {
  variant: SharedMessageEditorVariant
  disabled?: boolean
  uploadingAsset?: boolean
  manageLocalUploading?: boolean
  quickReplies?: {
    open: boolean
    loading: boolean
    items: MessageEditorQuickReply[]
    onOpenChange: (open: boolean) => void
  }
  onSend: (html: string) => Promise<void>
  onUploadImage: (file: File) => Promise<UploadedMessageEditorImage | null>
  onSendAttachment: (file: File) => Promise<void>
  onSendVoice: (file: File, durationSeconds: number) => Promise<void>
  onTypingChange?: (typing: boolean) => void
}

export function SharedMessageEditor({
  variant,
  disabled = false,
  uploadingAsset = false,
  manageLocalUploading = false,
  quickReplies,
  onSend,
  onUploadImage,
  onSendAttachment,
  onSendVoice,
  onTypingChange,
}: SharedMessageEditorProps) {
  const t = useI18n()
  const [localUploading, setLocalUploading] = useState(false)
  const imageInputRef = useRef<HTMLInputElement | null>(null)
  const attachmentInputRef = useRef<HTMLInputElement | null>(null)
  const onSendRef = useRef(onSend)
  const onUploadImageRef = useRef(onUploadImage)
  const onSendAttachmentRef = useRef(onSendAttachment)
  const onSendVoiceRef = useRef(onSendVoice)
  const onTypingChangeRef = useRef(onTypingChange)
  const typingRef = useRef(false)
  const shouldRestoreFocusRef = useRef(false)
  const objectUrlsRef = useRef<Set<string>>(new Set())
  const uploadedImagesRef = useRef(new Map<string, UploadedMessageEditorImage>())
  const placeholderRef = useRef(t("conversation.editorPlaceholder"))
  const isCustomer = variant === "customer"
  const mediaBusy = uploadingAsset || (manageLocalUploading && localUploading)

  placeholderRef.current = t("conversation.editorPlaceholder")

  useEffect(() => {
    const objectUrls = objectUrlsRef.current
    return () => {
      revokeEditorObjectUrls(objectUrls)
    }
  }, [])

  useEffect(() => {
    onSendRef.current = onSend
  }, [onSend])

  useEffect(() => {
    onUploadImageRef.current = onUploadImage
  }, [onUploadImage])

  useEffect(() => {
    onSendAttachmentRef.current = onSendAttachment
  }, [onSendAttachment])

  useEffect(() => {
    onSendVoiceRef.current = onSendVoice
  }, [onSendVoice])

  useEffect(() => {
    onTypingChangeRef.current = onTypingChange
  }, [onTypingChange])

  useEffect(() => () => {
    if (typingRef.current) onTypingChangeRef.current?.(false)
  }, [])

  const editor = useEditor({
    immediatelyRender: false,
    extensions: [
      StarterKit.configure({
        heading: false,
        blockquote: false,
        codeBlock: false,
        bulletList: false,
        orderedList: false,
        horizontalRule: false,
      }),
      MessageImageExtension,
      Placeholder.configure({
        placeholder: () => placeholderRef.current,
      }),
    ],
    content: "",
    onUpdate: ({ editor: currentEditor }) => {
      const typing = currentEditor.getText().trim().length > 0
      if (typing === typingRef.current) return
      typingRef.current = typing
      onTypingChangeRef.current?.(typing)
    },
	    editorProps: {
	      attributes: {
	        class: getEditorClassName(variant),
	        role: "textbox",
	        "aria-label": t("conversation.editorPlaceholder"),
	        "aria-multiline": "true",
	      },
      handleKeyDown: (_view, event) => {
        if (event.key === "Enter" && !event.shiftKey) {
          event.preventDefault()
          void handleSend()
          return true
        }
        return false
      },
      handlePaste: (_view, event) => {
        if (disabled || mediaBusy) {
          return false
        }
        const imageFile = getClipboardImageFile(event.clipboardData)
        if (!imageFile) {
          return false
        }
        event.preventDefault()
        void insertUploadedImage(imageFile)
        return true
      },
    },
  })

  useEffect(() => {
    if (!editor) {
      return
    }
    editor.setEditable(!disabled)
  }, [disabled, editor])

  useEffect(() => {
    if (!editor || disabled || !shouldRestoreFocusRef.current) {
      return
    }
    requestAnimationFrame(() => {
      editor.commands.focus()
    })
  }, [disabled, editor])

  async function handleSend() {
    if (!editor || disabled) {
      return
    }
    const rawHTML = editor.getHTML()
    if (hasUploadingEditorImages(rawHTML, uploadedImagesRef.current)) {
      return
    }
    const html = buildSendableEditorHTML(rawHTML, uploadedImagesRef.current)
    if (!isMeaningfulHTML(html)) {
      return
    }
    await onSendRef.current(html)
    editor.commands.clearContent(true)
    if (typingRef.current) {
      typingRef.current = false
      onTypingChangeRef.current?.(false)
    }
    revokeEditorObjectUrls(objectUrlsRef.current)
    uploadedImagesRef.current.clear()
    if (!isCustomer) {
      requestAnimationFrame(() => {
        editor.commands.focus("end")
      })
    }
  }

  async function handleSelectImage(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ""
    if (!file || !editor || disabled || mediaBusy) {
      restoreFocusIfNeeded()
      return
    }
    await insertUploadedImage(file)
  }

  async function insertUploadedImage(file: File) {
    if (!editor || disabled || mediaBusy) {
      return
    }

    shouldRestoreFocusRef.current = true
    const objectUrl = URL.createObjectURL(file)
    objectUrlsRef.current.add(objectUrl)
    const placeholderId = `uploading-${generateUUID()}`
    editor
      .chain()
      .focus()
      .setImage({
        src: objectUrl,
        alt: file.name || "uploading-image",
        title: placeholderId,
      })
      .run()
    setEditorImageUploadingByTitle(editor, placeholderId)

    try {
      setLocalUploading(true)
      const uploaded = await onUploadImageRef.current(file)
      if (!uploaded?.assetId) {
        removeEditorImageByTitle(editor, placeholderId)
        revokeEditorObjectUrl(objectUrlsRef.current, objectUrl)
        return
      }
      markEditorImageUploadedByTitle(
        editor,
        placeholderId,
        uploaded,
        uploadedImagesRef.current
      )
    } finally {
      setLocalUploading(false)
      requestAnimationFrame(() => {
        if (!disabled && shouldRestoreFocusRef.current) {
          editor.commands.focus()
        }
      })
    }
  }

  async function handleSelectAttachment(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ""
    if (!file || disabled || mediaBusy) {
      restoreFocusIfNeeded()
      return
    }

    shouldRestoreFocusRef.current = editor?.isFocused ?? true
    try {
      setLocalUploading(true)
      await onSendAttachmentRef.current(file)
    } finally {
      setLocalUploading(false)
      requestAnimationFrame(() => {
        if (editor && !disabled && shouldRestoreFocusRef.current) {
          editor.commands.focus()
        }
      })
    }
  }

  function handleInsertQuickReply(item: MessageEditorQuickReply) {
    if (!editor || disabled) {
      return
    }
    if (!item.content.trim()) {
      return
    }
    editor.chain().focus().insertContent(item.content).run()
    quickReplies?.onOpenChange(false)
  }

  function restoreFocusIfNeeded() {
    if (editor && shouldRestoreFocusRef.current) {
      requestAnimationFrame(() => {
        editor.commands.focus()
      })
    }
  }

  const editorContent = (
    <>
      <input
        ref={imageInputRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={handleSelectImage}
      />
      <input
        ref={attachmentInputRef}
        type="file"
        className="hidden"
        onChange={handleSelectAttachment}
      />
      {isCustomer ? (
        <div className="min-h-9">
          <EditorContent editor={editor} />
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-hidden px-2 py-1">
          <EditorContent editor={editor} className="h-full" />
        </div>
      )}
      <div className={getToolbarClassName(variant)}>
        <div className={isCustomer ? "flex items-center gap-1.5" : "flex items-center gap-1"}>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={getIconButtonClassName(variant)}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => {
              shouldRestoreFocusRef.current = editor?.isFocused ?? true
              imageInputRef.current?.click()
            }}
            disabled={disabled || mediaBusy}
            aria-label={mediaBusy ? t("conversation.imageUploading") : t("conversation.sendImage")}
            title={mediaBusy ? t("conversation.imageUploading") : t("conversation.sendImage")}
          >
            <ImageIcon className="size-3.5" />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={getIconButtonClassName(variant)}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => {
              shouldRestoreFocusRef.current = editor?.isFocused ?? true
              attachmentInputRef.current?.click()
            }}
            disabled={disabled || mediaBusy}
            aria-label={mediaBusy ? t("conversation.attachmentUploading") : t("conversation.sendAttachment")}
            title={mediaBusy ? t("conversation.attachmentUploading") : t("conversation.sendAttachment")}
          >
            <PaperclipIcon className="size-3.5" />
          </Button>
          <VoiceRecorderButton
            disabled={disabled || mediaBusy}
            className={isCustomer ? "size-7" : getIconButtonClassName(variant)}
            onRecorded={(file, durationSeconds) => onSendVoiceRef.current(file, durationSeconds)}
            onError={(message) => toast.error(message)}
          />
          {quickReplies ? (
            <Popover open={quickReplies.open} onOpenChange={quickReplies.onOpenChange}>
              <PopoverTrigger
              render={
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className={isCustomer ? "size-7" : "size-8"}
                    disabled={disabled || quickReplies.loading}
                    onMouseDown={(event) => event.preventDefault()}
                  />
                }
              >
                <MessageSquareTextIcon className="size-3.5" />
              </PopoverTrigger>
              <PopoverContent className="w-[30rem] p-0" align="start">
                <Command>
                  <CommandInput placeholder={t("conversation.searchQuickReplies")} />
                  <CommandList>
                    <CommandEmpty>{t("conversation.emptyQuickReplies")}</CommandEmpty>
                    <CommandGroup>
                      {quickReplies.items.map((item) => (
                        <CommandItem
                          key={item.id}
                          value={`${item.groupName ?? ""} ${item.title} ${item.content}`}
                          onSelect={() => handleInsertQuickReply(item)}
                        >
                          <div className="flex min-w-0 flex-col gap-0.5 py-0.5">
                            <span className="line-clamp-1 text-sm">
                              {item.groupName
                                ? `${item.groupName} / ${item.title}`
                                : item.title}
                            </span>
                            <span className="line-clamp-2 text-xs text-muted-foreground">
                              {item.content}
                            </span>
                          </div>
                        </CommandItem>
                      ))}
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            </Popover>
          ) : null}
        </div>
        <div className="flex items-center gap-2">
          <p className={isCustomer ? "hidden text-rhd-2xs text-muted-foreground sm:block" : "text-xs text-muted-foreground"}>
            {t("conversation.enterToSend")}
          </p>
          {isCustomer ? (
            <Button
              type="button"
              size="icon"
              onClick={() => void handleSend()}
              disabled={disabled}
              aria-label={t("conversation.send")}
              title={t("conversation.send")}
              className="bg-primary text-white shadow-[0_10px_20px_color-mix(in_srgb,var(--primary)_24%,transparent)] hover:bg-primary hover:brightness-105"
            >
              <SendHorizonalIcon className="size-3.5" />
            </Button>
          ) : (
            <Button
              type="button"
              size="sm"
              onClick={() => void handleSend()}
              disabled={disabled}
            >
              <SendIcon className="mr-1 size-4" />
              {t("conversation.send")}
            </Button>
          )}
        </div>
      </div>
    </>
  )

  if (isCustomer) {
    return (
      <div className="px-3 pt-2 pb-3">
        <div className="rounded-xl border border-border bg-background p-2 shadow-[0_8px_24px_rgba(15,23,42,0.05)] dark:shadow-none">
          {editorContent}
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col p-2">
      <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-sm border border-border bg-card">
        {editorContent}
      </div>
    </div>
  )
}

function getEditorClassName(variant: SharedMessageEditorVariant) {
  if (variant === "customer") {
    return "remote-helpdesk-scrollbar min-h-12 max-h-40 overflow-y-auto px-1.5 py-1 text-rhd-md leading-5 text-foreground outline-none [&_p]:m-0 [&_p+*]:mt-2 [&_.remote-helpdesk-editor-image-wrap]:my-2 [&_.remote-helpdesk-editor-image]:max-h-64 [&_.remote-helpdesk-editor-image]:max-w-full [&_.remote-helpdesk-editor-image]:rounded-lg [&_.remote-helpdesk-editor-image]:object-contain [&_.remote-helpdesk-editor-image-wrap-uploading_.remote-helpdesk-editor-image]:opacity-55"
  }
  return "h-full min-h-12 max-h-[20vh] overflow-y-auto px-1.5 py-1 text-sm leading-6 text-foreground outline-none sm:max-h-none [&_.ProseMirror-focused]:outline-none [&_p]:m-0 [&_p+.remote-helpdesk-editor-image-wrap]:mt-2 [&_.remote-helpdesk-editor-image-wrap]:my-2 [&_.remote-helpdesk-editor-image]:max-h-64 [&_.remote-helpdesk-editor-image]:max-w-full [&_.remote-helpdesk-editor-image]:rounded-md [&_.remote-helpdesk-editor-image]:object-contain [&_.remote-helpdesk-editor-image-wrap-uploading_.remote-helpdesk-editor-image]:opacity-55 [&_p.is-editor-empty:first-child]:before:text-muted-foreground"
}

function getToolbarClassName(variant: SharedMessageEditorVariant) {
  if (variant === "customer") {
    return "mt-1.5 flex items-center justify-between"
  }
  return "flex items-center justify-between rounded-b-sm border-t border-border bg-card px-2 pt-1 pb-2"
}

function getIconButtonClassName(variant: SharedMessageEditorVariant) {
  if (variant === "customer") {
    return "size-7 text-muted-foreground hover:bg-muted hover:text-foreground"
  }
  return "size-8"
}

function isMeaningfulHTML(html: string) {
  const normalized = html
    .replace(/<p><\/p>/g, "")
    .replace(/<p><br><\/p>/g, "")
    .replace(/\s+/g, "")
  if (/<img[\s\S]*?>/i.test(normalized)) {
    return true
  }
  const plainText = normalized.replace(/<[^>]+>/g, "").trim()
  return plainText !== ""
}

function getClipboardImageFile(data: DataTransfer | null) {
  if (!data) {
    return null
  }

  for (const item of Array.from(data.items)) {
    if (item.kind === "file" && item.type.startsWith("image/")) {
      return item.getAsFile()
    }
  }
  return null
}
