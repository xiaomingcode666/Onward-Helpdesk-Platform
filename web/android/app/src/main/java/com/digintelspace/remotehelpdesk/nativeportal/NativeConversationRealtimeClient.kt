package com.digintelspace.remotehelpdesk.nativeportal

import android.os.Handler
import android.os.Looper
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONObject
import java.util.concurrent.TimeUnit

class NativeConversationRealtimeClient(
    private val session: NativeCustomerSession,
    private val onConnectionChanged: (Boolean) -> Unit,
    private val onMessageCreated: (NativeMessage) -> Unit,
    private val onResyncRequired: () -> Unit,
) {
    private val client = OkHttpClient.Builder()
        .pingInterval(25, TimeUnit.SECONDS)
        .retryOnConnectionFailure(true)
        .build()
    private val handler = Handler(Looper.getMainLooper())
    private var socket: WebSocket? = null
    private var conversationId = 0L
    private var active = false
    private var reconnectAttempt = 0

    fun connect(targetConversationId: Long) {
        if (targetConversationId <= 0) return
        if (active && conversationId == targetConversationId && socket != null) return
        disconnect()
        active = true
        conversationId = targetConversationId
        openSocket()
    }

    fun disconnect() {
        active = false
        conversationId = 0
        reconnectAttempt = 0
        handler.removeCallbacksAndMessages(null)
        socket?.close(1000, "client navigation")
        socket = null
        onConnectionChanged(false)
    }

    fun shutdown() {
        disconnect()
        client.dispatcher.executorService.shutdown()
        client.connectionPool.evictAll()
    }

    private fun openSocket() {
        if (!active || conversationId <= 0) return
        val webSocketUrl = session.apiBaseUrl.trimEnd('/')
            .replaceFirst("https://", "wss://")
            .replaceFirst("http://", "ws://") + "/api/ws/open"
        val targetConversationId = conversationId
        val request = Request.Builder()
            .url(webSocketUrl)
            .header("Sec-WebSocket-Protocol", "rhd.credential, rhd.access.${session.accessToken}")
            .build()
        socket = client.newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                if (!active || conversationId != targetConversationId) {
                    webSocket.close(1000, "stale conversation")
                    return
                }
                reconnectAttempt = 0
                socket = webSocket
                webSocket.send(
                    JSONObject()
                        .put("type", "subscribe")
                        .put("topics", org.json.JSONArray().put("conversation:$targetConversationId"))
                        .toString(),
                )
                onConnectionChanged(true)
                onResyncRequired()
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                if (!active || conversationId != targetConversationId) return
                val root = runCatching { JSONObject(text) }.getOrNull() ?: return
                val type = root.optString("type")
                val data = root.optJSONObject("data") ?: root.optJSONObject("payload")
                when {
                    type == "resyncRequired" -> onResyncRequired()
                    type == "message.created" -> parseMessage(data)?.let(onMessageCreated)
                    type.startsWith("conversation.") -> onResyncRequired()
                }
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                if (socket === webSocket) socket = null
                onConnectionChanged(false)
                scheduleReconnect(targetConversationId)
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                if (socket === webSocket) socket = null
                onConnectionChanged(false)
                scheduleReconnect(targetConversationId)
            }
        })
    }

    private fun parseMessage(data: JSONObject?): NativeMessage? {
        if (data == null) return null
        data.optJSONObject("message")?.takeIf { it.optLong("id") > 0 }?.let { return it.toMessage() }
        val id = data.optLong("messageId")
        val targetConversationId = data.optLong("conversationId")
        if (id <= 0 || targetConversationId <= 0) return null
        return JSONObject()
            .put("id", id)
            .put("conversationId", targetConversationId)
            .put("clientMsgId", data.optString("clientMsgId"))
            .put("senderType", data.optString("senderType"))
            .put("senderName", data.optString("senderName"))
            .put("messageType", data.optString("messageType"))
            .put("content", data.optString("content"))
            .put("payload", data.optString("payload"))
            .put("sendStatus", data.optInt("sendStatus"))
            .put("sentAt", data.optString("sentAt"))
            .toMessage()
    }

    private fun scheduleReconnect(targetConversationId: Long) {
        if (!active || conversationId != targetConversationId) return
        val delay = (500L shl reconnectAttempt.coerceAtMost(4)).coerceAtMost(8_000L)
        reconnectAttempt += 1
        handler.removeCallbacksAndMessages(null)
        handler.postDelayed({
            if (active && conversationId == targetConversationId) openSocket()
        }, delay)
    }
}
