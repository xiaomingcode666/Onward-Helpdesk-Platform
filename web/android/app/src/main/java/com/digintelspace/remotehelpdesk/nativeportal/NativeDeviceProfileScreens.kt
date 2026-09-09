package com.digintelspace.remotehelpdesk.nativeportal

import android.content.Intent
import android.net.Uri
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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.Assignment
import androidx.compose.material.icons.automirrored.outlined.Logout
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.outlined.ChatBubbleOutline
import androidx.compose.material.icons.outlined.Close
import androidx.compose.material.icons.outlined.DeleteOutline
import androidx.compose.material.icons.outlined.Devices
import androidx.compose.material.icons.outlined.Description
import androidx.compose.material.icons.automirrored.outlined.KeyboardArrowRight
import androidx.compose.material.icons.outlined.Save
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Videocam
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.digintelspace.remotehelpdesk.R

@Composable
internal fun NativeDevicesScreen(
    state: NativePortalUiState,
    viewModel: NativeCustomerPortalViewModel,
) {
    state.selectedDevice?.let {
        DeviceDetail(state, it, viewModel)
        return
    }

    var query by rememberSaveable { mutableStateOf("") }
    var bindDialog by rememberSaveable { mutableStateOf(false) }
    var serviceCode by rememberSaveable { mutableStateOf("") }
    LaunchedEffect(state.pendingServiceCode) {
        if (state.pendingServiceCode.isNotBlank()) {
            serviceCode = state.pendingServiceCode
            bindDialog = true
            viewModel.consumePendingServiceCode()
        }
    }
    val normalizedQuery = query.trim().lowercase()
    val filtered = remember(state.devices, normalizedQuery) {
        state.devices.filter { item ->
            normalizedQuery.isBlank() || listOf(
                item.deviceNo,
                item.serialNo,
                item.productName,
                item.modelName,
            ).joinToString(" ").lowercase().contains(normalizedQuery)
        }
    }

    Column(Modifier.fillMaxSize()) {
        PortalMessageBanner(state, viewModel::reloadCurrent)
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 14.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            OutlinedTextField(
                value = query,
                onValueChange = { query = it },
                modifier = Modifier.weight(1f),
                singleLine = true,
                leadingIcon = { Icon(Icons.Outlined.Search, contentDescription = null, modifier = Modifier.size(19.dp)) },
                trailingIcon = {
                    if (query.isNotBlank()) {
                        IconButton(onClick = { query = "" }) {
                            Icon(Icons.Outlined.Close, contentDescription = stringResource(R.string.common_clear_search), modifier = Modifier.size(17.dp))
                        }
                    }
                },
                placeholder = { Text(stringResource(R.string.device_search_placeholder), fontSize = 13.sp) },
                shape = RoundedCornerShape(7.dp),
            )
            Spacer(Modifier.width(8.dp))
            IconButton(
                onClick = { bindDialog = true },
                modifier = Modifier
                    .size(50.dp)
                    .background(PortalBlue, RoundedCornerShape(7.dp)),
            ) {
                Icon(Icons.Outlined.Add, contentDescription = stringResource(R.string.device_bind), tint = Color.White)
            }
        }
        HorizontalDivider(color = PortalBorder)

        if (filtered.isEmpty() && !state.loading && !state.devicesHasMore) {
            PortalEmptyState(stringResource(R.string.device_empty_title), stringResource(R.string.device_empty_detail))
        } else {
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = androidx.compose.foundation.layout.PaddingValues(12.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                items(filtered, key = { it.id }) { device ->
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(Color.White, RoundedCornerShape(7.dp))
                            .clickable { viewModel.selectDevice(device.id) }
                            .padding(horizontal = 14.dp, vertical = 13.dp),
                    ) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                device.productName.ifBlank { stringResource(R.string.device_default_name) },
                                modifier = Modifier.weight(1f),
                                color = PortalInk,
                                fontSize = 14.sp,
                                fontWeight = FontWeight.SemiBold,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                            PortalStatusPill(device.status)
                        }
                        Spacer(Modifier.height(7.dp))
                        Text(
                            device.deviceNo.ifBlank { stringResource(R.string.device_no_confirm) },
                            color = Color(0xFF465266),
                            fontSize = 13.sp,
                            fontWeight = FontWeight.Medium,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                        Spacer(Modifier.height(9.dp))
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                device.modelName.ifBlank { device.serialNo.ifBlank { stringResource(R.string.device_default_detail) } },
                                modifier = Modifier.weight(1f),
                                color = PortalMuted,
                                fontSize = 10.sp,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                            Icon(
                                Icons.AutoMirrored.Outlined.KeyboardArrowRight,
                                contentDescription = stringResource(R.string.device_open_detail),
                                tint = PortalMuted,
                                modifier = Modifier.size(17.dp),
                            )
                        }
                    }
                }
                if (state.devicesHasMore) {
                    item(key = "load-more-devices") {
                        PortalLoadMoreRow(NativePortalTab.DEVICES, state, viewModel)
                    }
                }
            }
        }
    }

    if (bindDialog) {
        AlertDialog(
            onDismissRequest = { if (!state.busy) bindDialog = false },
            title = { Text(stringResource(R.string.device_bind)) },
            text = {
                Column {
                    Text(stringResource(R.string.device_bind_hint), color = PortalMuted, fontSize = 13.sp)
                    Spacer(Modifier.height(10.dp))
                    OutlinedTextField(
                        value = serviceCode,
                        onValueChange = { serviceCode = it.uppercase().take(80) },
                        modifier = Modifier.fillMaxWidth(),
                        singleLine = true,
                        label = { Text(stringResource(R.string.device_service_code_label)) },
                        placeholder = { Text(stringResource(R.string.device_service_code_placeholder)) },
                    )
                }
            },
            confirmButton = {
                Button(
                    onClick = {
                        viewModel.bindDevice(serviceCode)
                        bindDialog = false
                        serviceCode = ""
                    },
                    enabled = serviceCode.isNotBlank() && !state.busy,
                ) { Text(stringResource(R.string.device_bind_confirm)) }
            },
            dismissButton = { TextButton(onClick = { bindDialog = false }) { Text(stringResource(R.string.common_cancel)) } },
        )
    }
}

@Composable
private fun DeviceDetail(
    state: NativePortalUiState,
    device: NativeDevice,
    viewModel: NativeCustomerPortalViewModel,
) {
    val context = LocalContext.current
    Column(Modifier.fillMaxSize()) {
        PortalMessageBanner(state, viewModel::reloadCurrent)
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(18.dp),
        ) {
            item {
                Row(verticalAlignment = Alignment.Top) {
                    Box(
                        modifier = Modifier
                            .size(44.dp)
                            .background(Color(0xFFE8F7F2), RoundedCornerShape(7.dp)),
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(Icons.Outlined.Build, contentDescription = null, tint = Color(0xFF0F8B6D))
                    }
                    Spacer(Modifier.width(12.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            device.productName.ifBlank { stringResource(R.string.device_default_name) },
                            color = PortalInk,
                            fontWeight = FontWeight.SemiBold,
                            fontSize = 18.sp,
                            lineHeight = 25.sp,
                        )
                        Text(device.deviceNo, color = PortalMuted, fontSize = 11.sp)
                    }
                    PortalStatusPill(device.status)
                }
                Spacer(Modifier.height(20.dp))
                HorizontalDivider(color = PortalBorder)
                PortalKeyValue(stringResource(R.string.device_model), device.modelName)
                PortalKeyValue(stringResource(R.string.device_serial), device.serialNo)
                PortalKeyValue(stringResource(R.string.device_product_code), device.productCode)
                PortalKeyValue(stringResource(R.string.device_region), device.regionCode)
                PortalKeyValue(stringResource(R.string.device_warranty_until), displayDate(device.warrantyEndAt))
                if (device.modelName.isBlank() && device.serialNo.isBlank() && device.regionCode.isBlank() && device.warrantyEndAt.isBlank()) {
                    Text(
                        stringResource(R.string.device_profile_incomplete),
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(Color(0xFFFFF5D7), RoundedCornerShape(7.dp))
                            .padding(12.dp),
                        color = Color(0xFF775500),
                        fontSize = 12.sp,
                        lineHeight = 18.sp,
                    )
                }
                Spacer(Modifier.height(18.dp))
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .background(PortalBackground, RoundedCornerShape(7.dp))
                        .padding(vertical = 14.dp),
                ) {
                    DeviceMetric(stringResource(R.string.device_metric_open_tickets), device.openTicketCount, Modifier.weight(1f))
                    DeviceMetric(stringResource(R.string.device_metric_conversations), device.conversationCount, Modifier.weight(1f))
                    DeviceMetric(stringResource(R.string.device_metric_repairs), device.repairHistoryCount, Modifier.weight(1f))
                }
                Spacer(Modifier.height(16.dp))
                Button(
                    onClick = viewModel::startDeviceConversation,
                    enabled = !state.busy,
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(7.dp),
                ) {
                    Icon(Icons.Outlined.ChatBubbleOutline, contentDescription = null, modifier = Modifier.size(18.dp))
                    Spacer(Modifier.width(7.dp))
                    Text(stringResource(R.string.device_consult))
                }
                Spacer(Modifier.height(18.dp))
                PortalSectionTitle(stringResource(R.string.device_resources))
                if (state.deviceManuals.isEmpty()) {
                    Text(
                        if (device.manualCount > 0) {
                            stringResource(R.string.device_manuals_count_hint, device.manualCount)
                        } else {
                            stringResource(R.string.device_no_manuals)
                        },
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(Color(0xFFEEF2F6), RoundedCornerShape(7.dp))
                            .padding(13.dp),
                        color = PortalMuted,
                        fontSize = 12.sp,
                    )
                } else {
                    state.deviceManuals.forEach { manual ->
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable(enabled = manual.url.startsWith("https://") || manual.url.startsWith("http://")) {
                                    runCatching {
                                        context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(manual.url)))
                                    }
                                }
                                .padding(vertical = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Icon(Icons.Outlined.Description, contentDescription = null, tint = PortalBlue, modifier = Modifier.size(19.dp))
                            Spacer(Modifier.width(10.dp))
                            Column(modifier = Modifier.weight(1f)) {
                                Text(
                                    manual.title.ifBlank { manual.filename.ifBlank { stringResource(R.string.device_manual_fallback) } },
                                    color = PortalInk,
                                    fontSize = 13.sp,
                                    fontWeight = FontWeight.Medium,
                                    maxLines = 2,
                                    overflow = TextOverflow.Ellipsis,
                                )
                                Text(manual.mimeType.ifBlank { stringResource(R.string.device_document_fallback) }, color = PortalMuted, fontSize = 10.sp)
                            }
                        }
                        HorizontalDivider(color = PortalBorder)
                    }
                }
                Spacer(Modifier.height(24.dp))
            }
        }
    }
}

@Composable
private fun DeviceMetric(label: String, value: Int, modifier: Modifier = Modifier) {
    Column(modifier = modifier, horizontalAlignment = Alignment.CenterHorizontally) {
        Text(value.toString(), color = PortalInk, fontWeight = FontWeight.SemiBold, fontSize = 17.sp)
        Text(label, color = PortalMuted, fontSize = 9.sp)
    }
}

@Composable
internal fun NativeProfileScreen(
    state: NativePortalUiState,
    displayName: String,
    viewModel: NativeCustomerPortalViewModel,
    onLogout: () -> Unit,
    onOpenGlasses: () -> Unit,
) {
    val profile = state.profile
    if (profile == null) {
        Column(Modifier.fillMaxSize()) {
            PortalMessageBanner(state, viewModel::reloadCurrent)
            if (!state.loading) PortalEmptyState(stringResource(R.string.profile_load_failed_title), stringResource(R.string.profile_load_failed_detail))
        }
        return
    }

    var name by rememberSaveable(profile.id, profile.name) { mutableStateOf(profile.name) }
    var email by rememberSaveable(profile.id, profile.email) { mutableStateOf(profile.email) }
    var mobile by rememberSaveable(profile.id, profile.mobile) { mutableStateOf(profile.mobile) }
    var deleteDialog by rememberSaveable { mutableStateOf(false) }
    var password by rememberSaveable { mutableStateOf("") }
    var confirmation by rememberSaveable { mutableStateOf("") }
    var acknowledged by rememberSaveable { mutableStateOf(false) }

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(bottom = 28.dp),
    ) {
        item {
            PortalMessageBanner(state, viewModel::reloadCurrent)
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(PortalInk)
                    .padding(horizontal = 18.dp, vertical = 20.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(
                    modifier = Modifier
                        .size(48.dp)
                        .background(Color.White.copy(alpha = 0.12f), CircleShape),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        profile.name.ifBlank { displayName }.trim().take(1).uppercase().ifBlank { stringResource(R.string.profile_avatar_fallback) },
                        color = Color.White,
                        fontWeight = FontWeight.SemiBold,
                        fontSize = 19.sp,
                    )
                }
                Spacer(Modifier.width(12.dp))
                Column(modifier = Modifier.weight(1f)) {
                    Text(profile.name.ifBlank { displayName }, color = Color.White, fontWeight = FontWeight.SemiBold, fontSize = 18.sp)
                    Text(profile.companyName.ifBlank { stringResource(R.string.profile_customer_account) }, color = Color.White.copy(alpha = 0.62f), fontSize = 11.sp)
                }
            }

            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(Color.White)
                    .padding(vertical = 15.dp),
            ) {
                ProfileMetric(Icons.Outlined.Build, stringResource(R.string.profile_metric_devices), profile.boundDeviceCount, Modifier.weight(1f))
                ProfileMetric(Icons.Outlined.ChatBubbleOutline, stringResource(R.string.profile_metric_conversations), profile.activeConversationCount, Modifier.weight(1f))
                ProfileMetric(Icons.AutoMirrored.Outlined.Assignment, stringResource(R.string.profile_metric_tickets), profile.openTicketCount, Modifier.weight(1f))
                ProfileMetric(Icons.Outlined.Videocam, stringResource(R.string.profile_metric_meetings), profile.upcomingMeetingCount, Modifier.weight(1f))
            }

            Spacer(Modifier.height(12.dp))
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(Color.White)
                    .padding(16.dp),
            ) {
                PortalSectionTitle(stringResource(R.string.profile_basic_info))
                OutlinedTextField(
                    value = name,
                    onValueChange = { name = it.take(80) },
                    modifier = Modifier.fillMaxWidth(),
                    label = { Text(stringResource(R.string.profile_name)) },
                    singleLine = true,
                )
                Spacer(Modifier.height(10.dp))
                OutlinedTextField(
                    value = email,
                    onValueChange = { email = it.take(160) },
                    modifier = Modifier.fillMaxWidth(),
                    label = { Text(stringResource(R.string.profile_email)) },
                    singleLine = true,
                )
                Spacer(Modifier.height(10.dp))
                OutlinedTextField(
                    value = mobile,
                    onValueChange = { mobile = it.take(40) },
                    modifier = Modifier.fillMaxWidth(),
                    label = { Text(stringResource(R.string.profile_mobile)) },
                    singleLine = true,
                )
                Spacer(Modifier.height(12.dp))
                Button(
                    onClick = { viewModel.updateProfile(name, email, mobile) },
                    enabled = name.isNotBlank() && !state.busy,
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(7.dp),
                ) {
                    Icon(Icons.Outlined.Save, contentDescription = null, modifier = Modifier.size(18.dp))
                    Spacer(Modifier.width(7.dp))
                    Text(stringResource(R.string.profile_save))
                }
            }

            Spacer(Modifier.height(12.dp))
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(Color.White)
                    .padding(16.dp),
            ) {
                PortalSectionTitle(stringResource(R.string.profile_glasses_section))
                OutlinedButton(
                    onClick = onOpenGlasses,
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(7.dp),
                ) {
                    Icon(Icons.Outlined.Devices, contentDescription = null, modifier = Modifier.size(18.dp))
                    Spacer(Modifier.width(7.dp))
                    Text(stringResource(R.string.profile_connect_glasses))
                }
            }

            Spacer(Modifier.height(12.dp))
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(Color.White)
                    .padding(16.dp),
            ) {
                OutlinedButton(
                    onClick = onLogout,
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(7.dp),
                ) {
                    Icon(Icons.AutoMirrored.Outlined.Logout, contentDescription = null, modifier = Modifier.size(18.dp))
                    Spacer(Modifier.width(7.dp))
                    Text(stringResource(R.string.profile_logout))
                }
                Spacer(Modifier.height(10.dp))
                TextButton(
                    onClick = { deleteDialog = true },
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    Icon(Icons.Outlined.DeleteOutline, contentDescription = null, tint = PortalDanger, modifier = Modifier.size(18.dp))
                    Spacer(Modifier.width(7.dp))
                    Text(stringResource(R.string.profile_delete_account), color = PortalDanger)
                }
            }
        }
    }

    if (deleteDialog) {
        AlertDialog(
            onDismissRequest = { if (!state.busy) deleteDialog = false },
            title = { Text(stringResource(R.string.profile_delete_dialog_title)) },
            text = {
                Column {
                    Text(
                        stringResource(R.string.profile_delete_dialog_hint),
                        color = PortalMuted,
                        fontSize = 12.sp,
                        lineHeight = 18.sp,
                    )
                    Spacer(Modifier.height(12.dp))
                    OutlinedTextField(
                        value = password,
                        onValueChange = { password = it },
                        modifier = Modifier.fillMaxWidth(),
                        label = { Text(stringResource(R.string.profile_current_password)) },
                        visualTransformation = PasswordVisualTransformation(),
                        singleLine = true,
                    )
                    Spacer(Modifier.height(10.dp))
                    OutlinedTextField(
                        value = confirmation,
                        onValueChange = { confirmation = it.uppercase().take(6) },
                        modifier = Modifier.fillMaxWidth(),
                        label = { Text(stringResource(R.string.profile_delete_confirm_label)) },
                        singleLine = true,
                    )
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clickable { acknowledged = !acknowledged }
                            .padding(top = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Checkbox(checked = acknowledged, onCheckedChange = { acknowledged = it })
                        Text(stringResource(R.string.profile_delete_acknowledged), color = PortalInk, fontSize = 12.sp)
                    }
                }
            },
            confirmButton = {
                Button(
                    onClick = {
                        viewModel.deleteAccount(password)
                        deleteDialog = false
                    },
                    enabled = password.isNotBlank() && confirmation == "DELETE" && acknowledged && !state.busy,
                    colors = ButtonDefaults.buttonColors(containerColor = PortalDanger),
                ) { Text(stringResource(R.string.profile_delete_confirm)) }
            },
            dismissButton = { TextButton(onClick = { deleteDialog = false }) { Text(stringResource(R.string.common_cancel)) } },
        )
    }
}

@Composable
private fun ProfileMetric(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    label: String,
    value: Int,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier, horizontalAlignment = Alignment.CenterHorizontally) {
        Icon(icon, contentDescription = null, tint = Color(0xFF8B96A8), modifier = Modifier.size(17.dp))
        Spacer(Modifier.height(5.dp))
        Text(value.toString(), color = PortalInk, fontWeight = FontWeight.SemiBold, fontSize = 17.sp)
        Text(label, color = PortalMuted, fontSize = 9.sp)
    }
}
