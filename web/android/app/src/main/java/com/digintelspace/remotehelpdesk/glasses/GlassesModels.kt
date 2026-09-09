package com.digintelspace.remotehelpdesk.glasses

enum class GlassesTransportMode {
    BLUETOOTH,
    WIFI_DIRECT,
}

data class NearbyGlassesDevice(
    val id: String,
    val name: String,
    val address: String,
    val transport: GlassesTransportMode,
    val signalStrength: Int? = null,
    val paired: Boolean = false,
)

data class GlassesCapabilityState(
    val bluetoothSupported: Boolean = false,
    val bluetoothEnabled: Boolean = false,
    val bluetoothPermissionGranted: Boolean = false,
    val wifiEnabled: Boolean = false,
    val wifiDirectSupported: Boolean = false,
    val wifiAwareSupported: Boolean = false,
    val wifiPermissionGranted: Boolean = false,
)

enum class GlassesConnectionPhase {
    IDLE,
    DISCOVERING,
    CONNECTING,
    CONNECTED,
    ERROR,
}

data class GlassesUiState(
    val capabilities: GlassesCapabilityState = GlassesCapabilityState(),
    val selectedMode: GlassesTransportMode = GlassesTransportMode.BLUETOOTH,
    val phase: GlassesConnectionPhase = GlassesConnectionPhase.IDLE,
    val devices: List<NearbyGlassesDevice> = emptyList(),
    val selectedDeviceId: String? = null,
    val statusMessage: String,
    val connectedEndpoint: String? = null,
) {
    val selectedDevice: NearbyGlassesDevice?
        get() = devices.firstOrNull { it.id == selectedDeviceId }
}

data class GlassesVendorProfile(
    val vendorId: String,
    val displayName: String,
    val supportsBluetoothControl: Boolean,
    val supportsWifiMedia: Boolean,
)

interface GlassesVendorAdapter {
    val profile: GlassesVendorProfile

    fun matches(device: NearbyGlassesDevice): Boolean

    fun canConnect(device: NearbyGlassesDevice): Boolean
}

class GlassesVendorRegistry(
    private val adapters: List<GlassesVendorAdapter> = emptyList(),
) {
    fun adapterFor(device: NearbyGlassesDevice): GlassesVendorAdapter? =
        adapters.firstOrNull { it.matches(device) }

    fun canConnect(device: NearbyGlassesDevice): Boolean =
        adapterFor(device)?.canConnect(device) == true
}
