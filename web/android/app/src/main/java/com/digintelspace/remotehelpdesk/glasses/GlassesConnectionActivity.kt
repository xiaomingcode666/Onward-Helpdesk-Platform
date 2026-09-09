package com.digintelspace.remotehelpdesk.glasses

import android.Manifest
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.outlined.Bluetooth
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.Devices
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.Link
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material.icons.outlined.Wifi
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.Typography
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
import androidx.core.view.WindowCompat
import androidx.lifecycle.ViewModelProvider
import com.digintelspace.remotehelpdesk.R

class GlassesConnectionActivity : ComponentActivity() {
    private lateinit var viewModel: GlassesConnectionViewModel
    private var pendingDiscoveryMode = GlassesTransportMode.BLUETOOTH

    private val permissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions(),
    ) { result ->
        viewModel.refreshCapabilities()
        if (result.isNotEmpty() && result.values.all { it }) {
            viewModel.selectMode(pendingDiscoveryMode)
            viewModel.beginDiscovery()
        } else {
            viewModel.permissionDenied()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        WindowCompat.setDecorFitsSystemWindows(window, false)
        WindowCompat.getInsetsController(window, window.decorView).apply {
            isAppearanceLightStatusBars = true
            isAppearanceLightNavigationBars = true
        }

        viewModel = ViewModelProvider(this)[GlassesConnectionViewModel::class.java]
        setContent {
            val state by viewModel.state
            RemoteHelpDeskNativeTheme {
                GlassesWorkspace(
                    state = state,
                    onBack = ::finish,
                    onModeSelected = viewModel::selectMode,
                    onDiscover = ::requestDiscovery,
                    onDeviceSelected = viewModel::selectDevice,
                    onConnect = viewModel::connectSelected,
                )
            }
        }
    }

    override fun onResume() {
        super.onResume()
        if (::viewModel.isInitialized) viewModel.refreshCapabilities()
    }

    private fun requestDiscovery(mode: GlassesTransportMode) {
        viewModel.selectMode(mode)
        val missingPermissions = requiredPermissions(mode).filter {
            ContextCompat.checkSelfPermission(this, it) != android.content.pm.PackageManager.PERMISSION_GRANTED
        }
        if (missingPermissions.isEmpty()) {
            viewModel.beginDiscovery()
            return
        }
        pendingDiscoveryMode = mode
        permissionLauncher.launch(missingPermissions.toTypedArray())
    }

    private fun requiredPermissions(mode: GlassesTransportMode): List<String> = when (mode) {
        GlassesTransportMode.BLUETOOTH -> if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            listOf(Manifest.permission.BLUETOOTH_SCAN, Manifest.permission.BLUETOOTH_CONNECT)
        } else {
            listOf(Manifest.permission.ACCESS_FINE_LOCATION)
        }
        GlassesTransportMode.WIFI_DIRECT -> if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            listOf(Manifest.permission.NEARBY_WIFI_DEVICES)
        } else {
            listOf(Manifest.permission.ACCESS_FINE_LOCATION)
        }
    }
}

private val NativeBlue = Color(0xFF1769E0)
private val NativeInk = Color(0xFF172033)
private val NativeMuted = Color(0xFF657084)
private val NativeCanvas = Color(0xFFF4F6F8)
private val NativeBorder = Color(0xFFDCE2EA)
private val NativeGreen = Color(0xFF087F5B)
private val NativeWarning = Color(0xFFB55B08)
private val NativeDanger = Color(0xFFB83232)

@Composable
private fun RemoteHelpDeskNativeTheme(content: @Composable () -> Unit) {
    val typography = Typography(
        titleLarge = TextStyle(fontSize = 20.sp, lineHeight = 28.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 0.sp),
        titleMedium = TextStyle(fontSize = 16.sp, lineHeight = 22.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 0.sp),
        bodyLarge = TextStyle(fontSize = 15.sp, lineHeight = 22.sp, letterSpacing = 0.sp),
        bodyMedium = TextStyle(fontSize = 13.sp, lineHeight = 20.sp, letterSpacing = 0.sp),
        bodySmall = TextStyle(fontSize = 11.sp, lineHeight = 16.sp, letterSpacing = 0.sp),
        labelLarge = TextStyle(fontSize = 14.sp, lineHeight = 20.sp, fontWeight = FontWeight.Medium, letterSpacing = 0.sp),
        labelMedium = TextStyle(fontSize = 12.sp, lineHeight = 18.sp, fontWeight = FontWeight.Medium, letterSpacing = 0.sp),
    )
    MaterialTheme(
        colorScheme = androidx.compose.material3.lightColorScheme(
            primary = NativeBlue,
            onPrimary = Color.White,
            background = NativeCanvas,
            onBackground = NativeInk,
            surface = Color.White,
            onSurface = NativeInk,
            outline = NativeBorder,
            error = NativeDanger,
        ),
        typography = typography,
        content = content,
    )
}

@Composable
private fun GlassesWorkspace(
    state: GlassesUiState,
    onBack: () -> Unit,
    onModeSelected: (GlassesTransportMode) -> Unit,
    onDiscover: (GlassesTransportMode) -> Unit,
    onDeviceSelected: (String) -> Unit,
    onConnect: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(NativeCanvas)
            .navigationBarsPadding(),
    ) {
        NativeTopBar(onBack)
        CapabilitySection(state.capabilities)
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 14.dp),
        ) {
            Text(stringResource(R.string.glasses_connection_method), style = MaterialTheme.typography.labelMedium, color = NativeMuted)
            Spacer(Modifier.height(8.dp))
            TransportSelector(state.selectedMode, onModeSelected)
            Spacer(Modifier.height(12.dp))
            DiscoveryStatus(state)
            Spacer(Modifier.height(12.dp))
            OutlinedButton(
                onClick = { onDiscover(state.selectedMode) },
                enabled = state.phase != GlassesConnectionPhase.DISCOVERING,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(46.dp),
                shape = RoundedCornerShape(6.dp),
                colors = ButtonDefaults.outlinedButtonColors(contentColor = NativeBlue),
            ) {
                if (state.phase == GlassesConnectionPhase.DISCOVERING) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(17.dp),
                        strokeWidth = 2.dp,
                        color = NativeBlue,
                    )
                } else {
                    Icon(Icons.Outlined.Refresh, contentDescription = null, modifier = Modifier.size(18.dp))
                }
                Spacer(Modifier.width(8.dp))
                Text(
                    if (state.phase == GlassesConnectionPhase.DISCOVERING) {
                        stringResource(R.string.glasses_searching)
                    } else {
                        stringResource(R.string.glasses_search_nearby_devices)
                    },
                )
            }
        }
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(stringResource(R.string.glasses_nearby_devices_title), style = MaterialTheme.typography.titleMedium, color = NativeInk)
            Spacer(Modifier.weight(1f))
            Text(stringResource(R.string.glasses_device_count, state.devices.size), style = MaterialTheme.typography.bodySmall, color = NativeMuted)
        }
        DeviceList(
            state = state,
            onDeviceSelected = onDeviceSelected,
            modifier = Modifier.weight(1f),
        )
        SelectedDeviceAction(state, onConnect)
    }
}

@Composable
private fun NativeTopBar(onBack: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(Color.White)
            .statusBarsPadding()
            .height(58.dp)
            .padding(horizontal = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        IconButton(onClick = onBack, modifier = Modifier.size(44.dp)) {
            Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = stringResource(R.string.glasses_back), tint = NativeInk)
        }
        Column(modifier = Modifier.padding(start = 4.dp)) {
            Text(stringResource(R.string.glasses_connection_title), style = MaterialTheme.typography.titleLarge, color = NativeInk)
            Text(stringResource(R.string.glasses_workbench_subtitle), style = MaterialTheme.typography.bodySmall, color = NativeMuted)
        }
    }
    HorizontalDivider(color = NativeBorder)
}

@Composable
private fun CapabilitySection(capabilities: GlassesCapabilityState) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .background(Color.White)
            .padding(horizontal = 16.dp, vertical = 10.dp),
    ) {
        CapabilityRow(
            icon = { Icon(Icons.Outlined.Bluetooth, contentDescription = null) },
            label = stringResource(R.string.glasses_bluetooth),
            value = when {
                !capabilities.bluetoothSupported -> stringResource(R.string.glasses_capability_unsupported)
                !capabilities.bluetoothEnabled -> stringResource(R.string.glasses_capability_disabled)
                !capabilities.bluetoothPermissionGranted -> stringResource(R.string.glasses_capability_pending_auth)
                else -> stringResource(R.string.glasses_capability_ready)
            },
            ready = capabilities.bluetoothSupported && capabilities.bluetoothEnabled && capabilities.bluetoothPermissionGranted,
        )
        HorizontalDivider(color = Color(0xFFEDF0F4))
        CapabilityRow(
            icon = { Icon(Icons.Outlined.Wifi, contentDescription = null) },
            label = "Wi-Fi",
            value = buildString {
                append(
                    if (capabilities.wifiEnabled) stringResource(R.string.glasses_capability_enabled)
                    else stringResource(R.string.glasses_capability_disabled),
                )
                if (capabilities.wifiDirectSupported) append(" · Direct")
                if (capabilities.wifiAwareSupported) append(" · Aware")
            },
            ready = capabilities.wifiEnabled && capabilities.wifiDirectSupported && capabilities.wifiPermissionGranted,
        )
    }
}

@Composable
private fun CapabilityRow(
    icon: @Composable () -> Unit,
    label: String,
    value: String,
    ready: Boolean,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(42.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(modifier = Modifier.size(20.dp), contentAlignment = Alignment.Center) {
            androidx.compose.material3.ProvideTextStyle(
                value = MaterialTheme.typography.bodyMedium.copy(color = if (ready) NativeGreen else NativeMuted),
                content = icon,
            )
        }
        Spacer(Modifier.width(10.dp))
        Text(label, style = MaterialTheme.typography.bodyMedium, color = NativeInk)
        Spacer(Modifier.weight(1f))
        Text(value, style = MaterialTheme.typography.bodySmall, color = if (ready) NativeGreen else NativeWarning)
    }
}

@Composable
private fun TransportSelector(
    selected: GlassesTransportMode,
    onSelected: (GlassesTransportMode) -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(44.dp)
            .border(1.dp, NativeBorder, RoundedCornerShape(7.dp))
            .padding(3.dp),
    ) {
        TransportOption(
            label = stringResource(R.string.glasses_bluetooth),
            icon = { Icon(Icons.Outlined.Bluetooth, contentDescription = null, modifier = Modifier.size(17.dp)) },
            selected = selected == GlassesTransportMode.BLUETOOTH,
            onClick = { onSelected(GlassesTransportMode.BLUETOOTH) },
            modifier = Modifier.weight(1f),
        )
        Spacer(Modifier.width(4.dp))
        TransportOption(
            label = stringResource(R.string.glasses_transport_wifi_direct),
            icon = { Icon(Icons.Outlined.Wifi, contentDescription = null, modifier = Modifier.size(17.dp)) },
            selected = selected == GlassesTransportMode.WIFI_DIRECT,
            onClick = { onSelected(GlassesTransportMode.WIFI_DIRECT) },
            modifier = Modifier.weight(1f),
        )
    }
}

@Composable
private fun TransportOption(
    label: String,
    icon: @Composable () -> Unit,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(
        modifier = modifier
            .fillMaxSize()
            .clickable(onClick = onClick),
        color = if (selected) NativeBlue else Color.Transparent,
        contentColor = if (selected) Color.White else NativeMuted,
        shape = RoundedCornerShape(5.dp),
    ) {
        Row(
            horizontalArrangement = Arrangement.Center,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            icon()
            Spacer(Modifier.width(7.dp))
            Text(label, style = MaterialTheme.typography.labelLarge)
        }
    }
}

@Composable
private fun DiscoveryStatus(state: GlassesUiState) {
    val tone = when (state.phase) {
        GlassesConnectionPhase.CONNECTED -> NativeGreen
        GlassesConnectionPhase.ERROR -> NativeDanger
        GlassesConnectionPhase.CONNECTING,
        GlassesConnectionPhase.DISCOVERING -> NativeBlue
        GlassesConnectionPhase.IDLE -> NativeMuted
    }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(Color.White, RoundedCornerShape(7.dp))
            .border(1.dp, NativeBorder, RoundedCornerShape(7.dp))
            .padding(horizontal = 12.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            imageVector = when (state.phase) {
                GlassesConnectionPhase.CONNECTED -> Icons.Outlined.CheckCircle
                GlassesConnectionPhase.ERROR -> Icons.Outlined.ErrorOutline
                else -> Icons.Outlined.Devices
            },
            contentDescription = null,
            tint = tone,
            modifier = Modifier.size(18.dp),
        )
        Spacer(Modifier.width(9.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(state.statusMessage, style = MaterialTheme.typography.bodyMedium, color = NativeInk)
            state.connectedEndpoint?.let {
                Text(stringResource(R.string.glasses_endpoint_label, it), style = MaterialTheme.typography.bodySmall, color = NativeMuted)
            }
        }
    }
}

@Composable
private fun DeviceList(
    state: GlassesUiState,
    onDeviceSelected: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    if (state.devices.isEmpty()) {
        Column(
            modifier = modifier
                .fillMaxWidth()
                .padding(horizontal = 32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center,
        ) {
            Icon(Icons.Outlined.Devices, contentDescription = null, tint = Color(0xFFA8B0BD), modifier = Modifier.size(34.dp))
            Spacer(Modifier.height(10.dp))
            Text(
                if (state.phase == GlassesConnectionPhase.DISCOVERING) stringResource(R.string.glasses_waiting_for_devices)
                else stringResource(R.string.glasses_no_devices_found),
                style = MaterialTheme.typography.bodyMedium,
                color = NativeMuted,
            )
        }
        return
    }

    LazyColumn(
        modifier = modifier.fillMaxWidth(),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 12.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        items(state.devices, key = NearbyGlassesDevice::id) { device ->
            DeviceRow(
                device = device,
                selected = device.id == state.selectedDeviceId,
                onClick = { onDeviceSelected(device.id) },
            )
        }
    }
}

@Composable
private fun DeviceRow(
    device: NearbyGlassesDevice,
    selected: Boolean,
    onClick: () -> Unit,
) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .border(
                width = if (selected) 1.5.dp else 1.dp,
                color = if (selected) NativeBlue else NativeBorder,
                shape = RoundedCornerShape(7.dp),
            ),
        color = Color.White,
        shape = RoundedCornerShape(7.dp),
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 13.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                modifier = Modifier
                    .size(38.dp)
                    .background(if (selected) Color(0xFFEAF2FF) else NativeCanvas, RoundedCornerShape(7.dp)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    if (device.transport == GlassesTransportMode.BLUETOOTH) Icons.Outlined.Bluetooth else Icons.Outlined.Wifi,
                    contentDescription = null,
                    tint = if (selected) NativeBlue else NativeMuted,
                    modifier = Modifier.size(20.dp),
                )
            }
            Spacer(Modifier.width(12.dp))
            Column(modifier = Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        device.name,
                        style = MaterialTheme.typography.bodyLarge.copy(fontWeight = FontWeight.Medium),
                        color = NativeInk,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f, fill = false),
                    )
                    if (device.paired) {
                        Spacer(Modifier.width(6.dp))
                        Text(stringResource(R.string.glasses_paired), style = MaterialTheme.typography.bodySmall, color = NativeGreen)
                    }
                }
                Spacer(Modifier.height(2.dp))
                Text(device.address, style = MaterialTheme.typography.bodySmall, color = NativeMuted)
            }
            device.signalStrength?.let {
                Text(stringResource(R.string.glasses_signal_strength_dbm, it), style = MaterialTheme.typography.bodySmall, color = NativeMuted)
            }
            if (selected) {
                Spacer(Modifier.width(8.dp))
                Icon(Icons.Outlined.CheckCircle, contentDescription = stringResource(R.string.glasses_selected), tint = NativeBlue, modifier = Modifier.size(19.dp))
            }
        }
    }
}

@Composable
private fun SelectedDeviceAction(state: GlassesUiState, onConnect: () -> Unit) {
    val selected = state.selectedDevice ?: return
    val canRequestConnection = selected.transport == GlassesTransportMode.WIFI_DIRECT
    Surface(color = Color.White, shadowElevation = 4.dp) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 12.dp),
        ) {
            Button(
                onClick = onConnect,
                enabled = canRequestConnection && state.phase != GlassesConnectionPhase.CONNECTING,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(48.dp),
                shape = RoundedCornerShape(6.dp),
                colors = ButtonDefaults.buttonColors(containerColor = NativeBlue),
            ) {
                if (state.phase == GlassesConnectionPhase.CONNECTING) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(17.dp),
                        strokeWidth = 2.dp,
                        color = Color.White,
                    )
                } else {
                    Icon(Icons.Outlined.Link, contentDescription = null, modifier = Modifier.size(18.dp))
                }
                Spacer(Modifier.width(8.dp))
                Text(
                    if (canRequestConnection) stringResource(R.string.glasses_connect_device, selected.name)
                    else stringResource(R.string.glasses_waiting_vendor_protocol),
                )
            }
        }
    }
}
