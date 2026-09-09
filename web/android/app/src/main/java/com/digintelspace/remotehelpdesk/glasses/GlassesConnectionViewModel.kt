package com.digintelspace.remotehelpdesk.glasses

import android.app.Application
import androidx.annotation.StringRes
import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import androidx.lifecycle.AndroidViewModel
import com.digintelspace.remotehelpdesk.R

class GlassesConnectionViewModel(application: Application) : AndroidViewModel(application),
    GlassesDiscoveryListener {
    private val capabilityProbe = GlassesCapabilityProbe(application)
    private val vendorRegistry = GlassesVendorRegistry()
    private val discovery = GlassesDiscoveryCoordinator(application, this)
    private val devicesByMode = mutableMapOf<GlassesTransportMode, List<NearbyGlassesDevice>>()
    private val mutableState = mutableStateOf(
        GlassesUiState(statusMessage = str(R.string.glasses_checking_capabilities)),
    )

    val state: State<GlassesUiState> = mutableState

    private fun str(@StringRes id: Int): String = getApplication<Application>().getString(id)

    init {
        refreshCapabilities()
    }

    fun refreshCapabilities() {
        val capabilities = capabilityProbe.current()
        mutableState.value = mutableState.value.copy(
            capabilities = capabilities,
            statusMessage = initialStatus(capabilities, mutableState.value.selectedMode),
        )
    }

    fun selectMode(mode: GlassesTransportMode) {
        if (mode == mutableState.value.selectedMode) return
        discovery.stop()
        mutableState.value = mutableState.value.copy(
            selectedMode = mode,
            phase = GlassesConnectionPhase.IDLE,
            devices = devicesByMode[mode].orEmpty(),
            selectedDeviceId = null,
            connectedEndpoint = null,
            statusMessage = initialStatus(mutableState.value.capabilities, mode),
        )
    }

    fun beginDiscovery() {
        val current = mutableState.value
        mutableState.value = current.copy(
            phase = GlassesConnectionPhase.DISCOVERING,
            devices = emptyList(),
            selectedDeviceId = null,
            connectedEndpoint = null,
            statusMessage = when (current.selectedMode) {
                GlassesTransportMode.BLUETOOTH -> str(R.string.glasses_bt_searching)
                GlassesTransportMode.WIFI_DIRECT -> str(R.string.glasses_wifi_direct_searching)
            },
        )
        devicesByMode[current.selectedMode] = emptyList()
        discovery.start(current.selectedMode)
    }

    fun permissionDenied() {
        refreshCapabilities()
        mutableState.value = mutableState.value.copy(
            phase = GlassesConnectionPhase.ERROR,
            statusMessage = when (mutableState.value.selectedMode) {
                GlassesTransportMode.BLUETOOTH -> str(R.string.glasses_bt_permission_required_search)
                GlassesTransportMode.WIFI_DIRECT -> str(R.string.glasses_wifi_direct_permission_required_search)
            },
        )
    }

    fun selectDevice(deviceId: String) {
        val device = mutableState.value.devices.firstOrNull { it.id == deviceId } ?: return
        val message = when {
            device.transport == GlassesTransportMode.WIFI_DIRECT ->
                str(R.string.glasses_device_selected_wifi, device.name)
            vendorRegistry.canConnect(device) ->
                str(R.string.glasses_device_recognized_vendor, device.name)
            else ->
                str(R.string.glasses_device_discovered_bt_limited, device.name)
        }
        mutableState.value = mutableState.value.copy(
            selectedDeviceId = deviceId,
            phase = GlassesConnectionPhase.IDLE,
            statusMessage = message,
        )
    }

    fun connectSelected() {
        val device = mutableState.value.selectedDevice ?: return
        if (device.transport == GlassesTransportMode.BLUETOOTH && !vendorRegistry.canConnect(device)) {
            mutableState.value = mutableState.value.copy(
                phase = GlassesConnectionPhase.ERROR,
                statusMessage = str(R.string.glasses_bt_vendor_not_configured),
            )
            return
        }
        mutableState.value = mutableState.value.copy(
            phase = GlassesConnectionPhase.CONNECTING,
            statusMessage = str(R.string.glasses_connecting_device, device.name),
        )
        discovery.connect(device)
    }

    override fun onDevicesChanged(
        mode: GlassesTransportMode,
        devices: List<NearbyGlassesDevice>,
    ) {
        devicesByMode[mode] = devices
        if (mutableState.value.selectedMode != mode) return
        val selectedId = mutableState.value.selectedDeviceId
            ?.takeIf { id -> devices.any { it.id == id } }
        mutableState.value = mutableState.value.copy(
            devices = devices,
            selectedDeviceId = selectedId,
        )
    }

    override fun onDiscoveryStateChanged(
        mode: GlassesTransportMode,
        discovering: Boolean,
        message: String,
        failed: Boolean,
    ) {
        if (mutableState.value.selectedMode != mode) return
        mutableState.value = mutableState.value.copy(
            phase = when {
                failed -> GlassesConnectionPhase.ERROR
                discovering -> GlassesConnectionPhase.DISCOVERING
                else -> GlassesConnectionPhase.IDLE
            },
            statusMessage = message,
        )
    }

    override fun onConnectionChanged(
        connected: Boolean,
        message: String,
        endpoint: String? = null,
        failed: Boolean = false,
    ) {
        mutableState.value = mutableState.value.copy(
            phase = when {
                connected -> GlassesConnectionPhase.CONNECTED
                failed -> GlassesConnectionPhase.ERROR
                else -> GlassesConnectionPhase.CONNECTING
            },
            statusMessage = message,
            connectedEndpoint = endpoint,
        )
    }

    override fun onCleared() {
        discovery.close()
        super.onCleared()
    }

    private fun initialStatus(
        capabilities: GlassesCapabilityState,
        mode: GlassesTransportMode,
    ): String = when (mode) {
        GlassesTransportMode.BLUETOOTH -> when {
            !capabilities.bluetoothSupported -> str(R.string.glasses_bt_phone_unsupported)
            !capabilities.bluetoothEnabled -> str(R.string.glasses_bt_enable_required)
            !capabilities.bluetoothPermissionGranted -> str(R.string.glasses_bt_permission_required)
            else -> str(R.string.glasses_bt_ready)
        }
        GlassesTransportMode.WIFI_DIRECT -> when {
            !capabilities.wifiDirectSupported -> str(R.string.glasses_wifi_direct_phone_unsupported)
            !capabilities.wifiEnabled -> str(R.string.glasses_wifi_enable_required)
            !capabilities.wifiPermissionGranted -> str(R.string.glasses_wifi_permission_required)
            else -> str(R.string.glasses_wifi_direct_ready)
        }
    }
}
