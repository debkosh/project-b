package io.github.sudiptadeb.oskit;

import android.app.Activity;
import android.app.DownloadManager;
import android.app.role.RoleManager;
import android.content.Intent;
import android.net.Uri;
import android.os.Handler;
import android.os.Looper;
import android.util.Log;

/**
 * Opening what the app has in other apps, and links from them (oskit/open). Go calls these
 * through JNI from its own threads; whatever needs the activity hops to the main thread.
 */
public final class Open {
    /** The request code of Android's sheet asking to be the default browser. */
    static final int DEFAULT_BROWSER_REQUEST = 0x05c2;

    private static final String TAG = "oskit";
    private static final Handler sMain = new Handler(Looper.getMainLooper());

    private Open() {}

    static native void defaultBrowserAnswered(boolean isDefault);

    static native void opened(String url);

    /**
     * Opens a file in the app the person picks for it, as Chromium opens a download: a
     * content:// address, such as a download's copy in the shared Downloads, is handed on with
     * the right to read it. A file only the app can read has no address another app may open.
     */
    public static void openFile(String path) {
        sMain.post(
                () -> {
                    Activity activity = OsKit.activity();
                    if (!path.startsWith("content://")) {
                        Log.w(TAG, "no content address to open for " + path);
                        return;
                    }
                    Uri uri = Uri.parse(path);
                    Intent view =
                            new Intent(Intent.ACTION_VIEW)
                                    .setDataAndType(uri, activity.getContentResolver().getType(uri))
                                    .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
                    try {
                        activity.startActivity(Intent.createChooser(view, null));
                    } catch (Exception e) {
                        Log.w(TAG, "nothing opens " + path, e);
                    }
                });
    }

    /** Shows the phone's Downloads: Android has no file manager to select a file in. */
    static void reveal(String path) {
        sMain.post(
                () -> {
                    try {
                        OsKit.activity().startActivity(new Intent(DownloadManager.ACTION_VIEW_DOWNLOADS));
                    } catch (Exception e) {
                        Log.w(TAG, "no downloads app", e);
                    }
                });
    }

    /** Whether the app holds the browser role now. */
    static boolean isDefaultBrowser() {
        return OsKit.activity().getSystemService(RoleManager.class).isRoleHeld(RoleManager.ROLE_BROWSER);
    }

    /** Asks Android for the browser role: its own sheet asks the person. Held already, it answers at once. */
    static void askDefaultBrowser() {
        sMain.post(
                () -> {
                    RoleManager roles = OsKit.activity().getSystemService(RoleManager.class);
                    if (roles.isRoleHeld(RoleManager.ROLE_BROWSER)) {
                        defaultBrowserAnswered(true);
                        return;
                    }
                    OsKit.activity()
                            .startActivityForResult(
                                    roles.createRequestRoleIntent(RoleManager.ROLE_BROWSER),
                                    DEFAULT_BROWSER_REQUEST);
                });
    }

    /** The sheet has closed, answered or not. */
    static void sheetClosed() {
        sMain.post(() -> defaultBrowserAnswered(isDefaultBrowser()));
    }

    /** A link another app sent, an ACTION_VIEW intent: true when it was one, handed to Go. */
    static boolean onIntent(Intent intent) {
        if (!Intent.ACTION_VIEW.equals(intent.getAction()) || intent.getDataString() == null) {
            return false;
        }
        opened(intent.getDataString());
        return true;
    }
}
