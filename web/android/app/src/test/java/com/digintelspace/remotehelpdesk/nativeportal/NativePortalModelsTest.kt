package com.digintelspace.remotehelpdesk.nativeportal

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class NativePortalModelsTest {
    @Test
    fun customerConversationUsesPortalSnakeCaseFields() {
        val conversation = JSONObject(
            """
            {
              "id": 42,
              "status": "active",
              "last_message_summary": "设备无法启动",
              "last_message_at": "2026-08-18T10:20:00Z",
              "customer_unread_count": 3,
              "device_id": 9,
              "device_no": "DEV-009",
              "product_name": "控制器",
              "current_assignee_name": "服务工程师",
              "current_ticket_no": "T-001",
              "current_meeting_id": "meeting-1",
              "human_handoff_enabled": true
            }
            """.trimIndent(),
        ).toConversation()

        assertEquals(42L, conversation.id)
        assertEquals("控制器", conversation.productName)
        assertEquals(3, conversation.unreadCount)
        assertTrue(conversation.humanHandoffEnabled)
    }

    @Test
    fun messageUsesExistingCamelCaseContract() {
        val message = JSONObject(
            """
            {
              "id": 88,
              "conversationId": 42,
              "clientMsgId": "native-1",
              "senderType": "external_customer",
              "senderName": "客户",
              "messageType": "text",
              "content": "现场温度过高",
              "sentAt": "2026-08-18T10:21:00Z",
              "sendStatus": 1
            }
            """.trimIndent(),
        ).toMessage()

        assertEquals(42L, message.conversationId)
        assertEquals("native-1", message.clientMessageId)
        assertEquals("现场温度过高", message.content)
    }

    @Test
    fun ticketAndManualKeepActionAndFileFields() {
        val ticket = JSONObject(
            """
            {
              "id": 7,
              "ticket_no": "T-007",
              "title": "报警复位失败",
              "status": "resolved",
              "priority": "high",
              "repair_summary": "更换传感器",
              "progress": [{"id": 1, "event_type": "resolved", "content": "处理完成", "created_at": "2026-08-18"}],
              "feedback": {"rating": 5},
              "can_confirm": true,
              "can_reopen": true,
              "can_rate": false
            }
            """.trimIndent(),
        ).toTicket()
        val manual = JSONObject(
            """
            {"id": 3, "title": "安装手册", "filename": "manual.pdf", "file_size": 1024, "mime_type": "application/pdf", "url": "https://example.com/manual.pdf"}
            """.trimIndent(),
        ).toManualFile()

        assertEquals(5, ticket.feedbackRating)
        assertEquals(1, ticket.progress.size)
        assertTrue(ticket.canConfirm)
        assertEquals("manual.pdf", manual.filename)
        assertEquals(1024L, manual.fileSize)
    }
}
