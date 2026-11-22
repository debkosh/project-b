package io.github.sudiptadeb.oskit;

import android.content.ClipData;
import android.content.ClipboardManager;
import android.os.Handler;
import android.os.Looper;
import android.os.PersistableBundle;

/** The clipboard's secret copy (oskit/clip). Go calls it through JNI from its own threads. */
final class Clip {
    private static final Handler sMain = new Handler(Looper.getMainLooper());

    private Clip() {}

    /**
     * Puts a secret a person copied, a password, on the clipboard as a clip marked sensitive:
     * Android 13 and later show no preview of it, and keyboards that keep a clipboard's history
     * leave it out. The extra's name is ClipDescription.EXTRA_IS_SENSITIVE's own value, which
     * has no constant before API 33.
     */
    static void copySecret(String text) {
        sMain.post(
                () -> {
                    ClipData clip = ClipData.newPlainText("", text);
                    PersistableBundle extras = new PersistableBundle();
                    extras.putBoolean("android.content.extra.IS_SENSITIVE", true);
                    clip.getDescription().setExtras(extras);
                    OsKit.activity().getSystemService(ClipboardManager.class).setPrimaryClip(clip);
                });
    }
}
