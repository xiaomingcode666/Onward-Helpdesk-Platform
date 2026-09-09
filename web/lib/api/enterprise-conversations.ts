import { apiPost } from "@/lib/api/client"
import type { ApiResponse, ConversationMessageTranslationDTO } from "@/lib/api/types"

export async function translateConversationMessage(
  conversationId: number,
  messageId: number,
  targetLanguage: string,
): Promise<ApiResponse<ConversationMessageTranslationDTO>> {
  return apiPost<ConversationMessageTranslationDTO>(
    `/conversations/${conversationId}/_translate`,
    {
      message_id: messageId,
      target_language: targetLanguage,
    },
  )
}
