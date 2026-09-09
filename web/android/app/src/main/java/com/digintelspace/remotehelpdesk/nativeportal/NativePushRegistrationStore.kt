package com.digintelspace.remotehelpdesk.nativeportal

import android.content.Context
import java.util.UUID

class NativePushRegistrationStore(context: Context) {
    private val preferences = context.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)

    fun deviceId(): String {
        preferences.getString(KEY_DEVICE_ID, null)?.takeIf { it.isNotBlank() }?.let { return it }
        val generated = UUID.randomUUID().toString()
        preferences.edit().putString(KEY_DEVICE_ID, generated).commit()
        return generated
    }

    fun tokenId(): Long = preferences.getLong(KEY_TOKEN_ID, 0L)

    fun saveTokenId(value: Long) {
        if (value > 0) preferences.edit().putLong(KEY_TOKEN_ID, value).apply()
    }

    fun clearTokenId() {
        preferences.edit().remove(KEY_TOKEN_ID).apply()
    }

    private companion object {
        const val PREFERENCES_NAME = "native_mobile_push"
        const val KEY_DEVICE_ID = "device_id"
        const val KEY_TOKEN_ID = "token_id"
    }
}
