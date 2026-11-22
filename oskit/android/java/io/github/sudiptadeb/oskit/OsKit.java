package io.github.sudiptadeb.oskit;

import android.app.Activity;
import android.content.Intent;

/**
 * oskit's Java side (github.com/sudiptadeb/oskit). The app calls attach as its activity is made,
 * before Go asks anything of oskit, and its native code calls Go's OskitAttach once; it hands
 * the activity's new intents and permission answers on with onIntent and
 * onRequestPermissionsResult.
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
     * tap on a note, which it has handed to Go.
     */
    public static boolean onIntent(Intent intent) {
        return Notify.onIntent(intent);
    }

    /** The answer to a permission question: oskit's own, or one it leaves to the app. */
    public static void onRequestPermissionsResult(int requestCode) {
        if (requestCode == Notify.PERMISSION_REQUEST) {
            Notify.permissionAnswered();
        }
    }
}
