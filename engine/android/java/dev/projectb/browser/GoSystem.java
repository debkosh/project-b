package dev.projectb.browser;

import android.Manifest;
import android.app.Activity;
import android.app.role.RoleManager;
import android.content.pm.PackageManager;
import android.os.Handler;
import android.os.Looper;

import org.chromium.build.annotations.UsedByReflection;

/**
 * The Go core's door to Android, core/system's Android half (docs/layers.md): what Go asks of
 * Android itself, apart from the engine. go_table.c calls these through JNI from whichever of
 * Go's threads asks; whatever needs the activity hops to the main thread here. Each answer goes
 * back to Go on the main thread, always posted, so it comes after the call that asked has
 * returned.
 */
@UsedByReflection("go_table.c")
final class GoSystem {
    /** What BrowserActivity hears back from Android's sheet asking to be the default browser. */
    static final int DEFAULT_BROWSER_REQUEST = 1;

    /** What BrowserActivity hears back from Android's question about notifications. */
    static final int NOTIFICATIONS_REQUEST = 2;

    // The questions' numbers as Go knows them (core/system/platform).
    private static final int QUESTION_DEFAULT_BROWSER = 1;
    private static final int QUESTION_NOTIFICATIONS = 2;

    private static final Handler sMain = new Handler(Looper.getMainLooper());

    // The one activity, which Go's managers live as long as. Set before Go starts.
    private static Activity sActivity;

    private GoSystem() {}

    static void attach(Activity activity) {
        sActivity = activity;
    }

    /** Opens a finished download's shared copy. */
    @UsedByReflection("go_table.c")
    static void openFile(String path) {
        Downloads.open(path);
    }

    /** Shows the system's Downloads. */
    @UsedByReflection("go_table.c")
    static void revealFile() {
        sMain.post(Downloads::reveal);
    }

    /** Whether the app holds the browser role now. */
    @UsedByReflection("go_table.c")
    static boolean isDefaultBrowser() {
        return sActivity.getSystemService(RoleManager.class).isRoleHeld(RoleManager.ROLE_BROWSER);
    }

    /**
     * Asks Android to make the app the default browser: its own sheet asks the person, and
     * BrowserActivity hears how it ended. Already the default, it says so at once.
     */
    @UsedByReflection("go_table.c")
    static void askDefaultBrowser() {
        sMain.post(
                () -> {
                    RoleManager roles = sActivity.getSystemService(RoleManager.class);
                    if (roles.isRoleHeld(RoleManager.ROLE_BROWSER)) {
                        defaultBrowserAnswered();
                        return;
                    }
                    sActivity.startActivityForResult(
                            roles.createRequestRoleIntent(RoleManager.ROLE_BROWSER),
                            DEFAULT_BROWSER_REQUEST);
                });
    }

    /** Tells Go how asking to be the default browser ended, a sheet closed unanswered too. */
    static void defaultBrowserAnswered() {
        answer(QUESTION_DEFAULT_BROWSER, isDefaultBrowser());
    }

    /** Whether the app may post its notifications now. */
    @UsedByReflection("go_table.c")
    static boolean notificationsAllowed() {
        return android.os.Build.VERSION.SDK_INT < android.os.Build.VERSION_CODES.TIRAMISU
                || sActivity.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS)
                        == PackageManager.PERMISSION_GRANTED;
    }

    /**
     * Asks Android to let the app show its notifications, a download's: Android asks the
     * person from 13 on, and BrowserActivity hears how it ended. Allowed already, or before 13,
     * where nothing asks, it says so at once.
     */
    @UsedByReflection("go_table.c")
    static void askNotifications() {
        sMain.post(
                () -> {
                    if (notificationsAllowed()) {
                        notificationsAnswered();
                        return;
                    }
                    sActivity.requestPermissions(
                            new String[] {Manifest.permission.POST_NOTIFICATIONS},
                            NOTIFICATIONS_REQUEST);
                });
    }

    /** Tells Go how asking to show notifications ended. */
    static void notificationsAnswered() {
        answer(QUESTION_NOTIFICATIONS, notificationsAllowed());
    }

    private static void answer(int question, boolean yes) {
        sMain.post(() -> GoCore.answered(question, yes));
    }
}
