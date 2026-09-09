package com.digintelspace.remotehelpdesk.nativeportal

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.viewModels
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.core.view.WindowCompat
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen

class NativeCustomerGatewayActivity : ComponentActivity() {
    private val viewModel by viewModels<NativeCustomerGatewayViewModel>()

    override fun onCreate(savedInstanceState: Bundle?) {
        installSplashScreen()
        super.onCreate(savedInstanceState)
        WindowCompat.setDecorFitsSystemWindows(window, false)
        WindowCompat.getInsetsController(window, window.decorView).apply {
            isAppearanceLightStatusBars = true
            isAppearanceLightNavigationBars = true
        }
        setContent {
            val state by viewModel.state
            LaunchedEffect(state.authenticated) {
                if (state.authenticated) openPortal(state.initialTab, state.pendingServiceCode)
            }
            NativeCustomerGatewayApp(state = state, viewModel = viewModel)
        }
        if (savedInstanceState == null) applyRoute(intent)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        applyRoute(intent)
    }

    private fun applyRoute(intent: Intent?) {
        val route = NativeLaunchRoute.from(intent?.data)
        viewModel.initialize(route.tab, route.serviceCode)
    }

    private fun openPortal(initialTab: NativePortalTab, serviceCode: String) {
        startActivity(
            Intent(this, NativeCustomerPortalActivity::class.java)
                .putExtra(NativeCustomerPortalActivity.EXTRA_INITIAL_TAB, initialTab.wireValue)
                .putExtra(NativeCustomerPortalActivity.EXTRA_SERVICE_CODE, serviceCode),
        )
        finish()
    }
}

internal data class NativeLaunchRoute(
    val tab: NativePortalTab = NativePortalTab.CONVERSATIONS,
    val serviceCode: String = "",
) {
    companion object {
        fun from(uri: Uri?): NativeLaunchRoute {
            if (uri?.scheme != "remotehelpdesk") return NativeLaunchRoute()
            return when (uri.host) {
                "mobile" -> NativeLaunchRoute(
                    tab = NativePortalTab.fromWireValue(uri.getQueryParameter("state")),
                )
                "c" -> NativeLaunchRoute(
                    tab = NativePortalTab.DEVICES,
                    serviceCode = uri.pathSegments.firstOrNull().orEmpty().trim().uppercase(),
                )
                else -> NativeLaunchRoute()
            }
        }
    }
}
