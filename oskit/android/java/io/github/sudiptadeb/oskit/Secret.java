package io.github.sudiptadeb.oskit;

import android.security.keystore.KeyGenParameterSpec;
import android.security.keystore.KeyProperties;

import java.io.File;
import java.io.FileOutputStream;
import java.nio.file.Files;
import java.security.KeyStore;
import java.security.SecureRandom;

import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;

/**
 * The device key (oskit/secret): 32 random bytes kept in the app's files as device.key, sealed
 * by an AES key of the Android Keystore that never leaves it, under an alias named after the
 * key's name ("Project B": project_b_device_key). An app has one device key. Go asks for it
 * through JNI on a thread of its own, so nothing here runs on the main thread.
 */
final class Secret {
    private static final String FILE = "device.key";
    private static final int IV_BYTES = 12;
    private static final int TAG_BITS = 128;

    private Secret() {}

    /**
     * The device key. When there is none it makes one if create is true, else returns null. A
     * device.key without its Keystore key, as after a restore to another phone, is none: its key
     * is gone. One the Keystore key doesn't open throws, and is left as it is.
     */
    static synchronized byte[] get(String name, boolean create) throws Exception {
        String alias = name.toLowerCase(java.util.Locale.ROOT).replace(' ', '_') + "_device_key";
        File file = new File(OsKit.activity().getApplicationContext().getFilesDir(), FILE);
        KeyStore store = KeyStore.getInstance("AndroidKeyStore");
        store.load(null);
        SecretKey wrap = (SecretKey) store.getKey(alias, null);
        if (wrap != null && file.exists()) {
            byte[] sealed = Files.readAllBytes(file.toPath());
            Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
            cipher.init(Cipher.DECRYPT_MODE, wrap, new GCMParameterSpec(TAG_BITS, sealed, 0, IV_BYTES));
            return cipher.doFinal(sealed, IV_BYTES, sealed.length - IV_BYTES);
        }
        if (!create) {
            return null;
        }
        if (wrap == null) {
            KeyGenerator gen = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore");
            gen.init(new KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT | KeyProperties.PURPOSE_DECRYPT)
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .setKeySize(256)
                    .build());
            wrap = gen.generateKey();
        }
        byte[] key = new byte[32];
        new SecureRandom().nextBytes(key);
        // The Keystore picks the IV.
        Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
        cipher.init(Cipher.ENCRYPT_MODE, wrap);
        byte[] iv = cipher.getIV();
        byte[] sealed = cipher.doFinal(key);
        File tmp = new File(file.getPath() + ".tmp");
        try (FileOutputStream out = new FileOutputStream(tmp)) {
            out.write(iv);
            out.write(sealed);
            out.getFD().sync();
        }
        if (!tmp.renameTo(file)) {
            throw new java.io.IOException("can't rename " + tmp + " to " + file);
        }
        return key;
    }
}
