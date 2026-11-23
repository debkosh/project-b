package io.github.sudiptadeb.oskit;

import android.Manifest;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.os.Handler;
import android.os.Looper;
import android.provider.Settings;

import java.util.ArrayList;
import java.util.List;

/**
 * The app's own camera and microphone permission (oskit/media): Android's runtime permissions,
 * CAMERA and RECORD_AUDIO, which the manifest declares. Go calls these through JNI; asking
 * hops to the main thread, and the answer goes back there.
 */
final class Media {
    /** The request code of Android's question about the camera and microphone. */
    static final int PERMISSION_REQUEST = 0x05c3;

    private static final Handler sMain = new Handler(Looper.getMainLooper());

    private Media() {}

    static native void answered(boolean camera, boolean microphone);

    static native void returned();

    private static String permissionOf(int kind) {
        return kind == 0 ? Manifest.permission.CAMERA : Manifest.permission.RECORD_AUDIO;
    }

    /** Whether Android lets the app use kind: 0 the camera, 1 the microphone. */
    static boolean allowed(int kind) {
        return OsKit.activity().checkSelfPermission(permissionOf(kind))
                == PackageManager.PERMISSION_GRANTED;
    }

    /** Asks Android for the kinds not yet allowed; with none, or refused for good, it answers at once. */
    static void ask(boolean camera, boolean microphone) {
        sMain.post(
                () -> {
                    List<String> asked = new ArrayList<>();
                    if (camera && !allowed(0)) asked.add(permissionOf(0));
                    if (microphone && !allowed(1)) asked.add(permissionOf(1));
                    if (asked.isEmpty()) {
                        answered(allowed(0), allowed(1));
                        return;
                    }
                    OsKit.activity().requestPermissions(asked.toArray(new String[0]), PERMISSION_REQUEST);
                });
    }

    static void permissionAnswered() {
        sMain.post(() -> answered(allowed(0), allowed(1)));
    }

    /** Opens the app's details in Android's settings, where its permissions are. */
    static void openSettings() {
        sMain.post(
                () ->
                        OsKit.activity()
                                .startActivity(
                                        new Intent(
                                                Settings.ACTION_APPLICATION_DETAILS_SETTINGS,
                                                Uri.fromParts(
                                                        "package",
                                                        OsKit.activity().getPackageName(),
                                                        null))));
    }
}
