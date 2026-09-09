package com.digintelspace.remotehelpdesk.glasses

import android.Manifest
import android.bluetooth.BluetoothManager
import android.content.Context
import android.content.pm.PackageManager
import android.net.wifi.WifiManager
import android.os.Build
import androidx.core.content.ContextCompat

class GlassesCapabilityProbe(private val context: Context) {
    fun current(): GlassesCapabilityState {
        val packageManager = context.packageManager
        val bluetoothManager = context.getSystemService(BluetoothManager::class.java)
        val wifiManager = context.applicationContext.getSystemService(WifiManager::class.java)

        return GlassesCapabilityState(
            bluetoothSupported = packageManager.hasSystemFeature(PackageManager.FEATURE_BLUETOOTH_LE),
            bluetoothEnabled = bluetoothManager?.adapter?.isEnabled == true,
            bluetoothPermissionGranted = hasBluetoothPermissions(),
            wifiEnabled = wifiManager?.isWifiEnabled == true,
            wifiDirectSupported = packageManager.hasSystemFeature(PackageManager.FEATURE_WIFI_DIRECT),
            wifiAwareSupported = Build.VERSION.SDK_INT >= Build.VERSION_CODES.O &&
                packageManager.hasSystemFeature(PackageManager.FEATURE_WIFI_AWARE),
            wifiPermissionGranted = hasWifiPermission(),
        )
    }

    fun hasBluetoothPermissions(): Boolean {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            return isGranted(Manifest.permission.BLUETOOTH_SCAN) &&
                isGranted(Manifest.permission.BLUETOOTH_CONNECT)
        }
        return isGranted(Manifest.permission.ACCESS_FINE_LOCATION)
    }

    fun hasWifiPermission(): Boolean {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            isGranted(Manifest.permission.NEARBY_WIFI_DEVICES)
        } else {
            isGranted(Manifest.permission.ACCESS_FINE_LOCATION)
        }
    }

    private fun isGranted(permission: String): Boolean =
        ContextCompat.checkSelfPermission(context, permission) == PackageManager.PERMISSION_GRANTED
}
