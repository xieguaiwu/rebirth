package com.xieguiawu.rebirth.security

/**
 * In-memory [SecureStorage] fake. Used by Robolectric tests (no hardware
 * keystore) and previews; also serves as the contract oracle for
 * SecureStorageTest.
 */
class InMemorySecureStorage : SecureStorage {
    private val map = mutableMapOf<String, String>()

    override fun put(key: String, value: String) {
        map[key] = value
    }

    override fun get(key: String): String? = map[key]

    override fun remove(key: String) {
        map.remove(key)
    }
}
