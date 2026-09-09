package com.digintelspace.remotehelpdesk.nativeportal

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.KeyboardArrowRight
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Star
import androidx.compose.material.icons.outlined.Videocam
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.digintelspace.remotehelpdesk.R

private enum class TicketFilter { ALL, OPEN, CLOSED }
private enum class MeetingFilter { UPCOMING, HISTORY }

@Composable
internal fun NativeTicketsScreen(
    state: NativePortalUiState,
    viewModel: NativeCustomerPortalViewModel,
) {
    state.selectedTicket?.let {
        TicketDetail(state, it, viewModel)
        return
    }

    var query by rememberSaveable { mutableStateOf("") }
    var filter by rememberSaveable { mutableStateOf(TicketFilter.ALL) }
    val normalizedQuery = query.trim().lowercase()
    val filtered = remember(state.tickets, normalizedQuery, filter) {
        state.tickets.filter { item ->
            val open = isOpenTicket(item.status)
            val filterMatches = when (filter) {
                TicketFilter.ALL -> true
                TicketFilter.OPEN -> open
                TicketFilter.CLOSED -> !open
            }
            val queryMatches = normalizedQuery.isBlank() || listOf(
                item.ticketNo,
                item.title,
                item.deviceNo,
                item.productName,
            ).joinToString(" ").lowercase().contains(normalizedQuery)
            filterMatches && queryMatches
        }
    }

    Column(Modifier.fillMaxSize()) {
        PortalMessageBanner(state, viewModel::reloadCurrent)
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 14.dp, vertical = 12.dp),
        ) {
            OutlinedTextField(
                value = query,
                onValueChange = { query = it },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                leadingIcon = { Icon(Icons.Outlined.Search, contentDescription = null, modifier = Modifier.size(19.dp)) },
                trailingIcon = {
                    if (query.isNotBlank()) {
                        androidx.compose.material3.IconButton(onClick = { query = "" }) {
                            Icon(Icons.Outlined.Close, contentDescription = stringResource(R.string.common_clear_search), modifier = Modifier.size(17.dp))
                        }
                    }
                },
                placeholder = { Text(stringResource(R.string.ticket_search_placeholder), fontSize = 13.sp) },
                shape = RoundedCornerShape(7.dp),
            )
            Spacer(Modifier.height(10.dp))
            PortalSegmentedControl(
                options = listOf(
                    stringResource(R.string.ticket_filter_all),
                    stringResource(R.string.ticket_filter_open),
                    stringResource(R.string.ticket_filter_closed),
                ),
                selectedIndex = filter.ordinal,
                onSelected = { filter = TicketFilter.entries[it] },
            )
        }
        HorizontalDivider(color = PortalBorder)

        if (filtered.isEmpty() && !state.loading && !state.ticketsHasMore) {
            PortalEmptyState(stringResource(R.string.ticket_empty_title), stringResource(R.string.ticket_empty_detail))
        } else {
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = androidx.compose.foundation.layout.PaddingValues(12.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                items(filtered, key = { it.id }) { ticket ->
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(Color.White, RoundedCornerShape(7.dp))
                            .clickable { viewModel.selectTicket(ticket.id) }
                            .padding(horizontal = 14.dp, vertical = 13.dp),
                    ) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                ticket.ticketNo.ifBlank { stringResource(R.string.ticket_id_label, ticket.id) },
                                modifier = Modifier.weight(1f),
                                color = PortalInk,
                                fontSize = 14.sp,
                                fontWeight = FontWeight.SemiBold,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                            PortalStatusPill(ticket.status)
                        }
                        Spacer(Modifier.height(7.dp))
                        Text(
                            ticketSummaryTitle(ticket),
                            color = Color(0xFF3F4A5D),
                            fontSize = 13.sp,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                        Spacer(Modifier.height(9.dp))
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                ticket.deviceNo.ifBlank { stringResource(R.string.common_no_device) },
                                modifier = Modifier.weight(1f),
                                color = PortalMuted,
                                fontSize = 10.sp,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                            Text(displayDate(ticket.updatedAt), color = PortalMuted, fontSize = 10.sp)
                            Spacer(Modifier.width(4.dp))
                            Icon(
                                Icons.AutoMirrored.Outlined.KeyboardArrowRight,
                                contentDescription = stringResource(R.string.ticket_open_detail),
                                tint = PortalMuted,
                                modifier = Modifier.size(17.dp),
                            )
                        }
                    }
                }
                if (state.ticketsHasMore) {
                    item(key = "load-more-tickets") {
                        PortalLoadMoreRow(NativePortalTab.TICKETS, state, viewModel)
                    }
                }
            }
        }
    }
}

@Composable
private fun TicketDetail(
    state: NativePortalUiState,
    ticket: NativeTicket,
    viewModel: NativeCustomerPortalViewModel,
) {
    var reopenDialog by rememberSaveable { mutableStateOf(false) }
    var reopenReason by rememberSaveable(ticket.id) { mutableStateOf("") }
    var feedbackDialog by rememberSaveable { mutableStateOf(false) }
    var feedbackRating by rememberSaveable(ticket.id) { mutableStateOf(5) }
    var feedbackComment by rememberSaveable(ticket.id) { mutableStateOf("") }

    Column(Modifier.fillMaxSize()) {
        PortalMessageBanner(state, viewModel::reloadCurrent)
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(16.dp),
        ) {
            item {
                Row(verticalAlignment = Alignment.Top) {
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            ticketSummaryTitle(ticket),
                            color = PortalInk,
                            fontWeight = FontWeight.SemiBold,
                            fontSize = 18.sp,
                            lineHeight = 25.sp,
                        )
                        Spacer(Modifier.height(4.dp))
                        Text(ticket.deviceNo.ifBlank { stringResource(R.string.common_no_device) }, color = PortalMuted, fontSize = 11.sp)
                    }
                    Spacer(Modifier.width(12.dp))
                    PortalStatusPill(ticket.status)
                }
                Spacer(Modifier.height(20.dp))
                HorizontalDivider(color = PortalBorder)
                PortalKeyValue(stringResource(R.string.ticket_product), ticket.productName)
                PortalKeyValue(stringResource(R.string.ticket_assignee), ticket.assigneeName.ifBlank { stringResource(R.string.ticket_assignee_pending) })
                PortalKeyValue(stringResource(R.string.ticket_priority), displayPriority(ticket.priority))
                PortalKeyValue(stringResource(R.string.ticket_updated_at), displayDate(ticket.updatedAt))
                Spacer(Modifier.height(20.dp))
                if (ticket.repairSummary.isNotBlank()) {
                    PortalSectionTitle(stringResource(R.string.ticket_repair_summary))
                    Text(
                        ticket.repairSummary,
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(Color(0xFFEEF2F6), RoundedCornerShape(7.dp))
                            .padding(13.dp),
                        color = Color(0xFF3F4A5D),
                        fontSize = 13.sp,
                        lineHeight = 20.sp,
                    )
                    Spacer(Modifier.height(20.dp))
                }
                PortalSectionTitle(stringResource(R.string.ticket_progress))
            }
            if (ticket.progress.isEmpty()) {
                item { Text(stringResource(R.string.ticket_no_progress), color = PortalMuted, fontSize = 12.sp, modifier = Modifier.padding(vertical = 12.dp)) }
            } else {
                items(ticket.progress, key = { it.id }) { progress ->
                    Row(modifier = Modifier.padding(bottom = 16.dp)) {
                        Box(
                            modifier = Modifier
                                .padding(top = 5.dp)
                                .size(8.dp)
                                .background(PortalBlue, RoundedCornerShape(4.dp)),
                        )
                        Spacer(Modifier.width(12.dp))
                        Column {
                            Text(progress.content.ifBlank { progress.eventType }, color = PortalInk, fontSize = 13.sp, lineHeight = 19.sp)
                            Text(displayDate(progress.createdAt), color = PortalMuted, fontSize = 10.sp)
                        }
                    }
                }
            }
            item {
                if (ticket.canConfirm) {
                    Button(
                        onClick = viewModel::confirmTicket,
                        enabled = !state.busy,
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(7.dp),
                    ) {
                        Icon(Icons.Outlined.CheckCircle, contentDescription = null, modifier = Modifier.size(18.dp))
                        Spacer(Modifier.width(7.dp))
                        Text(stringResource(R.string.ticket_confirm_done))
                    }
                    Spacer(Modifier.height(10.dp))
                }
                if (ticket.canReopen) {
                    OutlinedButton(
                        onClick = { reopenDialog = true },
                        enabled = !state.busy,
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(7.dp),
                    ) {
                        Icon(Icons.Outlined.ErrorOutline, contentDescription = null, modifier = Modifier.size(18.dp))
                        Spacer(Modifier.width(7.dp))
                        Text(stringResource(R.string.ticket_reopen))
                    }
                }
                if (ticket.canRate && ticket.feedbackRating <= 0) {
                    Spacer(Modifier.height(10.dp))
                    OutlinedButton(
                        onClick = { feedbackDialog = true },
                        enabled = !state.busy,
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(7.dp),
                    ) {
                        Icon(Icons.Outlined.Star, contentDescription = null, modifier = Modifier.size(18.dp))
                        Spacer(Modifier.width(7.dp))
                        Text(stringResource(R.string.ticket_rate_service))
                    }
                } else if (ticket.feedbackRating > 0) {
                    Spacer(Modifier.height(12.dp))
                    Text(
                        stringResource(R.string.ticket_rating_submitted, ticket.feedbackRating),
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(Color(0xFFFFF5D7), RoundedCornerShape(7.dp))
                            .padding(12.dp),
                        color = Color(0xFF775500),
                        fontSize = 12.sp,
                    )
                }
                Spacer(Modifier.height(24.dp))
            }
        }
    }

    if (reopenDialog) {
        AlertDialog(
            onDismissRequest = { if (!state.busy) reopenDialog = false },
            title = { Text(stringResource(R.string.ticket_reopen_dialog_title)) },
            text = {
                Column {
                    Text(stringResource(R.string.ticket_reopen_dialog_hint), color = PortalMuted, fontSize = 13.sp)
                    Spacer(Modifier.height(10.dp))
                    OutlinedTextField(
                        value = reopenReason,
                        onValueChange = { reopenReason = it.take(1000) },
                        modifier = Modifier.fillMaxWidth(),
                        minLines = 3,
                        maxLines = 6,
                        placeholder = { Text(stringResource(R.string.ticket_reopen_placeholder)) },
                    )
                }
            },
            confirmButton = {
                Button(
                    onClick = {
                        viewModel.reopenTicket(reopenReason)
                        reopenDialog = false
                    },
                    enabled = reopenReason.isNotBlank() && !state.busy,
                ) { Text(stringResource(R.string.ticket_submit)) }
            },
            dismissButton = { TextButton(onClick = { reopenDialog = false }) { Text(stringResource(R.string.common_cancel)) } },
        )
    }

    if (feedbackDialog) {
        AlertDialog(
            onDismissRequest = { if (!state.busy) feedbackDialog = false },
            title = { Text(stringResource(R.string.ticket_feedback_dialog_title)) },
            text = {
                Column {
                    Text(stringResource(R.string.ticket_feedback_dialog_hint), color = PortalMuted, fontSize = 13.sp)
                    Spacer(Modifier.height(8.dp))
                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center) {
                        (1..5).forEach { rating ->
                            androidx.compose.material3.IconButton(onClick = { feedbackRating = rating }) {
                                Icon(
                                    Icons.Outlined.Star,
                                    contentDescription = stringResource(R.string.ticket_star_rating, rating),
                                    tint = if (rating <= feedbackRating) Color(0xFFE0A000) else Color(0xFFC5CBD4),
                                )
                            }
                        }
                    }
                    OutlinedTextField(
                        value = feedbackComment,
                        onValueChange = { feedbackComment = it.take(1000) },
                        modifier = Modifier.fillMaxWidth(),
                        minLines = 3,
                        maxLines = 5,
                        label = { Text(stringResource(R.string.ticket_feedback_label)) },
                    )
                }
            },
            confirmButton = {
                Button(
                    onClick = {
                        viewModel.submitTicketFeedback(feedbackRating, feedbackComment)
                        feedbackDialog = false
                    },
                    enabled = !state.busy,
                ) { Text(stringResource(R.string.ticket_submit_rating)) }
            },
            dismissButton = { TextButton(onClick = { feedbackDialog = false }) { Text(stringResource(R.string.common_cancel)) } },
        )
    }
}

@Composable
internal fun NativeMeetingsScreen(
    state: NativePortalUiState,
    viewModel: NativeCustomerPortalViewModel,
) {
    state.selectedMeeting?.let {
        MeetingDetail(state, it, viewModel)
        return
    }

    var filter by rememberSaveable { mutableStateOf(MeetingFilter.UPCOMING) }
    val filtered = remember(state.meetings, filter) {
        state.meetings.filter { meeting ->
            val joinable = isJoinableMeeting(meeting.status)
            if (filter == MeetingFilter.UPCOMING) joinable else !joinable
        }
    }

    Column(Modifier.fillMaxSize()) {
        PortalMessageBanner(state, viewModel::reloadCurrent)
        Box(modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp)) {
            PortalSegmentedControl(
                options = listOf(stringResource(R.string.meeting_filter_upcoming), stringResource(R.string.meeting_filter_history)),
                selectedIndex = filter.ordinal,
                onSelected = { filter = MeetingFilter.entries[it] },
            )
        }
        HorizontalDivider(color = PortalBorder)
        if (filtered.isEmpty() && !state.loading && !state.meetingsHasMore) {
            PortalEmptyState(
                if (filter == MeetingFilter.UPCOMING) stringResource(R.string.meeting_empty_upcoming_title) else stringResource(R.string.meeting_empty_history_title),
                stringResource(R.string.meeting_empty_detail),
            )
        } else {
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = androidx.compose.foundation.layout.PaddingValues(12.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                items(filtered, key = { it.id }) { meeting ->
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(Color.White, RoundedCornerShape(7.dp))
                            .clickable { viewModel.selectMeeting(meeting.id) }
                            .padding(horizontal = 14.dp, vertical = 13.dp),
                    ) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                meetingSummaryTitle(meeting),
                                modifier = Modifier.weight(1f),
                                color = PortalInk,
                                fontSize = 14.sp,
                                fontWeight = FontWeight.SemiBold,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                            PortalStatusPill(meeting.status)
                        }
                        Spacer(Modifier.height(7.dp))
                        Text(
                            meetingReference(meeting),
                            color = Color(0xFF465266),
                            fontSize = 13.sp,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                        Spacer(Modifier.height(9.dp))
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                meeting.deviceNo.ifBlank { stringResource(R.string.common_no_device) },
                                modifier = Modifier.weight(1f),
                                color = PortalMuted,
                                fontSize = 10.sp,
                                maxLines = 1,
                            )
                            Text(displayDate(meeting.scheduledAt.ifBlank { meeting.startedAt }), color = PortalMuted, fontSize = 10.sp)
                            Spacer(Modifier.width(4.dp))
                            Icon(
                                Icons.AutoMirrored.Outlined.KeyboardArrowRight,
                                contentDescription = stringResource(R.string.meeting_open_detail),
                                tint = PortalMuted,
                                modifier = Modifier.size(17.dp),
                            )
                        }
                    }
                }
                if (state.meetingsHasMore) {
                    item(key = "load-more-meetings") {
                        PortalLoadMoreRow(NativePortalTab.MEETINGS, state, viewModel)
                    }
                }
            }
        }
    }
}

@Composable
private fun MeetingDetail(
    state: NativePortalUiState,
    meeting: NativeMeeting,
    viewModel: NativeCustomerPortalViewModel,
) {
    Column(Modifier.fillMaxSize()) {
        PortalMessageBanner(state, viewModel::reloadCurrent)
        Column(
            modifier = Modifier
                .fillMaxSize()
                .background(Color.White)
                .padding(18.dp),
        ) {
            Row(verticalAlignment = Alignment.Top) {
                Box(
                    modifier = Modifier
                        .size(44.dp)
                        .background(Color(0xFFEEF3FF), RoundedCornerShape(7.dp)),
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(Icons.Outlined.Videocam, contentDescription = null, tint = Color(0xFF3F60D7))
                }
                Spacer(Modifier.width(12.dp))
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        meetingSummaryTitle(meeting),
                        color = PortalInk,
                        fontWeight = FontWeight.SemiBold,
                        fontSize = 18.sp,
                        lineHeight = 25.sp,
                    )
                    Text(meeting.deviceNo.ifBlank { meeting.productName }, color = PortalMuted, fontSize = 11.sp)
                }
                PortalStatusPill(meeting.status)
            }
            Spacer(Modifier.height(22.dp))
            HorizontalDivider(color = PortalBorder)
            PortalKeyValue(stringResource(R.string.meeting_scheduled_at), displayDate(meeting.scheduledAt.ifBlank { meeting.startedAt }))
            PortalKeyValue(stringResource(R.string.meeting_participants), stringResource(R.string.meeting_participant_count, meeting.participantCount))
            PortalKeyValue(stringResource(R.string.meeting_duration), displayDuration(meeting.durationSeconds))
            PortalKeyValue(stringResource(R.string.meeting_transcripts), stringResource(R.string.meeting_count_unit, meeting.transcriptCount))
            PortalKeyValue(stringResource(R.string.meeting_annotations), stringResource(R.string.meeting_count_unit, meeting.annotationCount))
            Spacer(Modifier.height(20.dp))

            if (isJoinableMeeting(meeting.status)) {
                Button(
                    onClick = viewModel::prepareMeetingJoin,
                    enabled = !state.busy,
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(7.dp),
                    colors = ButtonDefaults.buttonColors(containerColor = PortalBlue),
                ) {
                    Icon(Icons.Outlined.Videocam, contentDescription = null, modifier = Modifier.size(18.dp))
                    Spacer(Modifier.width(7.dp))
                    Text(stringResource(R.string.meeting_join))
                }
            } else {
                Text(
                    stringResource(R.string.meeting_archived),
                    modifier = Modifier
                        .fillMaxWidth()
                        .background(PortalBackground, RoundedCornerShape(7.dp))
                        .padding(14.dp),
                    color = PortalMuted,
                    fontSize = 12.sp,
                )
            }

            if (state.busy) {
                Spacer(Modifier.height(12.dp))
                Text(stringResource(R.string.meeting_join_busy), color = PortalMuted, fontSize = 12.sp)
            }
        }
    }
}

@Composable
private fun PortalSegmentedControl(
    options: List<String>,
    selectedIndex: Int,
    onSelected: (Int) -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(Color(0xFFE3E7EC), RoundedCornerShape(7.dp))
            .padding(3.dp),
    ) {
        options.forEachIndexed { index, label ->
            Box(
                modifier = Modifier
                    .weight(1f)
                    .height(34.dp)
                    .background(
                        if (index == selectedIndex) Color.White else Color.Transparent,
                        RoundedCornerShape(5.dp),
                    )
                    .clickable { onSelected(index) },
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    label,
                    color = if (index == selectedIndex) PortalInk else PortalMuted,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Medium,
                )
            }
        }
    }
}

@Composable
internal fun PortalLoadMoreRow(
    tab: NativePortalTab,
    state: NativePortalUiState,
    viewModel: NativeCustomerPortalViewModel,
) {
    val loading = state.loadingMoreTab == tab
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp),
        contentAlignment = Alignment.Center,
    ) {
        TextButton(onClick = { viewModel.loadMore(tab) }, enabled = !loading) {
            Text(if (loading) stringResource(R.string.common_loading_more) else stringResource(R.string.common_load_more), fontSize = 12.sp)
        }
    }
}

private fun isOpenTicket(status: String): Boolean = status.lowercase() in setOf(
    "open", "pending", "assigned", "processing", "in_progress", "waiting", "reopened",
)

@Composable
private fun ticketSummaryTitle(ticket: NativeTicket): String {
    val title = ticket.title.trim()
    return if (title.isBlank() || title == "会话工单") {
        ticket.productName.ifBlank { ticket.deviceNo.ifBlank { stringResource(R.string.ticket_summary_default) } }
    } else {
        title
    }
}

@Composable
private fun meetingSummaryTitle(meeting: NativeMeeting): String {
    val title = meeting.title.trim()
    return if (title.isBlank() || title == "会话工单") {
        meeting.productName.ifBlank { stringResource(R.string.meeting_summary_default) }
    } else {
        title
    }
}

@Composable
private fun meetingReference(meeting: NativeMeeting): String {
    val reference = meeting.id.trim().takeLast(8)
    return if (meeting.ticketNo.isBlank()) {
        stringResource(R.string.meeting_reference_no_ticket, reference)
    } else {
        stringResource(R.string.meeting_reference_with_ticket, meeting.ticketNo, reference)
    }
}

private fun isJoinableMeeting(status: String): Boolean =
    status.lowercase() in setOf("active", "waiting", "scheduled")
