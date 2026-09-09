"use client"

import {
  SharedMessageEditor,
  type UploadedMessageEditorImage,
} from "@/components/chat/shared-message-editor"

type CustomerMessageEditorProps = {
  disabled?: boolean
  uploadingAsset?: boolean
  onSend: (html: string) => Promise<void>
  onUploadImage: (file: File) => Promise<UploadedMessageEditorImage | null>
  onSendAttachment: (file: File) => Promise<void>
  onSendVoice: (file: File, durationSeconds: number) => Promise<void>
  onTypingChange?: (typing: boolean) => void
}

export function CustomerMessageEditor({
  disabled = false,
  uploadingAsset = false,
  onSend,
  onUploadImage,
  onSendAttachment,
  onSendVoice,
  onTypingChange,
}: CustomerMessageEditorProps) {
  return (
    <SharedMessageEditor
      variant="customer"
      disabled={disabled}
      uploadingAsset={uploadingAsset}
      manageLocalUploading
      onSend={onSend}
      onUploadImage={onUploadImage}
      onSendAttachment={onSendAttachment}
      onSendVoice={onSendVoice}
      onTypingChange={onTypingChange}
    />
  )
}
