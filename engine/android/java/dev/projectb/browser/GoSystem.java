package dev.projectb.browser;

import android.app.Activity;
import android.app.KeyguardManager;
import android.hardware.biometrics.BiometricManager;
import android.hardware.biometrics.BiometricPrompt;
import android.os.Build;
import android.os.CancellationSignal;
import android.os.Handler;
import android.os.Looper;
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
    private static final Handler sMain = new Handler(Looper.getMainLooper());

    // The one activity, which Go's managers live as long as. Set before Go starts.
    private static Activity sActivity;

    private GoSystem() {}

    static void attach(Activity activity) {
        sActivity = activity;
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

}
