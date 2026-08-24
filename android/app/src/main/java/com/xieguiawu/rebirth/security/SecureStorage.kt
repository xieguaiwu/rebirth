package com.xieguiawu.rebirth.security

/**
 * Abstraction over encrypted storage for LLM API keys. Keys are only ever
 * read to be injected into the core process memory for new_session /
 * resume_session; they are never written to disk in plaintext and never
 * logged.
 */
interface SecureStorage {
    fun put(key: String, value: String)
    fun get(key: String): String?
    fun remove(key: String)
}
