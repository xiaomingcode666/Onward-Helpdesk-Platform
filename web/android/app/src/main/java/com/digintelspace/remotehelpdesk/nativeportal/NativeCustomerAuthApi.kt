package com.digintelspace.remotehelpdesk.nativeportal

import android.content.Context
import android.net.Uri
import androidx.annotation.StringRes
import com.digintelspace.remotehelpdesk.BuildConfig
import com.digintelspace.remotehelpdesk.R
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStream
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL

enum class NativeRegistrationMethod(val wireValue: String) {
    INVITE("invite"),
    SERVICE_CODE("service_code"),
}

data class NativeRegistrationContext(
    val method: NativeRegistrationMethod,
    val displayName: String,
    val email: String,
    val tenantName: String,
    val customerOrgName: String,
    val productName: String,
    val deviceNo: String,
)

class NativeCustomerAuthApi(
    private val context: Context,
    private val apiBaseUrl: String = BuildConfig.RHD_API_BASE_URL,
) {
    init {
        require(isAllowedBaseUrl(apiBaseUrl)) { "Android API 地址必须使用 HTTPS" }
    }

    private fun str(@StringRes id: Int): String = context.getString(id)

    fun login(username: String, password: String): NativeCustomerSession = parseSession(
        requestObject(
            path = "/api/auth/login",
            body = JSONObject()
                .put("username", username)
                .put("password", password)
                .put("domainType", "customer")
                .put("portalChoiceConfirmed", true),
        ),
    )

    fun verifyRegistration(
        method: NativeRegistrationMethod,
        credential: String,
    ): NativeRegistrationContext {
        val data = requestObject(
            path = "/api/auth/customer-registration/verify",
            body = JSONObject()
                .put("method", method.wireValue)
                .put("credential", credential),
        )
        return NativeRegistrationContext(
            method = method,
            displayName = data.optString("displayName"),
            email = data.optString("email"),
            tenantName = data.optJSONObject("tenant")?.optString("name").orEmpty(),
            customerOrgName = data.optJSONObject("customerOrg")?.optString("name").orEmpty(),
            productName = data.optJSONObject("product")?.optString("name").orEmpty(),
            deviceNo = data.optJSONObject("device")?.optString("deviceNo").orEmpty(),
        )
    }

    fun register(
        method: NativeRegistrationMethod,
        credential: String,
        username: String,
        displayName: String,
        email: String,
        password: String,
    ): NativeCustomerSession = parseSession(
        requestObject(
            path = "/api/auth/customer-registration/register",
            body = JSONObject()
                .put("method", method.wireValue)
                .put("credential", credential)
                .put("username", username)
                .put("displayName", displayName)
                .put("email", email)
                .put("password", password),
        ),
    )

    private fun parseSession(data: JSONObject): NativeCustomerSession {
        val domainType = data.optString("domainType").ifBlank { data.optString("domain_type") }
        if (domainType != "customer") {
            throw NativeApiException(str(R.string.auth_error_not_customer))
        }
        val accessToken = data.optString("accessToken").ifBlank { data.optString("access_token") }
        if (accessToken.isBlank()) throw NativeApiException(str(R.string.auth_error_no_credentials))
        val user = data.optJSONObject("user") ?: JSONObject()
        val displayName = user.optString("nickname")
            .ifBlank { user.optString("username") }
            .ifBlank { str(R.string.auth_display_name_default) }
        return NativeCustomerSession(
            apiBaseUrl = apiBaseUrl.trimEnd('/'),
            accessToken = accessToken,
            expiresAt = data.optString("expiresAt").ifBlank { data.optString("expires_at") },
            displayName = displayName,
        )
    }

    private fun requestObject(path: String, body: JSONObject): JSONObject {
        val connection = URL("${apiBaseUrl.trimEnd('/')}$path").openConnection() as HttpURLConnection
        try {
            connection.requestMethod = "POST"
            connection.connectTimeout = CONNECT_TIMEOUT_MILLIS
            connection.readTimeout = READ_TIMEOUT_MILLIS
            connection.useCaches = false
            connection.doOutput = true
            connection.setRequestProperty("Accept", "application/json")
            connection.setRequestProperty("Accept-Language", "zh-CN")
            connection.setRequestProperty("Content-Type", "application/json; charset=utf-8")
            connection.outputStream.bufferedWriter(Charsets.UTF_8).use { it.write(body.toString()) }

            val status = connection.responseCode
            val responseText = readText(if (status in 200..299) connection.inputStream else connection.errorStream)
            return parseEnvelope(status, responseText)
        } finally {
            connection.disconnect()
        }
    }

    internal fun parseEnvelope(httpStatus: Int, responseText: String): JSONObject {
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
            }.ifBlank { str(R.string.auth_error_request_failed) }
            throw NativeApiException(
                message = message,
                errorCode = errorCode,
                unauthorized = httpStatus == 401 || errorCode == 3000 || errorCode == 3002,
            )
        }
        return root.optJSONObject("data") ?: throw NativeApiException(str(R.string.auth_error_empty_data))
    }

    private fun readText(stream: InputStream?): String {
        if (stream == null) return ""
        return BufferedReader(InputStreamReader(stream, Charsets.UTF_8)).use { it.readText() }
    }

    private fun isAllowedBaseUrl(value: String): Boolean {
        val uri = Uri.parse(value.trim())
        if (uri.scheme.equals("https", ignoreCase = true) && !uri.host.isNullOrBlank()) return true
        return BuildConfig.DEBUG &&
            uri.scheme.equals("http", ignoreCase = true) &&
            uri.host in setOf("10.0.2.2", "127.0.0.1", "localhost")
    }

    private companion object {
        const val CONNECT_TIMEOUT_MILLIS = 15_000
        const val READ_TIMEOUT_MILLIS = 30_000
    }
}
