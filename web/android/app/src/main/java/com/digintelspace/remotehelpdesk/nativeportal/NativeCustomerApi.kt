package com.digintelspace.remotehelpdesk.nativeportal

import android.content.Context
import android.net.Uri
import androidx.annotation.StringRes
import com.digintelspace.remotehelpdesk.BuildConfig
import com.digintelspace.remotehelpdesk.R
import org.json.JSONArray
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStream
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL
import java.util.UUID
import java.io.File
import java.util.concurrent.TimeUnit
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.MultipartBody
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.asRequestBody

class NativeApiException(
    message: String,
    val errorCode: Int = 0,
    val unauthorized: Boolean = false,
) : Exception(message)

class NativeCustomerApi(
    private val session: NativeCustomerSession,
    private val context: Context,
) {
    private val mediaClient = OkHttpClient.Builder()
        .connectTimeout(CONNECT_TIMEOUT_MILLIS.toLong(), TimeUnit.MILLISECONDS)
        .readTimeout(MEDIA_TIMEOUT_MILLIS.toLong(), TimeUnit.MILLISECONDS)
        .writeTimeout(MEDIA_TIMEOUT_MILLIS.toLong(), TimeUnit.MILLISECONDS)
        .build()
    init {
        require(isAllowedBaseUrl(session.apiBaseUrl)) { "Native API base URL must use HTTPS" }
    }

    private fun str(@StringRes id: Int): String = context.getString(id)

    fun conversations(page: Int = 1, limit: Int = DEFAULT_PAGE_SIZE): NativePortalPage<NativeConversation> =
        requestPage("/api/customer/v1/conversations/page", page, limit, JSONObject::toConversation)

    fun messages(conversationId: Long, cursor: String = ""): NativeMessagePage {
        val query = buildString {
            append("?conversationId=$conversationId&limit=50")
            if (cursor.isNotBlank()) append("&cursor=${Uri.encode(cursor)}")
        }
        val data = requestObject("/api/message/list$query")
        return NativeMessagePage(
            messages = data.optJSONArray("results")?.mapObjects(JSONObject::toMessage).orEmpty(),
            cursor = data.string("cursor"),
            hasMore = data.boolean("hasMore"),
        )
    }

    fun sendMessage(
        conversationId: Long,
        content: String,
        messageType: String = "text",
        payload: String = "",
        clientMessageId: String = UUID.randomUUID().toString(),
    ): NativeMessage {
        val data = requestObject(
            path = "/api/message/send",
            method = "POST",
            body = JSONObject()
                .put("conversationId", conversationId)
                .put("clientMsgId", clientMessageId)
                .put("messageType", messageType)
                .put("content", content)
                .put("payload", payload),
        )
        return data.toMessage()
    }

    fun uploadMedia(conversationId: Long, media: NativePendingMedia): NativeUploadedAsset {
        val file = File(media.localPath)
        if (!file.isFile) throw NativeApiException(str(R.string.api_error_file_unavailable))
        val endpoint = if (media.kind == NativeMediaKind.IMAGE) {
            "/api/message/upload_image"
        } else {
            "/api/message/upload_attachment"
        }
        val body = MultipartBody.Builder()
            .setType(MultipartBody.FORM)
            .addFormDataPart("conversationId", conversationId.toString())
            .addFormDataPart(
                "file",
                media.filename,
                file.asRequestBody(media.mimeType.toMediaTypeOrNull()),
            )
            .build()
        val request = Request.Builder()
            .url("${session.apiBaseUrl.trimEnd('/')}$endpoint")
            .header("Accept", "application/json")
            .header("Accept-Language", "zh-CN")
            .header("Authorization", "Bearer ${session.accessToken}")
            .post(body)
            .build()
        mediaClient.newCall(request).execute().use { response ->
            val data = parseEnvelope(response.code, response.body.string()) as? JSONObject
                ?: throw NativeApiException(str(R.string.api_error_upload_no_asset))
            return NativeUploadedAsset(
                assetId = data.string("assetId"),
                filename = data.string("filename").ifBlank { media.filename },
                mimeType = data.string("mimeType").ifBlank { media.mimeType },
                fileSize = data.optLong("fileSize", media.fileSize),
            ).also {
                if (it.assetId.isBlank()) throw NativeApiException(str(R.string.api_error_invalid_asset_id))
            }
        }
    }

    fun downloadMedia(assetId: String, destination: File): String {
        if (assetId.isBlank()) throw NativeApiException(str(R.string.api_error_invalid_attachment_id))
        val request = Request.Builder()
            .url("${session.apiBaseUrl.trimEnd('/')}/api/message/media/${Uri.encode(assetId)}")
            .header("Accept", "*/*")
            .header("Authorization", "Bearer ${session.accessToken}")
            .get()
            .build()
        try {
            mediaClient.newCall(request).execute().use { response ->
                if (!response.isSuccessful) {
                    val body = response.body.string()
                    parseEnvelope(response.code, body)
                    throw NativeApiException(str(R.string.api_error_download_failed))
                }
                val responseBody = response.body
                val declaredLength = responseBody.contentLength()
                if (declaredLength > MAX_DOWNLOAD_BYTES) throw NativeApiException(str(R.string.api_error_attachment_too_large))
                destination.parentFile?.mkdirs()
                var copied = 0L
                responseBody.byteStream().use { input ->
                    destination.outputStream().buffered().use { output ->
                        val buffer = ByteArray(DEFAULT_BUFFER_SIZE)
                        while (true) {
                            val read = input.read(buffer)
                            if (read < 0) break
                            copied += read
                            if (copied > MAX_DOWNLOAD_BYTES) throw NativeApiException(str(R.string.api_error_attachment_too_large))
                            output.write(buffer, 0, read)
                        }
                    }
                }
                if (copied <= 0) throw NativeApiException(str(R.string.api_error_empty_attachment))
                return responseBody.contentType()?.toString().orEmpty()
            }
        } catch (error: Exception) {
            destination.delete()
            throw error
        }
    }

    fun markRead(conversationId: Long, messageId: Long) {
        request(
            path = "/api/message/read",
            method = "POST",
            body = JSONObject().put("conversationId", conversationId).put("messageId", messageId),
        )
    }

    fun requestHumanSupport(conversationId: Long) {
        request(
            path = "/api/conversation/request_human",
            method = "POST",
            body = JSONObject().put("conversationId", conversationId).put("reason", "Android 原生客户端请求人工支持"),
        )
    }

    fun createConversation(deviceId: Long): Long = requestObject(
        path = "/api/conversation/create_or_match",
        method = "POST",
        body = JSONObject().put("deviceId", deviceId).put("forceNew", true),
    ).long("id")

    fun tickets(page: Int = 1, limit: Int = DEFAULT_PAGE_SIZE): NativePortalPage<NativeTicket> =
        requestPage("/api/customer/v1/tickets/page", page, limit, JSONObject::toTicket)

    fun confirmTicket(ticketId: Long): NativeTicket = requestObject(
        path = "/api/customer/v1/tickets/$ticketId/_confirm",
        method = "POST",
        body = JSONObject(),
    ).toTicket()

    fun reopenTicket(ticketId: Long, reason: String): NativeTicket = requestObject(
        path = "/api/customer/v1/tickets/$ticketId/_reopen",
        method = "POST",
        body = JSONObject().put("reason", reason),
    ).toTicket()

    fun submitTicketFeedback(ticketId: Long, rating: Int, comment: String) {
        request(
            path = "/api/customer/v1/tickets/$ticketId/feedback",
            method = "POST",
            body = JSONObject().put("rating", rating).put("comment", comment),
        )
    }

    fun meetings(page: Int = 1, limit: Int = DEFAULT_PAGE_SIZE): NativePortalPage<NativeMeeting> =
        requestPage("/api/customer/v1/meetings/page", page, limit, JSONObject::toMeeting)

    fun meetingJoinConfig(meetingId: String): NativeMeetingJoinConfig {
        val data = requestObject("/api/customer/v1/meetings/${Uri.encode(meetingId)}/join")
        return NativeMeetingJoinConfig(
            domain = data.string("domain"),
            roomName = data.string("roomName"),
            jwt = data.string("jwt"),
            jitsiUrl = data.string("jitsiUrl"),
            meetingId = data.string("meetingId"),
        )
    }

    fun meetingJoined(meetingId: String) {
        request(path = "/api/customer/v1/meetings/${Uri.encode(meetingId)}/_joined", method = "POST")
    }

    fun meetingLeft(meetingId: String) {
        request(path = "/api/customer/v1/meetings/${Uri.encode(meetingId)}/_left", method = "POST")
    }

    fun meetingHeartbeat(meetingId: String) {
        request(path = "/api/customer/v1/meetings/${Uri.encode(meetingId)}/_heartbeat", method = "POST")
    }

    fun devices(page: Int = 1, limit: Int = DEFAULT_PAGE_SIZE): NativePortalPage<NativeDevice> =
        requestPage("/api/customer/v1/devices/page", page, limit, JSONObject::toDevice)

    fun deviceManuals(deviceId: Long): List<NativeManualFile> =
        requestArray("/api/customer/v1/devices/$deviceId/manuals").mapObjects(JSONObject::toManualFile)

    fun bindDevice(serviceCode: String): Long = requestObject(
        path = "/api/customer/v1/devices/bind",
        method = "POST",
        body = JSONObject().put("serviceCode", serviceCode),
    ).long("deviceId")

    fun profile(): NativeProfile = requestObject("/api/customer/v1/me").toProfile()

    fun updateProfile(name: String, email: String, mobile: String): NativeProfile = requestObject(
        path = "/api/customer/v1/me/update",
        method = "POST",
        body = JSONObject()
            .put("name", name)
            .put("primary_email", email)
            .put("primary_mobile", mobile),
    ).toProfile()

    fun deleteAccount(currentPassword: String) {
        request(
            path = "/api/customer/v1/account-deletion",
            method = "POST",
            body = JSONObject()
                .put("current_password", currentPassword)
                .put("confirmation", "DELETE"),
        )
    }

    fun registerPushToken(token: String, deviceId: String, appVersion: String): Long = requestObject(
        path = "/api/customer/v1/notifications/push-tokens",
        method = "POST",
        body = JSONObject()
            .put("platform", "android")
            .put("token", token)
            .put("deviceId", deviceId)
            .put("appVersion", appVersion),
    ).long("id")

    fun revokePushToken(tokenId: Long) {
        if (tokenId <= 0) return
        request(
            path = "/api/customer/v1/notifications/push-tokens/$tokenId/_revoke",
            method = "POST",
        )
    }

    private fun <T> requestPage(
        path: String,
        page: Int,
        limit: Int,
        mapper: (JSONObject) -> T,
    ): NativePortalPage<T> {
        val safePage = page.coerceAtLeast(1)
        val safeLimit = limit.coerceIn(1, MAX_PAGE_SIZE)
        val data = requestObject("$path?page=$safePage&limit=$safeLimit")
        val pageData = data.optJSONObject("page")
        val responsePage = pageData?.optInt("page", safePage)?.coerceAtLeast(1) ?: safePage
        val responseLimit = pageData?.optInt("limit", safeLimit)?.coerceIn(1, MAX_PAGE_SIZE) ?: safeLimit
        val total = pageData?.optLong("total", 0L)?.coerceIn(0L, Int.MAX_VALUE.toLong())?.toInt() ?: 0
        return NativePortalPage(
            items = data.optJSONArray("results")?.mapObjects(mapper).orEmpty(),
            page = responsePage,
            limit = responseLimit,
            total = total,
        )
    }

    private fun requestArray(path: String): JSONArray = request(path) as? JSONArray ?: JSONArray()

    private fun requestObject(
        path: String,
        method: String = "GET",
        body: JSONObject? = null,
    ): JSONObject = request(path, method, body) as? JSONObject ?: JSONObject()

    private fun request(
        path: String,
        method: String = "GET",
        body: JSONObject? = null,
    ): Any? {
        val connection = URL("${session.apiBaseUrl.trimEnd('/')}$path").openConnection() as HttpURLConnection
        try {
            connection.requestMethod = method
            connection.connectTimeout = CONNECT_TIMEOUT_MILLIS
            connection.readTimeout = READ_TIMEOUT_MILLIS
            connection.useCaches = false
            connection.setRequestProperty("Accept", "application/json")
            connection.setRequestProperty("Accept-Language", "zh-CN")
            connection.setRequestProperty("Authorization", "Bearer ${session.accessToken}")
            if (body != null) {
                connection.doOutput = true
                connection.setRequestProperty("Content-Type", "application/json; charset=utf-8")
                connection.outputStream.bufferedWriter(Charsets.UTF_8).use { it.write(body.toString()) }
            }

            val status = connection.responseCode
            val responseText = readText(if (status in 200..299) connection.inputStream else connection.errorStream)
            return parseEnvelope(status, responseText)
        } finally {
            connection.disconnect()
        }
    }

    internal fun parseEnvelope(httpStatus: Int, responseText: String): Any? {
        val root = try {
            JSONObject(responseText)
        } catch (_: Exception) {
            throw NativeApiException(responseText.ifBlank { str(R.string.api_error_unparsable) })
        }
        val errorCode = root.optInt("errorCode", 0)
        val success = root.optBoolean("success", httpStatus in 200..299)
        if (httpStatus !in 200..299 || !success) {
            val message = root.optString("message").ifBlank {
                root.optJSONObject("error")?.optString("message").orEmpty()
            }.ifBlank { str(R.string.api_error_request_failed) }
            throw NativeApiException(
                message = message,
                errorCode = errorCode,
                unauthorized = httpStatus == 401 || errorCode == 3000 || errorCode == 3002,
            )
        }
        return root.opt("data").takeUnless { it == null || it === JSONObject.NULL }
    }

    private fun readText(stream: InputStream?): String {
        if (stream == null) return ""
        return BufferedReader(InputStreamReader(stream, Charsets.UTF_8)).use { it.readText() }
    }

    private fun isAllowedBaseUrl(value: String): Boolean {
        val uri = Uri.parse(value)
        if (uri.scheme.equals("https", ignoreCase = true) && !uri.host.isNullOrBlank()) return true
        if (!BuildConfig.DEBUG || !uri.scheme.equals("http", ignoreCase = true)) return false
        return uri.host in setOf("10.0.2.2", "127.0.0.1", "localhost")
    }

    private companion object {
        const val CONNECT_TIMEOUT_MILLIS = 15_000
        const val READ_TIMEOUT_MILLIS = 30_000
        const val MEDIA_TIMEOUT_MILLIS = 90_000
        const val MAX_DOWNLOAD_BYTES = 50L * 1024L * 1024L
        const val DEFAULT_PAGE_SIZE = 20
        const val MAX_PAGE_SIZE = 100
    }
}
