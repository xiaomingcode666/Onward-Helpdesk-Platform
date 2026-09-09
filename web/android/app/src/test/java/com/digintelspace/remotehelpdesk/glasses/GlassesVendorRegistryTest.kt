package com.digintelspace.remotehelpdesk.glasses

import org.junit.Assert.assertFalse
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test

class GlassesVendorRegistryTest {
    private val device = NearbyGlassesDevice(
        id = "bt:00:11:22:33:44:55",
        name = "Example Glasses",
        address = "00:11:22:33:44:55",
        transport = GlassesTransportMode.BLUETOOTH,
    )

    @Test
    fun unconfiguredRegistryNeverClaimsBluetoothConnectionSupport() {
        assertFalse(GlassesVendorRegistry().canConnect(device))
    }

    @Test
    fun matchingVendorAdapterControlsConnectionAvailability() {
        val adapter = object : GlassesVendorAdapter {
            override val profile = GlassesVendorProfile(
                vendorId = "example",
                displayName = "Example",
                supportsBluetoothControl = true,
                supportsWifiMedia = true,
            )

            override fun matches(device: NearbyGlassesDevice) = device.name.startsWith("Example")

            override fun canConnect(device: NearbyGlassesDevice) = matches(device)
        }
        val registry = GlassesVendorRegistry(listOf(adapter))

        assertSame(adapter, registry.adapterFor(device))
        assertTrue(registry.canConnect(device))
    }
}
