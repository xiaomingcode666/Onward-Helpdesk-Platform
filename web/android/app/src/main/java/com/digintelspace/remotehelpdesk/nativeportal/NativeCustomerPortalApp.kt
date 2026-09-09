package com.digintelspace.remotehelpdesk.nativeportal

import androidx.compose.foundation.background
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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.automirrored.outlined.Assignment
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.outlined.ChatBubbleOutline
import androidx.compose.material.icons.outlined.PersonOutline
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material.icons.outlined.Videocam
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.digintelspace.remotehelpdesk.R
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

internal val PortalInk = Color(0xFF172033)
internal val PortalMuted = Color(0xFF6F7B8E)
internal val PortalBlue = Color(0xFF1769E0)
internal val PortalBackground = Color(0xFFF4F6F8)
internal val PortalBorder = Color(0xFFE1E6EC)
internal val PortalSuccess = Color(0xFF087D61)
internal val PortalDanger = Color(0xFFC73737)

private data class PortalNavItem(
    val tab: NativePortalTab,
    val label: String,
    val icon: ImageVector,
)

@Composable
private fun portalNavItems(): List<PortalNavItem> = listOf(
    PortalNavItem(NativePortalTab.CONVERSATIONS, stringResource(R.string.portal_tab_conversations), Icons.Outlined.ChatBubbleOutline),
    PortalNavItem(NativePortalTab.TICKETS, stringResource(R.string.portal_tab_tickets), Icons.AutoMirrored.Outlined.Assignment),
    PortalNavItem(NativePortalTab.MEETINGS, stringResource(R.string.portal_tab_meetings), Icons.Outlined.Videocam),
    PortalNavItem(NativePortalTab.DEVICES, stringResource(R.string.portal_tab_devices), Icons.Outlined.Build),
    PortalNavItem(NativePortalTab.PROFILE, stringResource(R.string.portal_tab_profile), Icons.Outlined.PersonOutline),
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun NativeCustomerPortalApp(
    state: NativePortalUiState,
    displayName: String,
    viewModel: NativeCustomerPortalViewModel,
    onLogout: () -> Unit,
    onOpenGlasses: () -> Unit,
) {
    val detailVisible = state.hasDetailSelection()
    val title = state.detailTitle() ?: stringResource(state.selectedTab.titleRes)
    val focusManager = LocalFocusManager.current
    val keyboardController = LocalSoftwareKeyboardController.current

    MaterialTheme(
        colorScheme = MaterialTheme.colorScheme.copy(
            primary = PortalBlue,
            onPrimary = Color.White,
            background = PortalBackground,
            surface = Color.White,
            onSurface = PortalInk,
            error = PortalDanger,
        ),
    ) {
        Surface(modifier = Modifier.fillMaxSize(), color = PortalBackground) {
            Scaffold(
                containerColor = PortalBackground,
                topBar = {
                    TopAppBar(
                        title = {
                            Column {
                                if (!detailVisible) {
                                    Text(
                                        text = "RemoteHelpDesk",
                                        color = PortalMuted,
                                        fontSize = 10.sp,
                                        lineHeight = 12.sp,
                                    )
                                }
                                Text(
                                    text = title,
                                    color = PortalInk,
                                    fontWeight = FontWeight.SemiBold,
                                    fontSize = if (detailVisible) 17.sp else 20.sp,
                                    lineHeight = 24.sp,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                )
                            }
                        },
                        navigationIcon = {
                            if (detailVisible) {
                                IconButton(onClick = { viewModel.clearCurrentSelection() }) {
                                    Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = stringResource(R.string.portal_back_to_list))
                                }
                            }
                        },
                        actions = {
                            if (!detailVisible) {
                                IconButton(onClick = viewModel::reloadCurrent, enabled = !state.loading && !state.busy) {
                                    Icon(Icons.Outlined.Refresh, contentDescription = stringResource(R.string.portal_refresh))
                                }
                            }
                        },
                        colors = TopAppBarDefaults.topAppBarColors(containerColor = Color.White),
                    )
                },
                bottomBar = {
                    if (!detailVisible) {
                        NavigationBar(
                            modifier = Modifier.imePadding(),
                            containerColor = Color.White,
                            tonalElevation = 0.dp,
                        ) {
                            portalNavItems().forEach { item ->
                                NavigationBarItem(
                                    selected = state.selectedTab == item.tab,
                                    onClick = {
                                        keyboardController?.hide()
                                        focusManager.clearFocus(force = true)
                                        viewModel.selectTab(item.tab)
                                    },
                                    icon = { Icon(item.icon, contentDescription = null, modifier = Modifier.size(22.dp)) },
                                    label = { Text(item.label, fontSize = 10.sp) },
                                    alwaysShowLabel = true,
                                    colors = NavigationBarItemDefaults.colors(
                                        selectedIconColor = PortalBlue,
                                        selectedTextColor = PortalBlue,
                                        indicatorColor = Color(0xFFEDF4FF),
                                        unselectedIconColor = Color(0xFF7B8494),
                                        unselectedTextColor = Color(0xFF7B8494),
                                    ),
                                )
                            }
                        }
                    }
                },
            ) { padding ->
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(padding),
                ) {
                    when (state.selectedTab) {
                        NativePortalTab.CONVERSATIONS -> NativeConversationScreen(state, viewModel)
                        NativePortalTab.TICKETS -> NativeTicketsScreen(state, viewModel)
                        NativePortalTab.MEETINGS -> NativeMeetingsScreen(state, viewModel)
                        NativePortalTab.DEVICES -> NativeDevicesScreen(state, viewModel)
                        NativePortalTab.PROFILE -> NativeProfileScreen(
                            state = state,
                            displayName = displayName,
                            viewModel = viewModel,
                            onLogout = onLogout,
                            onOpenGlasses = onOpenGlasses,
                        )
                    }

                    if (state.loading) {
                        Box(
                            modifier = Modifier
                                .fillMaxSize()
                                .background(Color.White.copy(alpha = 0.78f)),
                            contentAlignment = Alignment.Center,
                        ) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                CircularProgressIndicator(modifier = Modifier.size(28.dp), strokeWidth = 2.5.dp)
                                Spacer(Modifier.height(12.dp))
                                Text(stringResource(R.string.portal_loading_data), color = PortalMuted, fontSize = 12.sp)
                            }
                        }
                    }
                }
            }
        }
    }
}

private fun NativePortalUiState.hasDetailSelection(): Boolean = when (selectedTab) {
    NativePortalTab.CONVERSATIONS -> selectedConversationId > 0
    NativePortalTab.TICKETS -> selectedTicketId > 0
    NativePortalTab.MEETINGS -> selectedMeetingId.isNotBlank()
    NativePortalTab.DEVICES -> selectedDeviceId > 0
    NativePortalTab.PROFILE -> false
}

@Composable
private fun NativePortalUiState.detailTitle(): String? = when (selectedTab) {
    NativePortalTab.CONVERSATIONS -> selectedConversation?.let { it.deviceNo.ifBlank { stringResource(R.string.portal_detail_conversation_title, it.id) } }
    NativePortalTab.TICKETS -> selectedTicket?.let { it.ticketNo.ifBlank { stringResource(R.string.portal_detail_ticket_title) } }
    NativePortalTab.MEETINGS -> selectedMeeting?.let { it.ticketNo.ifBlank { stringResource(R.string.portal_detail_meeting_title) } }
    NativePortalTab.DEVICES -> selectedDevice?.let { it.productName.ifBlank { stringResource(R.string.portal_detail_device_title) } }
    NativePortalTab.PROFILE -> null
}

@Composable
internal fun PortalMessageBanner(state: NativePortalUiState, onRetry: () -> Unit) {
    when {
        state.error.isNotBlank() -> {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(Color(0xFFFFEEEE))
                    .padding(horizontal = 16.dp, vertical = 10.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    state.error,
                    modifier = Modifier.weight(1f),
                    color = PortalDanger,
                    fontSize = 12.sp,
                    lineHeight = 17.sp,
                )
                androidx.compose.material3.TextButton(onClick = onRetry) { Text(stringResource(R.string.common_retry)) }
            }
        }
        state.notice.isNotBlank() -> {
            Text(
                state.notice,
                modifier = Modifier
                    .fillMaxWidth()
                    .background(Color(0xFFEAF7F2))
                    .padding(horizontal = 16.dp, vertical = 10.dp),
                color = PortalSuccess,
                fontSize = 12.sp,
            )
        }
    }
}

@Composable
internal fun PortalEmptyState(title: String, detail: String) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 28.dp, vertical = 56.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(title, color = PortalInk, fontWeight = FontWeight.SemiBold, fontSize = 15.sp)
        Spacer(Modifier.height(7.dp))
        Text(detail, color = PortalMuted, fontSize = 12.sp, lineHeight = 18.sp)
    }
}

private val statusLabelRes: Map<String, Int> = mapOf(
    "active" to R.string.status_active,
    "accepted" to R.string.status_accepted,
    "action_required" to R.string.status_action_required,
    "ai_serving" to R.string.status_ai_serving,
    "assigned" to R.string.status_assigned,
    "cancelled" to R.string.status_cancelled,
    "closed" to R.string.status_closed,
    "completed" to R.string.status_completed,
    "done" to R.string.status_done,
    "ended" to R.string.status_ended,
    "failed" to R.string.status_failed,
    "finished" to R.string.status_finished,
    "human_serving" to R.string.status_human_serving,
    "idle" to R.string.status_idle,
    "in_progress" to R.string.status_in_progress,
    "joined" to R.string.status_joined,
    "maintenance" to R.string.status_maintenance,
    "offline" to R.string.status_offline,
    "online" to R.string.status_online,
    "open" to R.string.status_open,
    "pending" to R.string.status_pending,
    "pending_acceptance" to R.string.status_pending_acceptance,
    "pending_assignee_accept" to R.string.status_pending_assignee_accept,
    "pending_confirmation" to R.string.status_pending_confirmation,
    "pending_dispatch" to R.string.status_pending_dispatch,
    "processing" to R.string.status_processing,
    "queued" to R.string.status_queued,
    "reopened" to R.string.status_reopened,
    "resolved" to R.string.status_resolved,
    "reviewing" to R.string.status_reviewing,
    "scheduled" to R.string.status_scheduled,
    "submitted" to R.string.status_submitted,
    "waiting" to R.string.status_waiting,
    "waiting_customer" to R.string.status_waiting_customer,
)

@Composable
internal fun PortalStatusPill(status: String) {
    val normalized = status.lowercase()
    val label = statusLabelRes[normalized]?.let { stringResource(it) } ?: status.ifBlank { stringResource(R.string.status_unknown) }
    val (color, background) = when (normalized) {
        "active", "accepted", "human_serving", "in_progress", "joined", "online", "processing" ->
            PortalSuccess to Color(0xFFEAF7F2)
        "action_required", "ai_serving", "pending", "pending_acceptance", "pending_assignee_accept",
        "pending_confirmation", "pending_dispatch", "queued", "reopened", "reviewing", "scheduled",
        "submitted", "waiting", "waiting_customer" -> Color(0xFF9A6700) to Color(0xFFFFF5D7)
        "cancelled", "failed" -> PortalDanger to Color(0xFFFFEEEE)
        else -> PortalMuted to Color(0xFFEEF1F4)
    }
    Text(
        text = label,
        modifier = Modifier
            .background(background, RoundedCornerShape(5.dp))
            .padding(horizontal = 8.dp, vertical = 4.dp),
        color = color,
        fontSize = 10.sp,
        lineHeight = 12.sp,
        maxLines = 1,
    )
}

@Composable
internal fun PortalSectionTitle(title: String) {
    Text(
        title,
        color = PortalMuted,
        fontWeight = FontWeight.Medium,
        fontSize = 11.sp,
        modifier = Modifier.padding(bottom = 8.dp),
    )
}

@Composable
internal fun PortalKeyValue(label: String, value: String) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 12.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.Top,
    ) {
        Text(label, color = PortalMuted, fontSize = 13.sp)
        Text(
            value.ifBlank { "--" },
            modifier = Modifier
                .padding(start = 20.dp)
                .weight(1f),
            color = PortalInk,
            fontSize = 13.sp,
            fontWeight = FontWeight.Medium,
            maxLines = 3,
            overflow = TextOverflow.Ellipsis,
        )
    }
    HorizontalDivider(color = Color(0xFFEDF0F4))
}

internal fun displayDate(value: String): String {
    if (value.isBlank()) return "--"
    return runCatching {
        Instant.parse(value)
            .atZone(ZoneId.systemDefault())
            .format(DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm"))
    }.getOrElse { value.replace('T', ' ').replace("Z", "").take(16) }
}

@Composable
internal fun displayPriority(value: String): String = when (value.lowercase()) {
    "low" -> stringResource(R.string.priority_low)
    "medium", "normal" -> stringResource(R.string.priority_medium)
    "high" -> stringResource(R.string.priority_high)
    "urgent", "critical" -> stringResource(R.string.priority_urgent)
    else -> value.ifBlank { "--" }
}

@Composable
internal fun displayDuration(seconds: Int): String {
    if (seconds <= 0) return "--"
    val minutes = seconds / 60
    val remainder = seconds % 60
    return if (minutes > 0) {
        stringResource(R.string.duration_minutes_seconds, minutes, remainder)
    } else {
        stringResource(R.string.duration_seconds, remainder)
    }
}
