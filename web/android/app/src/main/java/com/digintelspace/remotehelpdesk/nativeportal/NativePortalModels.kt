package com.digintelspace.remotehelpdesk.nativeportal

import androidx.annotation.StringRes
import com.digintelspace.remotehelpdesk.R
import org.json.JSONArray
import org.json.JSONObject

enum class NativePortalTab(val wireValue: String, @StringRes val titleRes: Int) {
    CONVERSATIONS("chat", R.string.portal_tab_conversations),
    TICKETS("tickets", R.string.portal_tab_tickets),
    MEETINGS("video", R.string.portal_tab_meetings),
    DEVICES("devices", R.string.portal_tab_devices),
    PROFILE("my", R.string.portal_tab_profile),
    ;

    companion object {
        fun fromWireValue(value: String?): NativePortalTab =
            entries.firstOrNull { it.wireValue == value } ?: CONVERSATIONS
    }
}

data class NativeConversation(
    val id: Long,
    val status: String,
    val lastMessageSummary: String,
    val lastMessageAt: String,
    val unreadCount: Int,
    val deviceId: Long,
    val deviceNo: String,
    val productName: String,
    val assigneeName: String,
    val ticketNo: String,
    val meetingId: String,
    val humanHandoffEnabled: Boolean,
)

data class NativeMessage(
    val id: Long,
    val conversationId: Long,
    val clientMessageId: String,
    val senderType: String,
    val senderName: String,
    val messageType: String,
    val content: String,
    val payload: String,
    val sentAt: String,
    val sendStatus: Int,
)

enum class NativeMessageSendPhase { SENDING, FAILED }

data class NativeMessageSendState(
    val content: String,
    val phase: NativeMessageSendPhase,
)

enum class NativeMediaKind(val messageType: String) {
    IMAGE("image"),
    ATTACHMENT("attachment"),
}

data class NativePendingMedia(
    val localPath: String,
    val filename: String,
    val mimeType: String,
    val fileSize: Long,
    val kind: NativeMediaKind,
)

data class NativeUploadedAsset(
    val assetId: String,
    val filename: String,
    val mimeType: String,
    val fileSize: Long,
)

data class NativeMessageAsset(
    val assetId: String,
    val filename: String,
    val mimeType: String,
)

internal fun NativeMessage.mediaAsset(): NativeMessageAsset? {
    if (messageType.lowercase() !in setOf("image", "attachment", "file", "audio", "video")) return null
    val data = runCatching { JSONObject(payload) }.getOrNull() ?: return null
    val assetId = data.string("assetId").trim()
    if (assetId.isBlank()) return null
    return NativeMessageAsset(
        assetId = assetId,
        filename = data.string("filename").ifBlank { content },
        mimeType = data.string("mimeType").ifBlank { "application/octet-stream" },
    )
}

data class NativeMessagePage(
    val messages: List<NativeMessage>,
    val cursor: String,
    val hasMore: Boolean,
)

data class NativePortalPage<T>(
    val items: List<T>,
    val page: Int,
    val limit: Int,
    val total: Int,
) {
    val hasMore: Boolean
        get() = page > 0 && limit > 0 && page * limit < total
}

data class NativeTicketProgress(
    val id: Long,
    val eventType: String,
    val content: String,
    val createdAt: String,
)

data class NativeTicket(
    val id: Long,
    val ticketNo: String,
    val title: String,
    val status: String,
    val priority: String,
    val deviceNo: String,
    val productName: String,
    val assigneeName: String,
    val updatedAt: String,
    val repairSummary: String,
    val progress: List<NativeTicketProgress>,
    val feedbackRating: Int,
    val canConfirm: Boolean,
    val canReopen: Boolean,
    val canRate: Boolean,
)

data class NativeManualFile(
    val id: Long,
    val title: String,
    val filename: String,
    val fileSize: Long,
    val mimeType: String,
    val url: String,
    val uploadedAt: String,
)

data class NativeMeeting(
    val id: String,
    val ticketNo: String,
    val title: String,
    val status: String,
    val deviceNo: String,
    val productName: String,
    val scheduledAt: String,
    val startedAt: String,
    val endedAt: String,
    val durationSeconds: Int,
    val participantCount: Int,
    val transcriptCount: Int,
    val annotationCount: Int,
)

data class NativeMeetingJoinConfig(
    val domain: String,
    val roomName: String,
    val jwt: String,
    val jitsiUrl: String,
    val meetingId: String,
)

data class NativeDevice(
    val id: Long,
    val deviceNo: String,
    val serialNo: String,
    val productName: String,
    val productCode: String,
    val modelName: String,
    val regionCode: String,
    val status: String,
    val warrantyEndAt: String,
    val manualCount: Int,
    val repairHistoryCount: Int,
    val openTicketCount: Int,
    val conversationCount: Int,
)

data class NativeProfile(
    val id: Long,
    val name: String,
    val companyName: String,
    val email: String,
    val mobile: String,
    val boundDeviceCount: Int,
    val activeConversationCount: Int,
    val openTicketCount: Int,
    val upcomingMeetingCount: Int,
)

internal fun JSONObject.string(key: String): String = optString(key, "")
internal fun JSONObject.long(key: String): Long = optLong(key, 0L)
internal fun JSONObject.integer(key: String): Int = optInt(key, 0)
internal fun JSONObject.boolean(key: String): Boolean = optBoolean(key, false)

internal inline fun <T> JSONArray.mapObjects(mapper: (JSONObject) -> T): List<T> =
    buildList {
        for (index in 0 until length()) {
            optJSONObject(index)?.let { add(mapper(it)) }
        }
    }

internal fun JSONObject.toConversation() = NativeConversation(
    id = long("id"),
    status = string("status"),
    lastMessageSummary = string("last_message_summary"),
    lastMessageAt = string("last_message_at").ifBlank { string("last_active_at") },
    unreadCount = integer("customer_unread_count"),
    deviceId = long("device_id"),
    deviceNo = string("device_no"),
    productName = string("product_name"),
    assigneeName = string("current_assignee_name"),
    ticketNo = string("current_ticket_no"),
    meetingId = string("current_meeting_id"),
    humanHandoffEnabled = boolean("human_handoff_enabled"),
)

internal fun JSONObject.toMessage() = NativeMessage(
    id = long("id"),
    conversationId = long("conversationId"),
    clientMessageId = string("clientMsgId"),
    senderType = string("senderType"),
    senderName = string("senderName"),
    messageType = string("messageType"),
    content = string("content"),
    payload = string("payload"),
    sentAt = string("sentAt"),
    sendStatus = integer("sendStatus"),
)

internal fun JSONObject.toTicket() = NativeTicket(
    id = long("id"),
    ticketNo = string("ticket_no"),
    title = string("title"),
    status = string("status"),
    priority = string("priority"),
    deviceNo = string("device_no"),
    productName = string("product_name"),
    assigneeName = string("assignee_name"),
    updatedAt = string("updated_at"),
    repairSummary = string("repair_summary"),
    progress = optJSONArray("progress")?.mapObjects {
        NativeTicketProgress(
            id = it.long("id"),
            eventType = it.string("event_type"),
            content = it.string("content"),
            createdAt = it.string("created_at"),
        )
    }.orEmpty(),
    feedbackRating = optJSONObject("feedback")?.integer("rating") ?: 0,
    canConfirm = boolean("can_confirm"),
    canReopen = boolean("can_reopen"),
    canRate = boolean("can_rate"),
)

internal fun JSONObject.toManualFile() = NativeManualFile(
    id = long("id"),
    title = string("title"),
    filename = string("filename"),
    fileSize = long("file_size"),
    mimeType = string("mime_type"),
    url = string("url"),
    uploadedAt = string("uploaded_at"),
)

internal fun JSONObject.toMeeting() = NativeMeeting(
    id = string("id"),
    ticketNo = string("ticket_no"),
    title = string("title"),
    status = string("status"),
    deviceNo = string("device_no"),
    productName = string("product_name"),
    scheduledAt = string("scheduled_at"),
    startedAt = string("started_at"),
    endedAt = string("ended_at"),
    durationSeconds = integer("duration_seconds"),
    participantCount = integer("participant_count"),
    transcriptCount = integer("transcript_count"),
    annotationCount = integer("annotation_count"),
)

internal fun JSONObject.toDevice() = NativeDevice(
    id = long("id"),
    deviceNo = string("device_no"),
    serialNo = string("serial_no"),
    productName = string("product_name"),
    productCode = string("product_code"),
    modelName = string("model_name"),
    regionCode = string("region_code"),
    status = string("status"),
    warrantyEndAt = string("warranty_end_at"),
    manualCount = integer("manual_count"),
    repairHistoryCount = integer("repair_history_count"),
    openTicketCount = integer("open_ticket_count"),
    conversationCount = integer("conversation_count"),
)

internal fun JSONObject.toProfile() = NativeProfile(
    id = long("id"),
    name = string("name"),
    companyName = string("company_name"),
    email = string("primary_email"),
    mobile = string("primary_mobile"),
    boundDeviceCount = integer("bound_device_count"),
    activeConversationCount = integer("active_conversation_count"),
    openTicketCount = integer("open_ticket_count"),
    upcomingMeetingCount = integer("upcoming_meeting_count"),
)
