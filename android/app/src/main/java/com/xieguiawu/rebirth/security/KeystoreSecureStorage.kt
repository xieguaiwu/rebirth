package com.xieguiawu.rebirth.security

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import android.util.Log
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * AES-GCM envelope encryption backed by the Android Keystore:
 *
 * - one master key ("rebirth_master_key") lives in the hardware-backed
 *   AndroidKeyStore and never leaves it;
 * - per-entry data keys are generated randomly, wrapped (encrypted) with the
 *   master key, and stored alongside the GCM IV and ciphertext in
 *   SharedPreferences.
 *
 * API keys are never stored in plaintext and never appear in logs.
 */
class KeystoreSecureStorage(context: Context) : SecureStorage {

    private val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    override fun put(key: String, value: String) {
        try {
            val entryKey = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES)
                .apply { init(256) }
                .generateKey()
            val (wrapped, wrappedIv) = wrapKey(entryKey)
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.ENCRYPT_MODE, entryKey)
            val ct = cipher.doFinal(value.toByteArray(Charsets.UTF_8))
            prefs.edit()
                .putString("$PREFIX$key.wk", Base64.encodeToString(wrapped, Base64.NO_WRAP))
                .putString("$PREFIX$key.wiv", Base64.encodeToString(wrappedIv, Base64.NO_WRAP))
                .putString("$PREFIX$key.iv", Base64.encodeToString(cipher.iv, Base64.NO_WRAP))
                .putString("$PREFIX$key.ct", Base64.encodeToString(ct, Base64.NO_WRAP))
                .apply()
        } catch (e: Exception) {
            Log.e(TAG, "secure put failed", e)
        }
    }

    override fun get(key: String): String? = try {
        val wk = prefs.getString("$PREFIX$key.wk", null) ?: return null
        val wiv = prefs.getString("$PREFIX$key.wiv", null) ?: return null
        val iv = prefs.getString("$PREFIX$key.iv", null) ?: return null
        val ct = prefs.getString("$PREFIX$key.ct", null) ?: return null
        val entryKey = unwrapKey(
            Base64.decode(wk, Base64.NO_WRAP),
            Base64.decode(wiv, Base64.NO_WRAP),
        )
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(
            Cipher.DECRYPT_MODE,
            entryKey,
            GCMParameterSpec(GCM_TAG_BITS, Base64.decode(iv, Base64.NO_WRAP)),
        )
        String(cipher.doFinal(Base64.decode(ct, Base64.NO_WRAP)), Charsets.UTF_8)
    } catch (e: Exception) {
        Log.e(TAG, "secure get failed", e)
        null
    }

    override fun remove(key: String) {
        prefs.edit()
            .remove("$PREFIX$key.wk")
            .remove("$PREFIX$key.wiv")
            .remove("$PREFIX$key.iv")
            .remove("$PREFIX$key.ct")
            .apply()
    }

    // ---- internals --------------------------------------------------------

    private fun masterKey(): SecretKey {
        val ks = KeyStore.getInstance(ANDROID_KEYSTORE).apply { load(null) }
        (ks.getKey(MASTER_ALIAS, null) as? SecretKey)?.let { return it }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, ANDROID_KEYSTORE)
        generator.init(
            KeyGenParameterSpec.Builder(
                MASTER_ALIAS,
                KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
            )
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build(),
        )
        return generator.generateKey()
    }

    /** Encrypt the raw entry-key bytes with the master key; returns (ciphertext, IV). */
    private fun wrapKey(key: SecretKey): Pair<ByteArray, ByteArray> {
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, masterKey())
        return cipher.doFinal(key.encoded) to cipher.iv
    }

    private fun unwrapKey(wrapped: ByteArray, wrappedIv: ByteArray): SecretKey {
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(
            Cipher.DECRYPT_MODE,
            masterKey(),
            GCMParameterSpec(GCM_TAG_BITS, wrappedIv),
        )
        return SecretKeySpec(cipher.doFinal(wrapped), KeyProperties.KEY_ALGORITHM_AES)
    }

    companion object {
        private const val TAG = "RebirthSecure"
        private const val PREFS = "rebirth_secure"
        private const val PREFIX = "key."
        private const val MASTER_ALIAS = "rebirth_master_key"
        private const val ANDROID_KEYSTORE = "AndroidKeyStore"
        private const val TRANSFORMATION = "AES/GCM/NoPadding"
        private const val GCM_TAG_BITS = 128
    }
}
