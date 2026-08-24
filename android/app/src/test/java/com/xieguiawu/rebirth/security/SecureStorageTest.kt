package com.xieguiawu.rebirth.security

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * Contract tests for [SecureStorage] against the in-memory fake (the
 * hardware-backed [KeystoreSecureStorage] needs a real device; the fake
 * doubles as the contract oracle).
 */
class SecureStorageTest {

    @Test
    fun putThenGet_returnsStoredValue() {
        val storage = InMemorySecureStorage()
        storage.put("provider_key_a", "sk-test-123")
        assertEquals("sk-test-123", storage.get("provider_key_a"))
    }

    @Test
    fun getMissingKey_returnsNull() {
        assertNull(InMemorySecureStorage().get("missing"))
    }

    @Test
    fun overwrite_replacesValue() {
        val storage = InMemorySecureStorage()
        storage.put("k", "v1")
        storage.put("k", "v2")
        assertEquals("v2", storage.get("k"))
    }

    @Test
    fun remove_deletesValue() {
        val storage = InMemorySecureStorage()
        storage.put("k", "v")
        storage.remove("k")
        assertNull(storage.get("k"))
    }

    @Test
    fun keys_areIsolated() {
        val storage = InMemorySecureStorage()
        storage.put("a", "1")
        storage.put("b", "2")
        assertEquals("1", storage.get("a"))
        assertEquals("2", storage.get("b"))
        storage.remove("a")
        assertNull(storage.get("a"))
        assertEquals("2", storage.get("b"))
    }

    @Test
    fun unicodeValues_surviveRoundTrip() {
        val storage = InMemorySecureStorage()
        storage.put("k", "密钥🔑-value")
        assertEquals("密钥🔑-value", storage.get("k"))
    }
}
