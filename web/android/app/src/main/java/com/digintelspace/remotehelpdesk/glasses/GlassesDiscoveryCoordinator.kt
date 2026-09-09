package com.digintelspace.remotehelpdesk.glasses

import android.annotation.SuppressLint
import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothManager
import android.bluetooth.le.ScanCallback
import android.bluetooth.le.ScanResult
import android.bluetooth.le.ScanSettings
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.net.wifi.p2p.WifiP2pConfig
import android.net.wifi.p2p.WifiP2pDeviceList
import android.net.wifi.p2p.WifiP2pInfo
import android.net.wifi.p2p.WifiP2pManager
import android.os.Handler
import android.os.Looper
import androidx.core.content.ContextCompat
import com.digintelspace.remotehelpdesk.R

interface GlassesDiscoveryListener {
    fun onDevicesChanged(mode: GlassesTransportMode, devices: List<NearbyGlassesDevice>)

    fun onDiscoveryStateChanged(
        mode: GlassesTransportMode,
        discovering: Boolean,
        message: String,
        failed: Boolean = false,
    )

    fun onConnectionChanged(
        connected: Boolean,
        message: String,
        endpoint: String? = null,
        failed: Boolean = false,
    )
}

class GlassesDiscoveryCoordinator(
    private val context: Context,
    private val listener: GlassesDiscoveryListener,
) {
    private val bluetooth = BluetoothGlassesDiscovery(context.applicationContext, listener)
    private val wifiDirect = WifiDirectGlassesDiscovery(context.applicationContext, listener)

    fun start(mode: GlassesTransportMode) {
        stop()
        when (mode) {
            GlassesTransportMode.BLUETOOTH -> bluetooth.start()
            GlassesTransportMode.WIFI_DIRECT -> wifiDirect.start()
        }
    }

    fun stop() {
        bluetooth.stop()
        wifiDirect.stopDiscovery()
    }

    fun connect(device: NearbyGlassesDevice) {
        when (device.transport) {
            GlassesTransportMode.BLUETOOTH -> listener.onConnectionChanged(
                false,
                context.getString(R.string.glasses_err_bluetooth_needs_vendor_protocol),
            )
            GlassesTransportMode.WIFI_DIRECT -> wifiDirect.connect(device)
        }
    }

    fun close() {
        stop()
        wifiDirect.close()
    }
}

private class BluetoothGlassesDiscovery(
    context: Context,
    private val listener: GlassesDiscoveryListener,
) {
    private val adapter: BluetoothAdapter? =
        context.getSystemService(BluetoothManager::class.java)?.adapter
    private val handler = Handler(Looper.getMainLooper())
    private val devices = linkedMapOf<String, NearbyGlassesDevice>()
    private var scanning = false

    private val timeout = Runnable {
        stop()
        listener.onDiscoveryStateChanged(
            GlassesTransportMode.BLUETOOTH,
            false,
            if (devices.isEmpty()) context.getString(R.string.glasses_bt_no_devices_found)
            else context.getString(R.string.glasses_bt_scan_complete),
        )
    }

    private val scanCallback = object : ScanCallback() {
        override fun onScanResult(callbackType: Int, result: ScanResult) {
            upsert(result)
        }

        override fun onBatchScanResults(results: MutableList<ScanResult>) {
            results.forEach(::upsert)
        }

        override fun onScanFailed(errorCode: Int) {
            scanning = false
            handler.removeCallbacks(timeout)
            listener.onDiscoveryStateChanged(
                GlassesTransportMode.BLUETOOTH,
                false,
                context.getString(R.string.glasses_bt_scan_failed, errorCode),
                failed = true,
            )
        }
    }

    @SuppressLint("MissingPermission")
    fun start() {
        val bluetoothAdapter = adapter
        if (bluetoothAdapter == null) {
            listener.onDiscoveryStateChanged(
                GlassesTransportMode.BLUETOOTH,
                false,
                context.getString(R.string.glasses_bt_unsupported),
                failed = true,
            )
            return
        }
        if (!bluetoothAdapter.isEnabled) {
            listener.onDiscoveryStateChanged(
                GlassesTransportMode.BLUETOOTH,
                false,
                context.getString(R.string.glasses_bt_enable_required),
                failed = true,
            )
            return
        }

        val scanner = bluetoothAdapter.bluetoothLeScanner
        if (scanner == null) {
            listener.onDiscoveryStateChanged(
                GlassesTransportMode.BLUETOOTH,
                false,
                context.getString(R.string.glasses_bt_scanner_unavailable),
                failed = true,
            )
            return
        }

        devices.clear()
        bluetoothAdapter.bondedDevices.orEmpty().forEach { device ->
            val address = device.address ?: return@forEach
            devices["bt:$address"] = NearbyGlassesDevice(
                id = "bt:$address",
                name = device.name?.takeIf(String::isNotBlank)
                    ?: context.getString(R.string.glasses_bt_paired_device_fallback),
                address = address,
                transport = GlassesTransportMode.BLUETOOTH,
                paired = true,
            )
        }
        publish()

        val settings = ScanSettings.Builder()
            .setScanMode(ScanSettings.SCAN_MODE_LOW_LATENCY)
            .build()
        scanning = true
        listener.onDiscoveryStateChanged(
            GlassesTransportMode.BLUETOOTH,
            true,
            context.getString(R.string.glasses_bt_searching),
        )
        scanner.startScan(emptyList(), settings, scanCallback)
        handler.removeCallbacks(timeout)
        handler.postDelayed(timeout, SCAN_DURATION_MILLIS)
    }

    @SuppressLint("MissingPermission")
    fun stop() {
        if (!scanning) return
        scanning = false
        handler.removeCallbacks(timeout)
        try {
            adapter?.bluetoothLeScanner?.stopScan(scanCallback)
        } catch (_: SecurityException) {
            // Permission state can change while a scan is active.
        }
    }

    @SuppressLint("MissingPermission")
    private fun upsert(result: ScanResult) {
        val address = result.device.address ?: return
        val id = "bt:$address"
        val advertisedName = result.scanRecord?.deviceName
        val deviceName = try {
            result.device.name
        } catch (_: SecurityException) {
            null
        }
        val existing = devices[id]
        devices[id] = NearbyGlassesDevice(
            id = id,
            name = advertisedName?.takeIf(String::isNotBlank)
                ?: deviceName?.takeIf(String::isNotBlank)
                ?: existing?.name
                ?: context.getString(R.string.glasses_bt_unnamed_device),
            address = address,
            transport = GlassesTransportMode.BLUETOOTH,
            signalStrength = result.rssi,
            paired = existing?.paired == true,
        )
        publish()
    }

    private fun publish() {
        listener.onDevicesChanged(
            GlassesTransportMode.BLUETOOTH,
            devices.values.sortedWith(
                compareByDescending<NearbyGlassesDevice> { it.paired }
                    .thenByDescending { it.signalStrength ?: Int.MIN_VALUE },
            ),
        )
    }

    private companion object {
        const val SCAN_DURATION_MILLIS = 10_000L
    }
}

private class WifiDirectGlassesDiscovery(
    private val context: Context,
    private val listener: GlassesDiscoveryListener,
) {
    private val manager = context.getSystemService(WifiP2pManager::class.java)
    private val channel = manager?.initialize(context, Looper.getMainLooper(), null)
    private var receiverRegistered = false

    private val receiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            when (intent.action) {
                WifiP2pManager.WIFI_P2P_PEERS_CHANGED_ACTION -> requestPeers()
                WifiP2pManager.WIFI_P2P_CONNECTION_CHANGED_ACTION -> requestConnectionInfo()
                WifiP2pManager.WIFI_P2P_STATE_CHANGED_ACTION -> {
                    val enabled = intent.getIntExtra(
                        WifiP2pManager.EXTRA_WIFI_STATE,
                        WifiP2pManager.WIFI_P2P_STATE_DISABLED,
                    ) == WifiP2pManager.WIFI_P2P_STATE_ENABLED
                    if (!enabled) {
                        listener.onDiscoveryStateChanged(
                            GlassesTransportMode.WIFI_DIRECT,
                            false,
                            context.getString(R.string.glasses_wifi_direct_unavailable),
                            failed = true,
                        )
                    }
                }
            }
        }
    }

    @SuppressLint("MissingPermission")
    fun start() {
        val p2pManager = manager
        val p2pChannel = channel
        if (p2pManager == null || p2pChannel == null) {
            listener.onDiscoveryStateChanged(
                GlassesTransportMode.WIFI_DIRECT,
                false,
                context.getString(R.string.glasses_wifi_direct_unsupported),
                failed = true,
            )
            return
        }
        ensureReceiver()
        listener.onDiscoveryStateChanged(
            GlassesTransportMode.WIFI_DIRECT,
            true,
            context.getString(R.string.glasses_wifi_direct_searching),
        )
        p2pManager.discoverPeers(p2pChannel, object : WifiP2pManager.ActionListener {
            override fun onSuccess() = Unit

            override fun onFailure(reason: Int) {
                listener.onDiscoveryStateChanged(
                    GlassesTransportMode.WIFI_DIRECT,
                    false,
                    context.getString(R.string.glasses_wifi_direct_scan_failed, reason),
                    failed = true,
                )
            }
        })
    }

    @SuppressLint("MissingPermission")
    fun stopDiscovery() {
        val p2pManager = manager ?: return
        val p2pChannel = channel ?: return
        try {
            p2pManager.stopPeerDiscovery(p2pChannel, null)
        } catch (_: SecurityException) {
            // Permission state can change while discovery is active.
        }
    }

    @SuppressLint("MissingPermission")
    fun connect(device: NearbyGlassesDevice) {
        val p2pManager = manager
        val p2pChannel = channel
        if (p2pManager == null || p2pChannel == null) {
            listener.onConnectionChanged(
                false,
                context.getString(R.string.glasses_wifi_direct_unavailable),
                failed = true,
            )
            return
        }
        ensureReceiver()
        val config = WifiP2pConfig().apply {
            deviceAddress = device.address
        }
        listener.onConnectionChanged(
            false,
            context.getString(R.string.glasses_wifi_direct_connecting, device.name),
        )
        p2pManager.connect(p2pChannel, config, object : WifiP2pManager.ActionListener {
            override fun onSuccess() {
                listener.onConnectionChanged(false, context.getString(R.string.glasses_wifi_direct_request_sent))
            }

            override fun onFailure(reason: Int) {
                listener.onConnectionChanged(
                    false,
                    context.getString(R.string.glasses_wifi_direct_connect_failed, reason),
                    failed = true,
                )
            }
        })
    }

    fun close() {
        if (!receiverRegistered) return
        context.unregisterReceiver(receiver)
        receiverRegistered = false
    }

    private fun ensureReceiver() {
        if (receiverRegistered) return
        val filter = IntentFilter().apply {
            addAction(WifiP2pManager.WIFI_P2P_STATE_CHANGED_ACTION)
            addAction(WifiP2pManager.WIFI_P2P_PEERS_CHANGED_ACTION)
            addAction(WifiP2pManager.WIFI_P2P_CONNECTION_CHANGED_ACTION)
        }
        ContextCompat.registerReceiver(
            context,
            receiver,
            filter,
            ContextCompat.RECEIVER_NOT_EXPORTED,
        )
        receiverRegistered = true
    }

    @SuppressLint("MissingPermission")
    private fun requestPeers() {
        val p2pManager = manager ?: return
        val p2pChannel = channel ?: return
        try {
            p2pManager.requestPeers(p2pChannel, ::publishPeers)
        } catch (_: SecurityException) {
            listener.onDiscoveryStateChanged(
                GlassesTransportMode.WIFI_DIRECT,
                false,
                context.getString(R.string.glasses_wifi_direct_permission_missing),
                failed = true,
            )
        }
    }

    @SuppressLint("MissingPermission")
    private fun requestConnectionInfo() {
        val p2pManager = manager ?: return
        val p2pChannel = channel ?: return
        try {
            p2pManager.requestConnectionInfo(p2pChannel, ::publishConnectionInfo)
        } catch (_: SecurityException) {
            listener.onConnectionChanged(
                false,
                context.getString(R.string.glasses_wifi_direct_permission_missing),
                failed = true,
            )
        }
    }

    private fun publishPeers(peerList: WifiP2pDeviceList) {
        val devices = peerList.deviceList.map { device ->
            NearbyGlassesDevice(
                id = "wifi:${device.deviceAddress}",
                name = device.deviceName?.takeIf(String::isNotBlank)
                    ?: context.getString(R.string.glasses_wifi_direct_unnamed_device),
                address = device.deviceAddress,
                transport = GlassesTransportMode.WIFI_DIRECT,
            )
        }.sortedBy { it.name.lowercase() }
        listener.onDevicesChanged(GlassesTransportMode.WIFI_DIRECT, devices)
        listener.onDiscoveryStateChanged(
            GlassesTransportMode.WIFI_DIRECT,
            true,
            if (devices.isEmpty()) context.getString(R.string.glasses_wifi_direct_searching)
            else context.getString(R.string.glasses_wifi_direct_devices_found, devices.size),
        )
    }

    private fun publishConnectionInfo(info: WifiP2pInfo) {
        if (!info.groupFormed) return
        val endpoint = info.groupOwnerAddress?.hostAddress
        listener.onConnectionChanged(
            true,
            context.getString(R.string.glasses_wifi_direct_connected),
            endpoint,
        )
    }
}
