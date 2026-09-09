package com.digintelspace.remotehelpdesk.nativeportal

import android.Manifest
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.text.Html
import java.io.File
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.Send
import androidx.compose.material.icons.outlined.HeadsetMic
import androidx.compose.material.icons.outlined.AttachFile
import androidx.compose.material.icons.outlined.CameraAlt
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.Image
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Wifi
import androidx.compose.material.icons.outlined.WifiOff
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.produceState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.core.content.ContextCompat
import androidx.core.content.FileProvider
import com.digintelspace.remotehelpdesk.R
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

private enum class ConversationFilter { ALL, ACTIVE, CLOSED }

@Composable
internal fun NativeConversationScreen(
    state: NativePortalUiState,
    viewModel: NativeCustomerPortalViewModel,
) {
    val selected = state.selectedConversation
    if (selected != null) {
        ConversationDetail(state, selected, viewModel)
        return
    }

    var query by rememberSaveable { mutableStateOf("") }
    var filter by rememberSaveable { mutableStateOf(ConversationFilter.ALL) }
    val normalizedQuery = query.trim().lowercase()
    val filtered = remember(state.conversations, normalizedQuery, filter) {
        state.conversations.filter { item ->
            val closed = isClosedConversation(item.status)
            val filterMatches = when (filter) {
                ConversationFilter.ALL -> true
                ConversationFilter.ACTIVE -> !closed
                ConversationFilter.CLOSED -> closed
            }
            val queryMatches = normalizedQuery.isBlank() || listOf(
                item.deviceNo,
                item.productName,
                item.ticketNo,
                item.lastMessageSummary,
                item.assigneeName,
            ).joinToString(" ").lowercase().contains(normalizedQuery)
            filterMatches && queryMatches
        }
    }

    Column(Modifier.fillMaxSize()) {
        PortalMessageBanner(state, viewModel::reloadCurrent)
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(PortalBackground)
                .padding(horizontal = 14.dp, vertical = 12.dp),
        ) {
            OutlinedTextField(
                value = query,
                onValueChange = { query = it },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                leadingIcon = { Icon(Icons.Outlined.Search, contentDescription = null, modifier = Modifier.size(19.dp)) },
                trailingIcon = {
                    if (query.isNotBlank()) {
                        IconButton(onClick = { query = "" }) {
                            Icon(Icons.Outlined.Close, contentDescription = stringResource(R.string.common_clear_search), modifier = Modifier.size(17.dp))
                        }
                    }
                },
                placeholder = { Text(stringResource(R.string.conversation_search_placeholder), fontSize = 13.sp) },
                shape = RoundedCornerShape(7.dp),
            )
            Spacer(Modifier.height(10.dp))
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(Color(0xFFE3E7EC), RoundedCornerShape(7.dp))
                    .padding(3.dp),
            ) {
                ConversationFilter.entries.forEach { item ->
                    val label = when (item) {
                        ConversationFilter.ALL -> stringResource(R.string.conversation_filter_all)
                        ConversationFilter.ACTIVE -> stringResource(R.string.conversation_filter_active)
                        ConversationFilter.CLOSED -> stringResource(R.string.conversation_filter_closed)
                    }
                    Box(
                        modifier = Modifier
                            .weight(1f)
                            .height(34.dp)
                            .background(
                                if (filter == item) Color.White else Color.Transparent,
                                RoundedCornerShape(5.dp),
                            )
                            .clickable { filter = item },
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(
                            label,
                            color = if (filter == item) PortalInk else PortalMuted,
                            fontSize = 12.sp,
                            fontWeight = FontWeight.Medium,
                        )
                    }
                }
            }
        }
        HorizontalDivider(color = PortalBorder)

        if (filtered.isEmpty() && !state.loading && !state.conversationsHasMore) {
            PortalEmptyState(stringResource(R.string.conversation_empty_title), stringResource(R.string.conversation_empty_detail))
        } else {
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.spacedBy(8.dp),
                contentPadding = androidx.compose.foundation.layout.PaddingValues(12.dp),
            ) {
                items(filtered, key = { it.id }) { item ->
                    ConversationListItem(item = item, onClick = { viewModel.selectConversation(item.id) })
                }
                if (state.conversationsHasMore) {
                    item(key = "load-more-conversations") {
                        PortalLoadMoreRow(NativePortalTab.CONVERSATIONS, state, viewModel)
                    }
                }
            }
        }
    }
}

@Composable
private fun ConversationListItem(item: NativeConversation, onClick: () -> Unit) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .background(Color.White, RoundedCornerShape(7.dp))
            .clickable(onClick = onClick)
            .padding(horizontal = 14.dp, vertical = 13.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    item.productName.ifBlank { item.deviceNo.ifBlank { stringResource(R.string.conversation_default_title) } },
                    color = PortalInk,
                    fontWeight = FontWeight.SemiBold,
                    fontSize = 14.sp,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    item.deviceNo.ifBlank { stringResource(R.string.conversation_id_label, item.id) },
                    color = PortalMuted,
                    fontSize = 10.sp,
                    maxLines = 1,
                )
            }
            PortalStatusPill(item.status)
        }
        Spacer(Modifier.height(8.dp))
        Text(
            plainMessage(item.lastMessageSummary).ifBlank { stringResource(R.string.conversation_no_messages) },
            color = Color(0xFF465266),
            fontSize = 13.sp,
            lineHeight = 18.sp,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
        )
        Spacer(Modifier.height(9.dp))
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                when {
                    item.ticketNo.isNotBlank() -> stringResource(R.string.conversation_ticket_label, item.ticketNo)
                    item.assigneeName.isNotBlank() -> stringResource(R.string.conversation_assignee_label, item.assigneeName)
                    else -> stringResource(R.string.conversation_ai_service)
                },
                modifier = Modifier.weight(1f),
                color = PortalMuted,
                fontSize = 10.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (item.unreadCount > 0) {
                Text(
                    if (item.unreadCount > 99) "99+" else item.unreadCount.toString(),
                    modifier = Modifier
                        .background(PortalDanger, RoundedCornerShape(9.dp))
                        .padding(horizontal = 6.dp, vertical = 2.dp),
                    color = Color.White,
                    fontSize = 9.sp,
                )
                Spacer(Modifier.width(8.dp))
            }
            Text(displayDate(item.lastMessageAt), color = PortalMuted, fontSize = 10.sp)
        }
    }
}

@Composable
private fun ConversationDetail(
    state: NativePortalUiState,
    conversation: NativeConversation,
    viewModel: NativeCustomerPortalViewModel,
) {
    val listState = rememberLazyListState()
    val context = LocalContext.current
    var draft by rememberSaveable(conversation.id) { mutableStateOf("") }
    var pendingCameraFile by remember { mutableStateOf<File?>(null) }
    val closed = isClosedConversation(conversation.status)
    val imagePicker = rememberLauncherForActivityResult(ActivityResultContracts.GetContent()) { uri ->
        uri?.let { viewModel.stageMedia(it, NativeMediaKind.IMAGE) }
    }
    val attachmentPicker = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        uri?.let { viewModel.stageMedia(it, NativeMediaKind.ATTACHMENT) }
    }
    val camera = rememberLauncherForActivityResult(ActivityResultContracts.TakePicture()) { success ->
        val file = pendingCameraFile
        pendingCameraFile = null
        if (success && file != null) viewModel.stageCameraFile(file) else file?.delete()
    }
    fun beginCameraCapture() {
        val directory = File(context.cacheDir, "conversation-media").apply { mkdirs() }
        val file = File(directory, "camera-${System.currentTimeMillis()}.jpg")
        pendingCameraFile = file
        camera.launch(FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", file))
    }
    val cameraPermission = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (granted) beginCameraCapture()
    }

    LaunchedEffect(state.messages.size) {
        if (state.messages.isNotEmpty()) listState.animateScrollToItem(state.messages.lastIndex)
    }

    fun send() {
        if (draft.isBlank() || state.busy || closed) return
        viewModel.sendMessage(draft) { draft = "" }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFFF0F3F6))
            .imePadding(),
    ) {
        PortalMessageBanner(state, viewModel::reloadCurrent)
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .background(Color.White)
                .padding(horizontal = 14.dp, vertical = 9.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    conversation.productName.ifBlank { stringResource(R.string.conversation_default_title) },
                    color = PortalInk,
                    fontSize = 13.sp,
                    fontWeight = FontWeight.Medium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    conversation.assigneeName.ifBlank { stringResource(R.string.conversation_support_default) },
                    color = PortalMuted,
                    fontSize = 10.sp,
                    maxLines = 1,
                )
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(
                        if (state.realtimeConnected) Icons.Outlined.Wifi else Icons.Outlined.WifiOff,
                        contentDescription = null,
                        modifier = Modifier.size(11.dp),
                        tint = if (state.realtimeConnected) PortalSuccess else PortalMuted,
                    )
                    Spacer(Modifier.width(3.dp))
                    Text(
                        if (state.realtimeConnected) stringResource(R.string.conversation_realtime_connected) else stringResource(R.string.conversation_realtime_reconnecting),
                        color = if (state.realtimeConnected) PortalSuccess else PortalMuted,
                        fontSize = 9.sp,
                    )
                }
            }
            PortalStatusPill(conversation.status)
            if (!closed && conversation.humanHandoffEnabled) {
                Spacer(Modifier.width(4.dp))
                IconButton(onClick = viewModel::requestHumanSupport, enabled = !state.busy) {
                    Icon(Icons.Outlined.HeadsetMic, contentDescription = stringResource(R.string.conversation_request_human), tint = PortalBlue)
                }
            }
        }
        HorizontalDivider(color = PortalBorder)

        LazyColumn(
            state = listState,
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.spacedBy(9.dp),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 12.dp, vertical = 12.dp),
        ) {
            if (state.hasOlderMessages) {
                item(key = "older") {
                    Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                        TextButton(onClick = viewModel::loadOlderMessages, enabled = !state.busy) {
                            Text(stringResource(R.string.conversation_load_older), fontSize = 12.sp)
                        }
                    }
                }
            }
            if (state.messages.isEmpty() && !state.loading) {
                item(key = "empty") {
                    PortalEmptyState(stringResource(R.string.conversation_empty_created_title), stringResource(R.string.conversation_empty_created_detail))
                }
            }
            items(state.messages, key = { "${it.id}:${it.clientMessageId}" }) { message ->
                ConversationMessageBubble(message, viewModel::openMessageMedia)
            }
        }

        if (closed) {
            Text(
                stringResource(R.string.conversation_closed_hint),
                modifier = Modifier
                    .fillMaxWidth()
                    .background(Color.White)
                    .padding(16.dp),
                color = PortalMuted,
                fontSize = 12.sp,
            )
        } else {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(Color.White)
                    .padding(horizontal = 10.dp, vertical = 8.dp),
            ) {
                state.pendingMedia?.let { media ->
                    PendingMediaPreview(media, state.busy, viewModel::cancelPendingMedia, viewModel::sendPendingMedia)
                    Spacer(Modifier.height(8.dp))
                }
                Row(verticalAlignment = Alignment.CenterVertically) {
                    IconButton(
                        onClick = {
                            if (ContextCompat.checkSelfPermission(context, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED) {
                                beginCameraCapture()
                            } else {
                                cameraPermission.launch(Manifest.permission.CAMERA)
                            }
                        },
                        enabled = !state.busy && state.pendingMedia == null,
                    ) { Icon(Icons.Outlined.CameraAlt, contentDescription = stringResource(R.string.conversation_take_photo), tint = PortalMuted) }
                    IconButton(
                        onClick = { imagePicker.launch("image/*") },
                        enabled = !state.busy && state.pendingMedia == null,
                    ) { Icon(Icons.Outlined.Image, contentDescription = stringResource(R.string.conversation_pick_image), tint = PortalMuted) }
                    IconButton(
                        onClick = { attachmentPicker.launch(arrayOf("*/*")) },
                        enabled = !state.busy && state.pendingMedia == null,
                    ) { Icon(Icons.Outlined.AttachFile, contentDescription = stringResource(R.string.conversation_pick_attachment), tint = PortalMuted) }
                    if (state.busy) {
                        Text(stringResource(R.string.conversation_busy), color = PortalMuted, fontSize = 11.sp)
                    }
                }
                state.messageSendState?.let { sendState ->
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 4.dp, vertical = 3.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(
                            if (sendState.phase == NativeMessageSendPhase.SENDING) stringResource(R.string.conversation_sending) else stringResource(R.string.conversation_send_failed),
                            modifier = Modifier.weight(1f),
                            color = if (sendState.phase == NativeMessageSendPhase.SENDING) PortalMuted else PortalDanger,
                            fontSize = 11.sp,
                        )
                        if (sendState.phase == NativeMessageSendPhase.FAILED) {
                            TextButton(
                                onClick = { viewModel.retryMessage { draft = "" } },
                                enabled = !state.busy,
                                contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 6.dp, vertical = 0.dp),
                            ) { Text(stringResource(R.string.common_retry), color = PortalBlue, fontSize = 11.sp) }
                        }
                    }
                }
                Row(verticalAlignment = Alignment.CenterVertically) {
                    OutlinedTextField(
                        value = draft,
                        onValueChange = { draft = it.take(2000) },
                        modifier = Modifier.weight(1f),
                        enabled = !state.busy && state.pendingMedia == null,
                        placeholder = { Text(stringResource(R.string.conversation_input_placeholder), fontSize = 13.sp) },
                        maxLines = 4,
                        shape = RoundedCornerShape(7.dp),
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Send),
                        keyboardActions = KeyboardActions(onSend = { send() }),
                    )
                    Spacer(Modifier.width(6.dp))
                    Button(
                        onClick = { send() },
                        enabled = draft.isNotBlank() && !state.busy && state.pendingMedia == null,
                        modifier = Modifier.size(48.dp),
                        contentPadding = androidx.compose.foundation.layout.PaddingValues(0.dp),
                        shape = RoundedCornerShape(7.dp),
                        colors = ButtonDefaults.buttonColors(containerColor = PortalBlue),
                    ) {
                        Icon(Icons.AutoMirrored.Outlined.Send, contentDescription = stringResource(R.string.conversation_send), modifier = Modifier.size(20.dp))
                    }
                }
            }
        }
    }
}

@Composable
private fun PendingMediaPreview(
    media: NativePendingMedia,
    busy: Boolean,
    onCancel: () -> Unit,
    onSend: () -> Unit,
) {
    val bitmap by produceState<Bitmap?>(initialValue = null, media.localPath, media.kind) {
        value = if (media.kind == NativeMediaKind.IMAGE) {
            withContext(Dispatchers.IO) { BitmapFactory.decodeFile(media.localPath) }
        } else {
            null
        }
    }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(PortalBackground, RoundedCornerShape(7.dp))
            .padding(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            modifier = Modifier
                .size(50.dp)
                .background(Color(0xFFE3E8EE), RoundedCornerShape(6.dp)),
            contentAlignment = Alignment.Center,
        ) {
            if (bitmap != null) {
                Image(
                    bitmap = bitmap!!.asImageBitmap(),
                    contentDescription = media.filename,
                    modifier = Modifier.fillMaxSize(),
                    contentScale = ContentScale.Crop,
                )
            } else {
                Icon(Icons.Outlined.AttachFile, contentDescription = null, tint = PortalMuted)
            }
        }
        Spacer(Modifier.width(9.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(media.filename, color = PortalInk, fontSize = 12.sp, fontWeight = FontWeight.Medium, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(
                "${
                    if (media.kind == NativeMediaKind.IMAGE) {
                        stringResource(R.string.conversation_media_image_label)
                    } else {
                        stringResource(R.string.conversation_media_attachment_label)
                    }
                } · ${formatMediaSize(media.fileSize)}",
                color = PortalMuted,
                fontSize = 10.sp,
            )
        }
        IconButton(onClick = onCancel, enabled = !busy) {
            Icon(Icons.Outlined.Close, contentDescription = stringResource(R.string.conversation_cancel_pick), tint = PortalMuted)
        }
        Button(
            onClick = onSend,
            enabled = !busy,
            shape = RoundedCornerShape(7.dp),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 12.dp),
        ) { Text(stringResource(R.string.conversation_send_button), fontSize = 12.sp) }
    }
}

@Composable
private fun ConversationMessageBubble(message: NativeMessage, onOpenMedia: (NativeMessage) -> Unit) {
    val mine = message.senderType.lowercase() in setOf("customer", "external_customer", "visitor")
    val media = message.mediaAsset()
    Column(
        modifier = Modifier.fillMaxWidth(),
        horizontalAlignment = if (mine) Alignment.End else Alignment.Start,
    ) {
        Text(
            if (mine) stringResource(R.string.conversation_me_label) else message.senderName.ifBlank { stringResource(R.string.conversation_staff_label) },
            color = PortalMuted,
            fontSize = 9.sp,
            modifier = Modifier.padding(horizontal = 4.dp, vertical = 2.dp),
        )
        Column(
            modifier = Modifier
                .fillMaxWidth(0.82f)
                .background(
                    if (mine) Color(0xFF1769E0) else Color.White,
                    RoundedCornerShape(7.dp),
                )
                .padding(horizontal = 12.dp, vertical = 9.dp),
        ) {
            if (message.messageType != "text") {
                Text(
                    when (message.messageType) {
                        "image" -> stringResource(R.string.conversation_image_message)
                        "attachment", "file" -> stringResource(R.string.conversation_attachment_message)
                        "video" -> stringResource(R.string.conversation_video_message)
                        else -> stringResource(R.string.conversation_other_message, message.messageType)
                    },
                    color = if (mine) Color.White.copy(alpha = 0.72f) else PortalMuted,
                    fontSize = 9.sp,
                )
                Spacer(Modifier.height(3.dp))
            }
            Text(
                plainMessage(message.content).ifBlank { stringResource(R.string.conversation_content_unavailable) },
                color = if (mine) Color.White else PortalInk,
                fontSize = 13.sp,
                lineHeight = 19.sp,
            )
            if (media != null) {
                Spacer(Modifier.height(5.dp))
                TextButton(
                    onClick = { onOpenMedia(message) },
                    contentPadding = androidx.compose.foundation.layout.PaddingValues(0.dp),
                ) {
                    Text(
                        if (message.messageType == "image") stringResource(R.string.conversation_view_image) else stringResource(R.string.conversation_open_attachment),
                        color = if (mine) Color.White else PortalBlue,
                        fontSize = 11.sp,
                        fontWeight = FontWeight.Medium,
                    )
                }
            }
        }
        Text(
            displayDate(message.sentAt),
            color = PortalMuted,
            fontSize = 9.sp,
            modifier = Modifier.padding(horizontal = 4.dp, vertical = 2.dp),
        )
    }
}

private fun plainMessage(value: String): String = Html.fromHtml(
    value,
    Html.FROM_HTML_MODE_LEGACY,
).toString().trim()

private fun isClosedConversation(status: String): Boolean =
    status.lowercase() in setOf("cancelled", "closed", "ended", "finished")

private fun formatMediaSize(bytes: Long): String = when {
    bytes >= 1024L * 1024L -> "%.1f MB".format(bytes / (1024f * 1024f))
    bytes >= 1024L -> "%.1f KB".format(bytes / 1024f)
    else -> "$bytes B"
}
