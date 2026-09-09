package com.digintelspace.remotehelpdesk.nativeportal

import android.app.Application
import android.content.ActivityNotFoundException
import android.content.Intent
import android.net.Uri
import android.os.Handler
import android.os.Looper
import android.provider.OpenableColumns
import androidx.annotation.StringRes
import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import androidx.lifecycle.AndroidViewModel
import androidx.core.content.FileProvider
import java.util.concurrent.Executors
import java.io.File
import java.util.UUID
import com.digintelspace.remotehelpdesk.R
import org.json.JSONObject

data class NativePortalUiState(
    val selectedTab: NativePortalTab = NativePortalTab.CONVERSATIONS,
    val loading: Boolean = false,
    val busy: Boolean = false,
    val error: String = "",
    val notice: String = "",
    val conversations: List<NativeConversation> = emptyList(),
    val selectedConversationId: Long = 0,
    val messages: List<NativeMessage> = emptyList(),
    val pendingMedia: NativePendingMedia? = null,
    val messageSendState: NativeMessageSendState? = null,
    val realtimeConnected: Boolean = false,
    val messageCursor: String = "",
    val hasOlderMessages: Boolean = false,
    val conversationsPage: Int = 0,
    val conversationsHasMore: Boolean = false,
    val ticketsPage: Int = 0,
    val ticketsHasMore: Boolean = false,
    val meetingsPage: Int = 0,
    val meetingsHasMore: Boolean = false,
    val devicesPage: Int = 0,
    val devicesHasMore: Boolean = false,
    val loadingMoreTab: NativePortalTab? = null,
    val tickets: List<NativeTicket> = emptyList(),
    val selectedTicketId: Long = 0,
    val meetings: List<NativeMeeting> = emptyList(),
    val selectedMeetingId: String = "",
    val meetingJoinConfig: NativeMeetingJoinConfig? = null,
    val devices: List<NativeDevice> = emptyList(),
    val selectedDeviceId: Long = 0,
    val deviceManuals: List<NativeManualFile> = emptyList(),
    val profile: NativeProfile? = null,
    val pendingServiceCode: String = "",
    val authExpired: Boolean = false,
    val accountDeleted: Boolean = false,
) {
    val selectedConversation: NativeConversation?
        get() = conversations.firstOrNull { it.id == selectedConversationId }
    val selectedTicket: NativeTicket?
        get() = tickets.firstOrNull { it.id == selectedTicketId }
    val selectedMeeting: NativeMeeting?
        get() = meetings.firstOrNull { it.id == selectedMeetingId }
    val selectedDevice: NativeDevice?
        get() = devices.firstOrNull { it.id == selectedDeviceId }
}

class NativeCustomerPortalViewModel(application: Application) : AndroidViewModel(application) {
    private val sessionStore = NativeCustomerSessionStore(application)
    private val pushRegistrationStore = NativePushRegistrationStore(application)
    private val session = sessionStore.read()
    private val api = session?.let { NativeCustomerApi(it, application) }
    private val executor = Executors.newFixedThreadPool(3)
    private val mainHandler = Handler(Looper.getMainLooper())
    private val loadedTabs = mutableSetOf<NativePortalTab>()
    private val mutableState = mutableStateOf(
        NativePortalUiState(authExpired = session == null),
    )
    private val realtimeClient = session?.let { currentSession ->
        NativeConversationRealtimeClient(
            session = currentSession,
            onConnectionChanged = { connected ->
                mainHandler.post {
                    mutableState.value = mutableState.value.copy(realtimeConnected = connected)
                }
            },
            onMessageCreated = { message -> mainHandler.post { handleRealtimeMessage(message) } },
            onResyncRequired = { mainHandler.post { resyncSelectedConversation() } },
        )
    }
    private var foreground = false
    private var activeMeetingId = ""

    val state: State<NativePortalUiState> = mutableState
    val displayName: String = session?.displayName.orEmpty()

    private fun str(@StringRes id: Int): String = getApplication<Application>().getString(id)

    private val messagePoller = object : Runnable {
        override fun run() {
            if (!foreground) return
            val selected = mutableState.value.selectedConversation
            if (selected != null && !isConversationClosed(selected.status) && !mutableState.value.realtimeConnected) {
                loadMessages(selected.id, silent = true)
            }
            mainHandler.postDelayed(this, MESSAGE_POLL_INTERVAL_MILLIS)
        }
    }

    private val meetingHeartbeat = object : Runnable {
        override fun run() {
            val meetingId = activeMeetingId
            if (meetingId.isBlank()) return
            execute(silentFailure = true, request = { requireApi().meetingHeartbeat(meetingId) }) { }
            mainHandler.postDelayed(this, MEETING_HEARTBEAT_INTERVAL_MILLIS)
        }
    }

    fun initialize(initialTab: NativePortalTab, serviceCode: String = "") {
        if (session == null) return
        val normalizedServiceCode = serviceCode.trim().uppercase()
        mutableState.value = mutableState.value.copy(pendingServiceCode = normalizedServiceCode)
        selectTab(if (normalizedServiceCode.isBlank()) initialTab else NativePortalTab.DEVICES)
    }

    fun consumePendingServiceCode() {
        mutableState.value = mutableState.value.copy(pendingServiceCode = "")
    }

    fun setForeground(value: Boolean) {
        foreground = value
        mainHandler.removeCallbacks(messagePoller)
        if (value) {
            connectSelectedConversation()
            mainHandler.postDelayed(messagePoller, MESSAGE_POLL_INTERVAL_MILLIS)
        } else {
            realtimeClient?.disconnect()
        }
    }

    fun selectTab(tab: NativePortalTab) {
        if (tab != NativePortalTab.CONVERSATIONS) realtimeClient?.disconnect()
        mutableState.value = mutableState.value.copy(
            selectedTab = tab,
            error = "",
            notice = "",
            messageSendState = null,
            selectedConversationId = if (tab == NativePortalTab.CONVERSATIONS) mutableState.value.selectedConversationId else 0,
            selectedTicketId = if (tab == NativePortalTab.TICKETS) mutableState.value.selectedTicketId else 0,
            selectedMeetingId = if (tab == NativePortalTab.MEETINGS) mutableState.value.selectedMeetingId else "",
            selectedDeviceId = if (tab == NativePortalTab.DEVICES) mutableState.value.selectedDeviceId else 0,
        )
        if (tab !in loadedTabs) loadTab(tab)
        if (tab == NativePortalTab.CONVERSATIONS) connectSelectedConversation()
    }

    fun reloadCurrent() {
        loadTab(mutableState.value.selectedTab, force = true)
    }

    fun clearCurrentSelection(): Boolean = when {
        mutableState.value.selectedConversationId > 0 -> true.also { clearConversationSelection() }
        mutableState.value.selectedTicketId > 0 -> true.also { clearTicketSelection() }
        mutableState.value.selectedMeetingId.isNotBlank() -> true.also { clearMeetingSelection() }
        mutableState.value.selectedDeviceId > 0 -> true.also { clearDeviceSelection() }
        else -> false
    }

    fun selectConversation(id: Long) {
        mutableState.value = mutableState.value.copy(
            selectedConversationId = id,
            messages = emptyList(),
            messageCursor = "",
            hasOlderMessages = false,
            messageSendState = null,
            notice = "",
            error = "",
        )
        loadMessages(id)
        connectSelectedConversation()
    }

    fun clearConversationSelection() {
        realtimeClient?.disconnect()
        deletePendingMediaFile()
        mutableState.value = mutableState.value.copy(
            selectedConversationId = 0,
            messages = emptyList(),
            messageCursor = "",
            hasOlderMessages = false,
            pendingMedia = null,
            messageSendState = null,
            notice = "",
        )
    }

    fun stageMedia(uri: Uri, kind: NativeMediaKind) {
        if (mutableState.value.busy) return
        mutableState.value = mutableState.value.copy(busy = true, error = "", notice = str(R.string.notice_reading_file))
        executor.execute {
            try {
                val media = copyMediaToCache(uri, kind)
                mainHandler.post {
                    deletePendingMediaFile()
                    mutableState.value = mutableState.value.copy(
                        busy = false,
                        pendingMedia = media,
                        notice = "",
                    )
                }
            } catch (error: Exception) {
                mainHandler.post {
                    mutableState.value = mutableState.value.copy(
                        busy = false,
                        error = error.message ?: str(R.string.error_file_read_failed),
                        notice = "",
                    )
                }
            }
        }
    }

    fun stageCameraFile(file: File) {
        if (!file.isFile || file.length() <= 0) return
        if (file.length() > MAX_MEDIA_BYTES) {
            file.delete()
            mutableState.value = mutableState.value.copy(error = str(R.string.error_image_too_large))
            return
        }
        deletePendingMediaFile()
        mutableState.value = mutableState.value.copy(
            pendingMedia = NativePendingMedia(
                localPath = file.absolutePath,
                filename = file.name,
                mimeType = "image/jpeg",
                fileSize = file.length(),
                kind = NativeMediaKind.IMAGE,
            ),
            error = "",
            notice = "",
        )
    }

    fun cancelPendingMedia() {
        deletePendingMediaFile()
        mutableState.value = mutableState.value.copy(pendingMedia = null, notice = "")
    }

    fun sendPendingMedia() {
        val current = mutableState.value
        val media = current.pendingMedia ?: return
        val conversationId = current.selectedConversationId
        if (conversationId <= 0 || current.busy) return
        val clientMessageId = "android_customer_${media.kind.messageType}_${UUID.randomUUID()}"
        execute(
            busy = true,
            request = {
                val asset = requireApi().uploadMedia(conversationId, media)
                requireApi().sendMessage(
                    conversationId = conversationId,
                    content = asset.filename,
                    messageType = media.kind.messageType,
                    payload = JSONObject().put("assetId", asset.assetId).toString(),
                    clientMessageId = clientMessageId,
                )
            },
        ) { sent ->
            if (mutableState.value.selectedConversationId != conversationId) return@execute
            deletePendingMediaFile()
            mutableState.value = mutableState.value.copy(
                busy = false,
                pendingMedia = null,
                messages = (mutableState.value.messages + sent).distinctBy { it.id to it.clientMessageId },
                notice = if (media.kind == NativeMediaKind.IMAGE) str(R.string.notice_image_sent) else str(R.string.notice_attachment_sent),
            )
            mainHandler.postDelayed({ loadMessages(conversationId, silent = true) }, 900L)
        }
    }

    fun openMessageMedia(message: NativeMessage) {
        val asset = message.mediaAsset() ?: run {
            mutableState.value = mutableState.value.copy(error = str(R.string.error_no_attachment))
            return
        }
        if (mutableState.value.busy) return
        mutableState.value = mutableState.value.copy(busy = true, error = "", notice = str(R.string.notice_downloading_attachment))
        executor.execute {
            val safeName = asset.filename
                .replace(Regex("[^A-Za-z0-9._\\-\u4e00-\u9fff]"), "_")
                .take(120)
                .ifBlank { str(R.string.conversation_attachment_fallback) }
            val safeAssetId = asset.assetId
                .replace(Regex("[^A-Za-z0-9._-]"), "_")
                .take(48)
                .ifBlank { "asset" }
            val destination = File(
                File(getApplication<Application>().cacheDir, "conversation-media").apply { mkdirs() },
                "download-$safeAssetId-$safeName",
            )
            try {
                val responseMimeType = requireApi().downloadMedia(asset.assetId, destination)
                val mimeType = responseMimeType.ifBlank { asset.mimeType }
                val context = getApplication<Application>()
                val uri = FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", destination)
                val intent = Intent(Intent.ACTION_VIEW).apply {
                    setDataAndType(uri, mimeType)
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_GRANT_READ_URI_PERMISSION)
                }
                context.startActivity(intent)
                mainHandler.post {
                    mutableState.value = mutableState.value.copy(busy = false, notice = "", error = "")
                }
            } catch (_: ActivityNotFoundException) {
                destination.delete()
                mainHandler.post {
                    mutableState.value = mutableState.value.copy(
                        busy = false,
                        notice = "",
                        error = str(R.string.error_no_viewer_app),
                    )
                }
            } catch (error: Exception) {
                destination.delete()
                mainHandler.post {
                    mutableState.value = mutableState.value.copy(
                        busy = false,
                        notice = "",
                        error = error.message ?: str(R.string.error_attachment_open_failed),
                    )
                }
            }
        }
    }

    fun loadOlderMessages() {
        val current = mutableState.value
        if (!current.hasOlderMessages || current.messageCursor.isBlank() || current.selectedConversationId <= 0) return
        val conversationId = current.selectedConversationId
        execute(
            busy = true,
            request = { requireApi().messages(conversationId, current.messageCursor) },
        ) { page ->
            if (mutableState.value.selectedConversationId != conversationId) return@execute
            val merged = (page.messages + mutableState.value.messages).distinctBy { it.id to it.clientMessageId }
            mutableState.value = mutableState.value.copy(
                messages = merged,
                messageCursor = page.cursor,
                hasOlderMessages = page.hasMore,
                busy = false,
            )
        }
    }

    fun sendMessage(content: String, onSent: () -> Unit) {
        val conversationId = mutableState.value.selectedConversationId
        val normalizedContent = content.trim()
        if (conversationId <= 0 || normalizedContent.isBlank()) return
        sendMessageInternal(conversationId, normalizedContent, onSent)
    }

    fun retryMessage(onSent: () -> Unit) {
        val current = mutableState.value
        val pending = current.messageSendState ?: return
        if (pending.phase != NativeMessageSendPhase.FAILED || current.selectedConversationId <= 0) return
        sendMessageInternal(current.selectedConversationId, pending.content, onSent)
    }

    private fun sendMessageInternal(conversationId: Long, content: String, onSent: () -> Unit) {
        mutableState.value = mutableState.value.copy(
            messageSendState = NativeMessageSendState(content, NativeMessageSendPhase.SENDING),
        )
        execute(
            busy = true,
            onFailure = {
                if (mutableState.value.selectedConversationId == conversationId) {
                    mutableState.value = mutableState.value.copy(
                        messageSendState = NativeMessageSendState(content, NativeMessageSendPhase.FAILED),
                    )
                }
            },
            request = { requireApi().sendMessage(conversationId, content) },
        ) { sent ->
            if (mutableState.value.selectedConversationId == conversationId) {
                mutableState.value = mutableState.value.copy(
                    messages = (mutableState.value.messages + sent).distinctBy { it.id to it.clientMessageId },
                    busy = false,
                    messageSendState = null,
                    notice = "",
                )
                onSent()
                mainHandler.postDelayed({ loadMessages(conversationId, silent = true) }, 900L)
            }
        }
    }

    fun requestHumanSupport() {
        val conversationId = mutableState.value.selectedConversationId
        if (conversationId <= 0) return
        execute(
            busy = true,
            request = { requireApi().requestHumanSupport(conversationId) },
        ) {
            mutableState.value = mutableState.value.copy(busy = false, notice = str(R.string.notice_human_support_submitted))
            loadTab(NativePortalTab.CONVERSATIONS, force = true, silent = true)
        }
    }

    fun selectTicket(id: Long) {
        mutableState.value = mutableState.value.copy(selectedTicketId = id, notice = "", error = "")
    }

    fun clearTicketSelection() {
        mutableState.value = mutableState.value.copy(selectedTicketId = 0, notice = "")
    }

    fun confirmTicket() {
        val id = mutableState.value.selectedTicketId
        if (id <= 0) return
        execute(busy = true, request = { requireApi().confirmTicket(id) }) { updated ->
            replaceTicket(updated, str(R.string.notice_ticket_confirmed))
        }
    }

    fun reopenTicket(reason: String) {
        val id = mutableState.value.selectedTicketId
        if (id <= 0 || reason.isBlank()) return
        execute(busy = true, request = { requireApi().reopenTicket(id, reason.trim()) }) { updated ->
            replaceTicket(updated, str(R.string.notice_ticket_reopened))
        }
    }

    fun submitTicketFeedback(rating: Int, comment: String) {
        val id = mutableState.value.selectedTicketId
        if (id <= 0 || rating !in 1..5) return
        execute(busy = true, request = { requireApi().submitTicketFeedback(id, rating, comment.trim()) }) {
            mutableState.value = mutableState.value.copy(busy = false, notice = str(R.string.notice_feedback_submitted))
            loadTab(NativePortalTab.TICKETS, force = true, silent = true)
        }
    }

    fun selectMeeting(id: String) {
        mutableState.value = mutableState.value.copy(
            selectedMeetingId = id,
            meetingJoinConfig = null,
            notice = "",
            error = "",
        )
    }

    fun clearMeetingSelection() {
        mutableState.value = mutableState.value.copy(selectedMeetingId = "", meetingJoinConfig = null, notice = "")
    }

    fun prepareMeetingJoin() {
        val id = mutableState.value.selectedMeetingId
        if (id.isBlank()) return
        execute(busy = true, request = { requireApi().meetingJoinConfig(id) }) { config ->
            mutableState.value = mutableState.value.copy(
                busy = false,
                meetingJoinConfig = config,
                notice = str(R.string.notice_meeting_config_ready),
            )
        }
    }

    fun consumeMeetingJoinConfig() {
        mutableState.value = mutableState.value.copy(meetingJoinConfig = null)
    }

    fun meetingConferenceJoined(meetingId: String) {
        if (meetingId.isBlank()) return
        activeMeetingId = meetingId
        mainHandler.removeCallbacks(meetingHeartbeat)
        execute(silentFailure = true, request = { requireApi().meetingJoined(meetingId) }) { }
        mainHandler.postDelayed(meetingHeartbeat, MEETING_HEARTBEAT_INTERVAL_MILLIS)
    }

    fun meetingConferenceLeft(meetingId: String) {
        if (meetingId.isBlank()) return
        if (activeMeetingId == meetingId) activeMeetingId = ""
        mainHandler.removeCallbacks(meetingHeartbeat)
        execute(silentFailure = true, request = { requireApi().meetingLeft(meetingId) }) {
            loadTab(NativePortalTab.MEETINGS, force = true, silent = true)
        }
    }

    fun selectDevice(id: Long) {
        mutableState.value = mutableState.value.copy(
            selectedDeviceId = id,
            deviceManuals = emptyList(),
            notice = "",
            error = "",
        )
        execute(request = { requireApi().deviceManuals(id) }, silentFailure = true) { manuals ->
            if (mutableState.value.selectedDeviceId == id) {
                mutableState.value = mutableState.value.copy(deviceManuals = manuals)
            }
        }
    }

    fun clearDeviceSelection() {
        mutableState.value = mutableState.value.copy(selectedDeviceId = 0, deviceManuals = emptyList(), notice = "")
    }

    fun startDeviceConversation() {
        val deviceId = mutableState.value.selectedDeviceId
        if (deviceId <= 0) return
        execute(busy = true, request = { requireApi().createConversation(deviceId) }) { conversationId ->
            loadedTabs.remove(NativePortalTab.CONVERSATIONS)
            mutableState.value = mutableState.value.copy(
                busy = false,
                selectedDeviceId = 0,
                selectedTab = NativePortalTab.CONVERSATIONS,
                notice = "",
            )
            loadTab(NativePortalTab.CONVERSATIONS, force = true, targetConversationId = conversationId)
        }
    }

    fun bindDevice(serviceCode: String) {
        if (serviceCode.isBlank()) return
        execute(busy = true, request = { requireApi().bindDevice(serviceCode.trim()) }) { deviceId ->
            mutableState.value = mutableState.value.copy(busy = false, notice = str(R.string.notice_device_bound))
            loadTab(NativePortalTab.DEVICES, force = true, targetDeviceId = deviceId)
        }
    }

    fun updateProfile(name: String, email: String, mobile: String) {
        if (name.isBlank()) return
        execute(
            busy = true,
            request = { requireApi().updateProfile(name.trim(), email.trim(), mobile.trim()) },
        ) { profile ->
            mutableState.value = mutableState.value.copy(
                busy = false,
                profile = profile,
                notice = str(R.string.notice_profile_saved),
            )
        }
    }

    fun deleteAccount(currentPassword: String) {
        if (currentPassword.isBlank()) return
        execute(busy = true, request = { requireApi().deleteAccount(currentPassword) }) {
            sessionStore.clear()
            pushRegistrationStore.clearTokenId()
            mutableState.value = mutableState.value.copy(busy = false, accountDeleted = true)
        }
    }

    fun logout(onComplete: () -> Unit) {
        if (mutableState.value.busy) return
        mutableState.value = mutableState.value.copy(busy = true, error = "", notice = "")
        executor.execute {
            val tokenId = pushRegistrationStore.tokenId()
            if (tokenId > 0) runCatching { requireApi().revokePushToken(tokenId) }
            pushRegistrationStore.clearTokenId()
            sessionStore.clear()
            mainHandler.post {
                mutableState.value = mutableState.value.copy(busy = false)
                onComplete()
            }
        }
    }

    fun loadMore(tab: NativePortalTab) {
        val current = mutableState.value
        if (tab == NativePortalTab.PROFILE || current.loading || current.loadingMoreTab != null) return

        when (tab) {
            NativePortalTab.CONVERSATIONS -> {
                if (!current.conversationsHasMore) return
                val nextPage = current.conversationsPage + 1
                mutableState.value = current.copy(loadingMoreTab = tab, error = "", notice = "")
                execute(
                    onFailure = { mutableState.value = mutableState.value.copy(loadingMoreTab = null) },
                    request = { requireApi().conversations(nextPage) },
                ) { page ->
                    mutableState.value = mutableState.value.copy(
                        loadingMoreTab = null,
                        conversations = (mutableState.value.conversations + page.items).distinctBy { it.id },
                        conversationsPage = page.page,
                        conversationsHasMore = page.hasMore,
                    )
                }
            }
            NativePortalTab.TICKETS -> {
                if (!current.ticketsHasMore) return
                val nextPage = current.ticketsPage + 1
                mutableState.value = current.copy(loadingMoreTab = tab, error = "", notice = "")
                execute(
                    onFailure = { mutableState.value = mutableState.value.copy(loadingMoreTab = null) },
                    request = { requireApi().tickets(nextPage) },
                ) { page ->
                    mutableState.value = mutableState.value.copy(
                        loadingMoreTab = null,
                        tickets = (mutableState.value.tickets + page.items).distinctBy { it.id },
                        ticketsPage = page.page,
                        ticketsHasMore = page.hasMore,
                    )
                }
            }
            NativePortalTab.MEETINGS -> {
                if (!current.meetingsHasMore) return
                val nextPage = current.meetingsPage + 1
                mutableState.value = current.copy(loadingMoreTab = tab, error = "", notice = "")
                execute(
                    onFailure = { mutableState.value = mutableState.value.copy(loadingMoreTab = null) },
                    request = { requireApi().meetings(nextPage) },
                ) { page ->
                    mutableState.value = mutableState.value.copy(
                        loadingMoreTab = null,
                        meetings = (mutableState.value.meetings + page.items).distinctBy { it.id },
                        meetingsPage = page.page,
                        meetingsHasMore = page.hasMore,
                    )
                }
            }
            NativePortalTab.DEVICES -> {
                if (!current.devicesHasMore) return
                val nextPage = current.devicesPage + 1
                mutableState.value = current.copy(loadingMoreTab = tab, error = "", notice = "")
                execute(
                    onFailure = { mutableState.value = mutableState.value.copy(loadingMoreTab = null) },
                    request = { requireApi().devices(nextPage) },
                ) { page ->
                    mutableState.value = mutableState.value.copy(
                        loadingMoreTab = null,
                        devices = (mutableState.value.devices + page.items).distinctBy { it.id },
                        devicesPage = page.page,
                        devicesHasMore = page.hasMore,
                    )
                }
            }
            NativePortalTab.PROFILE -> Unit
        }
    }

    private fun loadTab(
        tab: NativePortalTab,
        force: Boolean = false,
        silent: Boolean = false,
        targetConversationId: Long = 0,
        targetDeviceId: Long = 0,
    ) {
        if (!force && tab in loadedTabs) return
        when (tab) {
            NativePortalTab.CONVERSATIONS -> execute(
                loading = !silent,
                request = { requireApi().conversations() },
            ) { page ->
                val items = page.items
                loadedTabs.add(tab)
                mutableState.value = mutableState.value.copy(
                    loading = false,
                    loadingMoreTab = null,
                    conversations = items,
                    conversationsPage = page.page,
                    conversationsHasMore = page.hasMore,
                    selectedConversationId = targetConversationId.takeIf { target -> items.any { it.id == target } }
                        ?: mutableState.value.selectedConversationId.takeIf { selected -> items.any { it.id == selected } }
                        ?: 0,
                )
                if (targetConversationId > 0 && items.any { it.id == targetConversationId }) {
                    loadMessages(targetConversationId)
                    connectSelectedConversation()
                }
            }
            NativePortalTab.TICKETS -> execute(loading = !silent, request = { requireApi().tickets() }) { page ->
                loadedTabs.add(tab)
                mutableState.value = mutableState.value.copy(
                    loading = false,
                    loadingMoreTab = null,
                    tickets = page.items,
                    ticketsPage = page.page,
                    ticketsHasMore = page.hasMore,
                )
            }
            NativePortalTab.MEETINGS -> execute(loading = !silent, request = { requireApi().meetings() }) { page ->
                loadedTabs.add(tab)
                mutableState.value = mutableState.value.copy(
                    loading = false,
                    loadingMoreTab = null,
                    meetings = page.items,
                    meetingsPage = page.page,
                    meetingsHasMore = page.hasMore,
                )
            }
            NativePortalTab.DEVICES -> execute(loading = !silent, request = { requireApi().devices() }) { page ->
                val items = page.items
                loadedTabs.add(tab)
                mutableState.value = mutableState.value.copy(
                    loading = false,
                    loadingMoreTab = null,
                    devices = items,
                    devicesPage = page.page,
                    devicesHasMore = page.hasMore,
                    selectedDeviceId = targetDeviceId.takeIf { target -> items.any { it.id == target } }
                        ?: mutableState.value.selectedDeviceId.takeIf { selected -> items.any { it.id == selected } }
                        ?: 0,
                )
            }
            NativePortalTab.PROFILE -> execute(loading = !silent, request = { requireApi().profile() }) { profile ->
                loadedTabs.add(tab)
                mutableState.value = mutableState.value.copy(loading = false, profile = profile)
            }
        }
    }

    private fun loadMessages(conversationId: Long, silent: Boolean = false) {
        execute(
            loading = !silent,
            silentFailure = silent,
            request = { requireApi().messages(conversationId) },
        ) { page ->
            if (mutableState.value.selectedConversationId != conversationId) return@execute
            mutableState.value = mutableState.value.copy(
                loading = false,
                messages = page.messages,
                messageCursor = page.cursor,
                hasOlderMessages = page.hasMore,
                conversations = mutableState.value.conversations.map {
                    if (it.id == conversationId) it.copy(unreadCount = 0) else it
                },
            )
            page.messages.lastOrNull()?.let { message ->
                executor.execute {
                    try {
                        requireApi().markRead(conversationId, message.id)
                    } catch (_: Exception) {
                        // Read receipts are best effort and must not hide loaded messages.
                    }
                }
            }
        }
    }

    private fun connectSelectedConversation() {
        val current = mutableState.value
        val selected = current.selectedConversation ?: return
        if (
            foreground &&
            current.selectedTab == NativePortalTab.CONVERSATIONS &&
            !isConversationClosed(selected.status)
        ) {
            realtimeClient?.connect(selected.id)
        }
    }

    private fun resyncSelectedConversation() {
        val conversationId = mutableState.value.selectedConversationId
        if (conversationId <= 0) return
        loadMessages(conversationId, silent = true)
        loadTab(NativePortalTab.CONVERSATIONS, force = true, silent = true)
    }

    private fun handleRealtimeMessage(message: NativeMessage) {
        val current = mutableState.value
        if (message.conversationId != current.selectedConversationId) {
            loadTab(NativePortalTab.CONVERSATIONS, force = true, silent = true)
            return
        }
        val messages = current.messages
            .filterNot {
                it.id == message.id ||
                    (message.clientMessageId.isNotBlank() && it.clientMessageId == message.clientMessageId)
            }
            .plus(message)
            .sortedWith(compareBy<NativeMessage> { it.id }.thenBy { it.sentAt })
        mutableState.value = current.copy(
            messages = messages,
            conversations = current.conversations.map {
                if (it.id == message.conversationId) {
                    it.copy(
                        lastMessageSummary = message.content,
                        lastMessageAt = message.sentAt,
                        unreadCount = 0,
                    )
                } else {
                    it
                }
            },
        )
        if (message.senderType.lowercase() !in setOf("customer", "user", "external_customer", "visitor")) {
            executor.execute { runCatching { requireApi().markRead(message.conversationId, message.id) } }
        }
    }

    private fun replaceTicket(ticket: NativeTicket, message: String) {
        mutableState.value = mutableState.value.copy(
            busy = false,
            tickets = mutableState.value.tickets.map { if (it.id == ticket.id) ticket else it },
            notice = message,
        )
    }

    private fun requireApi(): NativeCustomerApi = api ?: throw NativeApiException(str(R.string.error_login_expired), unauthorized = true)

    private fun <T> execute(
        loading: Boolean = false,
        busy: Boolean = false,
        silentFailure: Boolean = false,
        onFailure: ((Exception) -> Unit)? = null,
        request: () -> T,
        success: (T) -> Unit,
    ) {
        if (loading || busy) {
            mutableState.value = mutableState.value.copy(
                loading = loading || mutableState.value.loading,
                busy = busy || mutableState.value.busy,
                error = "",
                notice = "",
            )
        }
        executor.execute {
            try {
                val result = request()
                mainHandler.post { success(result) }
            } catch (error: Exception) {
                mainHandler.post {
                    val unauthorized = (error as? NativeApiException)?.unauthorized == true
                    if (unauthorized) sessionStore.clear()
                    mutableState.value = mutableState.value.copy(
                        loading = false,
                        busy = false,
                        error = if (silentFailure) mutableState.value.error else error.message ?: str(R.string.error_request_failed),
                        authExpired = unauthorized,
                    )
                    onFailure?.invoke(error)
                }
            }
        }
    }

    override fun onCleared() {
        foreground = false
        realtimeClient?.shutdown()
        deletePendingMediaFile()
        mainHandler.removeCallbacksAndMessages(null)
        executor.shutdownNow()
        super.onCleared()
    }

    private fun isConversationClosed(status: String): Boolean =
        status in setOf("cancelled", "closed", "ended", "finished")

    private fun copyMediaToCache(uri: Uri, kind: NativeMediaKind): NativePendingMedia {
        val resolver = getApplication<Application>().contentResolver
        var filename = ""
        var declaredSize = -1L
        resolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE), null, null, null)?.use { cursor ->
            if (cursor.moveToFirst()) {
                val nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                val sizeIndex = cursor.getColumnIndex(OpenableColumns.SIZE)
                if (nameIndex >= 0) filename = cursor.getString(nameIndex).orEmpty()
                if (sizeIndex >= 0 && !cursor.isNull(sizeIndex)) declaredSize = cursor.getLong(sizeIndex)
            }
        }
        val mimeType = resolver.getType(uri).orEmpty().ifBlank {
            if (kind == NativeMediaKind.IMAGE) "image/jpeg" else "application/octet-stream"
        }
        if (kind == NativeMediaKind.IMAGE && !mimeType.startsWith("image/")) {
            throw NativeApiException(str(R.string.error_pick_image))
        }
        if (declaredSize > MAX_MEDIA_BYTES) throw NativeApiException(str(R.string.error_file_too_large))
        val safeName = filename.ifBlank {
            if (kind == NativeMediaKind.IMAGE) str(R.string.media_camera_filename) else str(R.string.media_attachment_filename)
        }.replace(Regex("[^A-Za-z0-9._\\-\u4e00-\u9fff]"), "_").take(120)
        val directory = File(getApplication<Application>().cacheDir, "conversation-media").apply { mkdirs() }
        val output = File(directory, "${UUID.randomUUID()}-$safeName")
        try {
            resolver.openInputStream(uri)?.use { input ->
                output.outputStream().use { target -> input.copyTo(target) }
            } ?: throw NativeApiException(str(R.string.error_cannot_read_file))
            if (output.length() <= 0) throw NativeApiException(str(R.string.error_empty_file))
            if (output.length() > MAX_MEDIA_BYTES) throw NativeApiException(str(R.string.error_file_too_large))
            return NativePendingMedia(
                localPath = output.absolutePath,
                filename = safeName,
                mimeType = mimeType,
                fileSize = output.length(),
                kind = kind,
            )
        } catch (error: Exception) {
            output.delete()
            throw error
        }
    }

    private fun deletePendingMediaFile() {
        mutableState.value.pendingMedia?.localPath?.let { File(it).delete() }
    }

    private companion object {
        const val MESSAGE_POLL_INTERVAL_MILLIS = 5_000L
        const val MEETING_HEARTBEAT_INTERVAL_MILLIS = 20_000L
        const val MAX_MEDIA_BYTES = 20L * 1024L * 1024L
    }
}
