package dev.projectb.browser;

import android.app.Activity;
import android.app.KeyguardManager;
import android.content.Intent;
import android.hardware.biometrics.BiometricManager;
import android.hardware.biometrics.BiometricPrompt;
import android.os.Build;
import android.os.CancellationSignal;
import android.os.Handler;
import android.os.Looper;
import android.util.SparseArray;

import com.google.zxing.client.android.Intents;
import com.journeyapps.barcodescanner.CaptureActivity;

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
    /**
     * What BrowserActivity hears back from the QR scanner's screen: a number no question of
     * oskit's has (0x05c1, 0x05c2).
     */
    static final int SCAN_REQUEST = 0x05d1;

    // The scan Go waits on, by its number, 0 for none: the scanner is one screen.
    private static int sScan;

    private static final Handler sMain = new Handler(Looper.getMainLooper());

    // The one activity, which Go's managers live as long as. Set before Go starts.
    private static Activity sActivity;

    private GoSystem() {}

    static void attach(Activity activity) {
        sActivity = activity;
    }


    /** Settings' Relaunch: the app starts again once it has quit (BrowserActivity.onDestroy). */
    @UsedByReflection("go_table.c")
    static void relaunchAfterQuit() {
        BrowserActivity.relaunch = true;
    }

    /** What Android says of the app's processes that ended, for a crash report (ProcessEnds). */
    @UsedByReflection("go_table.c")
    static String processEnds() {
        return ProcessEnds.text();
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

    /**
     * Opens the QR scanner over the app, under prompt: ZXing's own screen
     * (engine/android/third_party/zxing), which asks for the camera the first time and reads
     * QR codes only. BrowserActivity hears how it ended.
     */
    @UsedByReflection("go_table.c")
    static void scanCode(int id, String prompt) {
        sMain.post(
                () -> {
                    if (sScan != 0) {
                        int waiting = sScan;
                        sScan = 0;
                        GoCore.codeScanned(waiting, "", false);
                    }
                    Intent scan = new Intent(sActivity, CaptureActivity.class);
                    scan.setAction(Intents.Scan.ACTION);
                    scan.putExtra(Intents.Scan.FORMATS, "QR_CODE");
                    scan.putExtra(Intents.Scan.PROMPT_MESSAGE, prompt);
                    scan.putExtra(Intents.Scan.BEEP_ENABLED, false);
                    scan.putExtra(Intents.Scan.ORIENTATION_LOCKED, false);
                    sScan = id;
                    try {
                        sActivity.startActivityForResult(scan, SCAN_REQUEST);
                    } catch (RuntimeException e) {
                        sScan = 0;
                        GoCore.codeScanned(id, "", false);
                    }
                });
    }

    /** Tells Go how the scanner ended: what it read, or that the person left it. */
    static void codeScanned(int resultCode, Intent data) {
        int id = sScan;
        sScan = 0;
        if (id == 0) {
            return;
        }
        String text = resultCode == Activity.RESULT_OK && data != null ? data.getStringExtra(Intents.Scan.RESULT) : null;
        sMain.post(() -> GoCore.codeScanned(id, text == null ? "" : text, text != null));
    }

    private static void personVerified(int id, int verdict) {
        sMain.post(() -> GoCore.personVerified(id, verdict));
    }

}
