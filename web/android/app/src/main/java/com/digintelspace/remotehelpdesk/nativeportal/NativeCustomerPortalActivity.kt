package com.digintelspace.remotehelpdesk.nativeportal

import android.Manifest
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Intent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.IntentFilter
import android.os.Bundle
import android.os.Build
import android.content.pm.PackageManager
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.OnBackPressedCallback
import androidx.activity.viewModels
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import com.digintelspace.remotehelpdesk.glasses.GlassesConnectionActivity
import androidx.localbroadcastmanager.content.LocalBroadcastManager
import androidx.core.content.ContextCompat
import com.digintelspace.remotehelpdesk.BuildConfig
import com.digintelspace.remotehelpdesk.R
import com.google.firebase.messaging.FirebaseMessaging
import org.jitsi.meet.sdk.BroadcastEvent
import org.jitsi.meet.sdk.JitsiMeetActivity
import org.jitsi.meet.sdk.JitsiMeetConferenceOptions
import java.net.URL

class NativeCustomerPortalActivity : ComponentActivity() {
    private val viewModel by viewModels<NativeCustomerPortalViewModel>()
    private var activeMeetingId = ""
    private val notificationPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        if (granted) registerPushToken()
    }
    private val jitsiBroadcastReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            if (intent == null || activeMeetingId.isBlank()) return
            when (BroadcastEvent(intent).type) {
                BroadcastEvent.Type.CONFERENCE_JOINED -> viewModel.meetingConferenceJoined(activeMeetingId)
                BroadcastEvent.Type.CONFERENCE_TERMINATED -> {
                    viewModel.meetingConferenceLeft(activeMeetingId)
                    activeMeetingId = ""
                }
                else -> Unit
            }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val initialTab = NativePortalTab.fromWireValue(intent.getStringExtra(EXTRA_INITIAL_TAB))

        setContent {
            val state by viewModel.state

            LaunchedEffect(state.authExpired, state.accountDeleted) {
                when {
                    state.accountDeleted -> clearFirebaseTokenAndReturn(accountDeleted = true)
                    state.authExpired -> returnToGateway()
                }
            }

            LaunchedEffect(state.meetingJoinConfig) {
                state.meetingJoinConfig?.let { config ->
                    launchJitsiMeeting(config)
                    viewModel.consumeMeetingJoinConfig()
                }
            }

            NativeCustomerPortalApp(
                state = state,
                displayName = viewModel.displayName,
                viewModel = viewModel,
                onLogout = {
                    viewModel.logout { clearFirebaseTokenAndReturn() }
                },
                onOpenGlasses = {
                    startActivity(Intent(this, GlassesConnectionActivity::class.java))
                },
            )
        }

        if (savedInstanceState == null) {
            viewModel.initialize(initialTab, intent.getStringExtra(EXTRA_SERVICE_CODE).orEmpty())
        }
        registerJitsiBroadcasts()
        configureNativePush()
        onBackPressedDispatcher.addCallback(
            this,
            object : OnBackPressedCallback(true) {
                override fun handleOnBackPressed() {
                    if (!viewModel.clearCurrentSelection()) moveTaskToBack(true)
                }
            },
        )
    }

    override fun onStart() {
        super.onStart()
        viewModel.setForeground(true)
    }

    override fun onStop() {
        viewModel.setForeground(false)
        super.onStop()
    }

    override fun onDestroy() {
        LocalBroadcastManager.getInstance(this).unregisterReceiver(jitsiBroadcastReceiver)
        super.onDestroy()
    }

    private fun returnToGateway(accountDeleted: Boolean = false) {
        val intent = Intent(this, NativeCustomerGatewayActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TASK
            if (accountDeleted) putExtra(EXTRA_ACCOUNT_DELETED, true)
        }
        startActivity(intent)
        finish()
    }

    private fun configureNativePush() {
        if (!BuildConfig.RHD_PUSH_CONFIGURED) return
        val manager = getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(
            NotificationChannel(
                getString(R.string.default_notification_channel_id),
                getString(R.string.notification_channel_name),
                NotificationManager.IMPORTANCE_HIGH,
            ).apply { description = getString(R.string.notification_channel_description) },
        )
        if (
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
        } else {
            registerPushToken()
        }
    }

    private fun registerPushToken() {
        if (!BuildConfig.RHD_PUSH_CONFIGURED) return
        FirebaseMessaging.getInstance().apply {
            isAutoInitEnabled = true
            register()
        }
    }

    private fun clearFirebaseTokenAndReturn(accountDeleted: Boolean = false) {
        if (BuildConfig.RHD_PUSH_CONFIGURED) {
            FirebaseMessaging.getInstance().apply {
                isAutoInitEnabled = false
                unregister()
            }
        }
        returnToGateway(accountDeleted)
    }

    private fun registerJitsiBroadcasts() {
        val filter = IntentFilter().apply {
            addAction(BroadcastEvent.Type.CONFERENCE_JOINED.action)
            addAction(BroadcastEvent.Type.CONFERENCE_TERMINATED.action)
        }
        LocalBroadcastManager.getInstance(this).registerReceiver(jitsiBroadcastReceiver, filter)
    }

    private fun launchJitsiMeeting(config: NativeMeetingJoinConfig) {
        if (config.roomName.isBlank()) return
        val domain = config.domain.trim().ifBlank {
            config.jitsiUrl.substringBefore("/${config.roomName}")
        }
        val serverUrl = runCatching {
            URL(if (domain.startsWith("http://") || domain.startsWith("https://")) domain else "https://$domain")
        }.getOrNull() ?: return

        activeMeetingId = config.meetingId
        val builder = JitsiMeetConferenceOptions.Builder()
            .setServerURL(serverUrl)
            .setRoom(config.roomName)
            .setFeatureFlag("welcomepage.enabled", false)
            .setFeatureFlag("prejoinpage.enabled", false)
            .setFeatureFlag("pip.enabled", true)
        if (config.jwt.isNotBlank()) builder.setToken(config.jwt)
        JitsiMeetActivity.launch(this, builder.build())
    }

    companion object {
        const val EXTRA_INITIAL_TAB = "initial_tab"
        const val EXTRA_SERVICE_CODE = "service_code"
        const val EXTRA_ACCOUNT_DELETED = "account_deleted"
    }
}
