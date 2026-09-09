package com.digintelspace.remotehelpdesk.nativeportal

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.core.app.NotificationCompat
import com.digintelspace.remotehelpdesk.BuildConfig
import com.digintelspace.remotehelpdesk.R
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage
import java.util.concurrent.Executors
import kotlin.math.absoluteValue

class RemoteHelpDeskFirebaseMessagingService : FirebaseMessagingService() {
    private val executor = Executors.newSingleThreadExecutor()

    override fun onRegistered(installationId: String) {
        super.onRegistered(installationId)
        if (!BuildConfig.RHD_PUSH_CONFIGURED || installationId.isBlank()) return
        executor.execute {
            val session = NativeCustomerSessionStore(this).read() ?: return@execute
            val registrationStore = NativePushRegistrationStore(this)
            runCatching {
                NativeCustomerApi(session, this).registerPushToken(
                    token = installationId,
                    deviceId = registrationStore.deviceId(),
                    appVersion = BuildConfig.VERSION_NAME,
                )
            }.onSuccess(registrationStore::saveTokenId)
        }
    }

    override fun onUnregistered(installationId: String) {
        NativePushRegistrationStore(this).clearTokenId()
        super.onUnregistered(installationId)
    }

    override fun onMessageReceived(message: RemoteMessage) {
        super.onMessageReceived(message)
        val title = message.notification?.title
            ?: message.data["title"]
            ?: "RemoteHelpDesk"
        val body = message.notification?.body
            ?: message.data["body"]
            ?: message.data["message"]
            ?: getString(R.string.notification_fallback_body)
        showNotification(title, body, message.data)
    }

    override fun onDestroy() {
        executor.shutdownNow()
        super.onDestroy()
    }

    private fun showNotification(title: String, body: String, data: Map<String, String>) {
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        manager.createNotificationChannel(
            NotificationChannel(
                CHANNEL_ID,
                getString(R.string.notification_channel_name),
                NotificationManager.IMPORTANCE_HIGH,
            ).apply { description = getString(R.string.notification_channel_description) },
        )
        val target = notificationTarget(data)
        val requestCode = (data["notification_id"] ?: data["conversationId"] ?: body).hashCode().absoluteValue
        val pendingIntent = PendingIntent.getActivity(
            this,
            requestCode,
            Intent(this, NativeCustomerGatewayActivity::class.java).apply {
                action = Intent.ACTION_VIEW
                this.data = target
                flags = Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP
            },
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        val notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_remotehelpdesk)
            .setContentTitle(title)
            .setContentText(body)
            .setStyle(NotificationCompat.BigTextStyle().bigText(body))
            .setContentIntent(pendingIntent)
            .setAutoCancel(true)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .build()
        manager.notify(requestCode, notification)
    }

    private fun notificationTarget(data: Map<String, String>): Uri {
        val actionUrl = data["action_url"] ?: data["actionUrl"] ?: ""
        val state = when {
            "state=tickets" in actionUrl || data["ticketId"].orEmpty().isNotBlank() -> "tickets"
            "state=video" in actionUrl || data["meetingId"].orEmpty().isNotBlank() -> "video"
            "state=devices" in actionUrl || data["deviceId"].orEmpty().isNotBlank() -> "devices"
            "state=my" in actionUrl -> "my"
            else -> "chat"
        }
        return Uri.parse("remotehelpdesk://mobile?state=$state")
    }

    private companion object {
        const val CHANNEL_ID = "conversation_updates"
    }
}
