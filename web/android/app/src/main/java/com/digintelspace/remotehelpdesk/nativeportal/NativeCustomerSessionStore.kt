package com.digintelspace.remotehelpdesk.nativeportal

import android.content.Context
import android.net.Uri
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import com.digintelspace.remotehelpdesk.BuildConfig
import java.security.KeyStore
import java.time.Instant
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

data class NativeCustomerSession(
    val apiBaseUrl: String,
    val accessToken: String,
    val expiresAt: String,
    val displayName: String,
)

class NativeCustomerSessionStore(context: Context) {
    private val preferences = context.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)

    fun save(session: NativeCustomerSession) {
        require(session.accessToken.isNotBlank()) { "access token is required" }
        require(isAllowedBaseUrl(session.apiBaseUrl)) { "API base URL must use HTTPS" }
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, secretKey())
        val encrypted = cipher.doFinal(session.accessToken.toByteArray(Charsets.UTF_8))

        val committed = preferences.edit()
            .putString(KEY_API_BASE_URL, session.apiBaseUrl.trimEnd('/'))
            .putString(KEY_TOKEN_IV, Base64.encodeToString(cipher.iv, Base64.NO_WRAP))
            .putString(KEY_TOKEN_DATA, Base64.encodeToString(encrypted, Base64.NO_WRAP))
            .putString(KEY_EXPIRES_AT, session.expiresAt)
            .putString(KEY_DISPLAY_NAME, session.displayName)
            .commit()
        check(committed) { "native customer session could not be persisted" }
    }

    fun read(): NativeCustomerSession? {
        val apiBaseUrl = preferences.getString(KEY_API_BASE_URL, null)?.trim().orEmpty()
        val iv = preferences.getString(KEY_TOKEN_IV, null)?.trim().orEmpty()
        val tokenData = preferences.getString(KEY_TOKEN_DATA, null)?.trim().orEmpty()
        if (apiBaseUrl.isBlank() || iv.isBlank() || tokenData.isBlank()) return null

        return try {
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(
                Cipher.DECRYPT_MODE,
                secretKey(),
                GCMParameterSpec(128, Base64.decode(iv, Base64.NO_WRAP)),
            )
            val accessToken = cipher.doFinal(Base64.decode(tokenData, Base64.NO_WRAP))
                .toString(Charsets.UTF_8)
            val expiresAt = preferences.getString(KEY_EXPIRES_AT, "").orEmpty()
            if (isExpired(expiresAt)) {
                clear()
                return null
            }
            NativeCustomerSession(
                apiBaseUrl = apiBaseUrl,
                accessToken = accessToken,
                expiresAt = expiresAt,
                displayName = preferences.getString(KEY_DISPLAY_NAME, "").orEmpty(),
            )
        } catch (_: Exception) {
            clear()
            null
        }
    }

    fun clear() {
        preferences.edit().clear().apply()
    }

    private fun secretKey(): SecretKey {
        val keyStore = KeyStore.getInstance(ANDROID_KEY_STORE).apply { load(null) }
        (keyStore.getKey(KEY_ALIAS, null) as? SecretKey)?.let { return it }

        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, ANDROID_KEY_STORE).run {
            init(
                KeyGenParameterSpec.Builder(
                    KEY_ALIAS,
                    KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
                )
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .build(),
            )
            generateKey()
        }
    }

    private fun isAllowedBaseUrl(value: String): Boolean {
        val uri = Uri.parse(value.trim())
        if (uri.scheme.equals("https", ignoreCase = true) && !uri.host.isNullOrBlank()) return true
        return BuildConfig.DEBUG &&
            uri.scheme.equals("http", ignoreCase = true) &&
            uri.host in setOf("10.0.2.2", "127.0.0.1", "localhost")
    }

    private fun isExpired(value: String): Boolean {
        if (value.isBlank()) return false
        return runCatching { Instant.parse(value).isBefore(Instant.now()) }.getOrDefault(false)
    }

    private companion object {
        const val PREFERENCES_NAME = "native_customer_session"
        const val ANDROID_KEY_STORE = "AndroidKeyStore"
        const val KEY_ALIAS = "remote_help_desk_customer_session_v1"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val KEY_API_BASE_URL = "api_base_url"
        const val KEY_TOKEN_IV = "token_iv"
        const val KEY_TOKEN_DATA = "token_data"
        const val KEY_EXPIRES_AT = "expires_at"
        const val KEY_DISPLAY_NAME = "display_name"
    }
}
