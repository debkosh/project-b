package io.github.sudiptadeb.oskit;

import android.app.Activity;
import android.content.Intent;

/**
 * oskit's Java side (github.com/sudiptadeb/oskit). The app calls attach as its activity is made,
 * before Go asks anything of oskit, and its native code calls Go's OskitAttach once; it hands
 * the activity's intents, permission answers and activity results on with onIntent,
 * onRequestPermissionsResult and onActivityResult.
 */
public final class OsKit {
    private static Activity sActivity;

    private OsKit() {}

    /** The activity oskit shows its sheets over and starts its notes' taps in. */
    public static void attach(Activity activity) {
        sActivity = activity;
    }

    static Activity activity() {
        return sActivity;
    }

    /**
     * An intent the activity was started or brought forward with: true when it was oskit's, a
     * tap on a note or a link from another app, which it has handed to Go. Call it once Go's
     * library is loaded; Go keeps what comes before the app listens.
     */
    public static boolean onIntent(Intent intent) {
        return intent != null && (Notify.onIntent(intent) || Open.onIntent(intent));
    }

    /** The result of an activity oskit started: Android's sheet for the default browser. */
    public static void onActivityResult(int requestCode) {
        if (requestCode == Open.DEFAULT_BROWSER_REQUEST) {
            Open.sheetClosed();
        }
    }

    /** The answer to a permission question: oskit's own, or one it leaves to the app. */
    public static void onRequestPermissionsResult(int requestCode) {
        if (requestCode == Notify.PERMISSION_REQUEST) {
            Notify.permissionAnswered();
        }
    }
}
