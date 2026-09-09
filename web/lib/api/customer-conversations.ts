"use client"

import { buildCustomerPortalQueryString } from "@/lib/api/customer-portal-query"
import { customerPortalRequest } from "@/lib/api/customer-portal-client"
import type {
  CustomerPortalConversation,
  CustomerPortalListQuery,
  CustomerPortalPage,
} from "@/lib/api/customer-portal-types"
import type { ConversationMessageTranslationDTO } from "@/lib/api/types"

export function fetchCustomerConversations() {
  return customerPortalRequest<CustomerPortalConversation[]>("/conversations")
}

export function fetchCustomerConversationsPage(query?: CustomerPortalListQuery) {
  return customerPortalRequest<CustomerPortalPage<CustomerPortalConversation>>(`/conversations/page${buildCustomerPortalQueryString(query)}`)
}

export function translateCustomerConversationMessage(
  conversationId: number,
  messageId: number,
  targetLanguage: string,
) {
  return customerPortalRequest<ConversationMessageTranslationDTO>(
    `/conversations/${conversationId}/_translate`,
    {
      method: "POST",
      body: JSON.stringify({
        message_id: messageId,
        target_language: targetLanguage,
      }),
    },
  )
}
