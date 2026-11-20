package dev.projectb.browser;

import android.Manifest;
import android.app.Activity;
import android.app.KeyguardManager;
import android.app.role.RoleManager;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.pm.PackageManager;
import android.hardware.biometrics.BiometricManager;
import android.hardware.biometrics.BiometricPrompt;
import android.os.Build;
import android.os.CancellationSignal;
import android.os.Handler;
import android.os.Looper;
import android.os.PersistableBundle;
import android.util.SparseArray;

import io.github.sudiptadeb.amarit.UpdateNotification;

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

    /**
     * Puts a secret a person copied, a password, on the clipboard as a clip marked sensitive:
     * Android 13 and later show no preview of it, and keyboards that keep a clipboard's history
     * leave it out. The extra's name is ClipDescription.EXTRA_IS_SENSITIVE's own value, which
     * has no constant before API 33.
     */
    @UsedByReflection("go_table.c")
    static void copySecret(String text) {
        sMain.post(() -> {
            ClipData clip = ClipData.newPlainText("", text);
            PersistableBundle extras = new PersistableBundle();
            extras.putBoolean("android.content.extra.IS_SENSITIVE", true);
            clip.getDescription().setExtras(extras);
            sActivity.getSystemService(ClipboardManager.class).setPrimaryClip(clip);
        });
    }

    /**
     * The Go updater's state, as its JSON, at each change: amarit's UpdateNotification shows it
     * as the system's notification, progress with Cancel, Install when ready, Try again after a
     * failure a retry can help. Its taps go back to Go.
     */
    @UsedByReflection("go_table.c")
    static void showUpdate(String json) {
        sMain.post(() -> {
            UpdateNotification.commands = GoCore::updateCommand;
            UpdateNotification.show(sActivity, json);
        });
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

    // How a check of the person ended, as Go knows it (core/system's Verdict).
    private static final int VERIFIED = 0;
    private static final int DENIED = 1;
    private static final int NO_SCREEN_LOCK = 2;

    // The checks of the person Android has not answered, by Go's number. Main thread.
    private static final SparseArray<CancellationSignal> sChecks = new SparseArray<>();

    /**
     * Checks the person is the phone's owner, in Android's own sheet: a fingerprint, a face or
     * the screen lock. A phone with no screen lock has nothing to check with, and says so.
     */
    @UsedByReflection("go_table.c")
    @SuppressWarnings("deprecation") // setDeviceCredentialAllowed is Android 10's way.
    static void verifyPerson(int id, String title, String subtitle) {
        sMain.post(
                () -> {
                    if (!sActivity.getSystemService(KeyguardManager.class).isDeviceSecure()) {
                        personVerified(id, NO_SCREEN_LOCK);
                        return;
                    }
                    BiometricPrompt.Builder prompt =
                            new BiometricPrompt.Builder(sActivity)
                                    .setTitle(title)
                                    .setSubtitle(subtitle);
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                        prompt.setAllowedAuthenticators(
                                BiometricManager.Authenticators.BIOMETRIC_STRONG
                                        | BiometricManager.Authenticators.DEVICE_CREDENTIAL);
                    } else {
                        prompt.setDeviceCredentialAllowed(true);
                    }
                    CancellationSignal cancel = new CancellationSignal();
                    sChecks.put(id, cancel);
                    prompt.build()
                            .authenticate(
                                    cancel,
                                    sActivity.getMainExecutor(),
                                    new BiometricPrompt.AuthenticationCallback() {
                                        @Override
                                        public void onAuthenticationSucceeded(
                                                BiometricPrompt.AuthenticationResult result) {
                                            sChecks.remove(id);
                                            personVerified(id, VERIFIED);
                                        }

                                        // A failed try keeps the sheet up; an error ends it.
                                        @Override
                                        public void onAuthenticationError(
                                                int code, CharSequence words) {
                                            sChecks.remove(id);
                                            personVerified(id, DENIED);
                                        }
                                    });
                });
    }

    /** Takes the sheet of check id away. Go no longer waits for it. */
    @UsedByReflection("go_table.c")
    static void cancelPerson(int id) {
        sMain.post(
                () -> {
                    CancellationSignal cancel = sChecks.get(id);
                    if (cancel != null) {
                        sChecks.remove(id);
                        cancel.cancel();
                    }
                });
    }

    private static void personVerified(int id, int verdict) {
        sMain.post(() -> GoCore.personVerified(id, verdict));
    }

    private static void answer(int question, boolean yes) {
        sMain.post(() -> GoCore.answered(question, yes));
    }
}
