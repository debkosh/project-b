package dev.projectb.browser;

import android.app.ActivityManager;
import android.app.ApplicationExitInfo;
import android.content.Context;
import android.os.Build;

import org.chromium.base.ContextUtils;
import org.jni_zero.CalledByNative;
import org.jni_zero.JNINamespace;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.util.List;

/**
 * Why an earlier run's process ended (pb_abi.h exit_reason): a crash, or Android ending it
 * for its own reasons, as it does to an app in the background that it wants the memory of,
 * one swiped away or one stopped from its app info. Android 11 and later keep the reason.
 * Before 11, and when Android lost it, as a phone that lost its power does, the app's own
 * crash log says: Android writes a crash there, Java's or a native one, as the process
 * dies, and nothing when it ends one.
 */
@JNINamespace("project_b")
final class ExitReasons {
    private ExitReasons() {}

    // No crash of the process in the app's crash log, and Android kept no reason.
    static final int NO_CRASH_LOGGED = -2;

    // ApplicationExitInfo's REASON_ number for the app's process with this id, else what
    // the crash log says of it, or -1 when neither can be read.
    @CalledByNative
    static int reason(long pid) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            Context context = ContextUtils.getApplicationContext();
            List<ApplicationExitInfo> exits = context.getSystemService(ActivityManager.class)
                    .getHistoricalProcessExitReasons(context.getPackageName(), (int) pid, 1);
            if (!exits.isEmpty()) return exits.get(0).getReason();
        }
        return logged(pid);
    }

    // The process's crash in the app's crash log, as logcat shows an app its own lines:
    // Java's "FATAL EXCEPTION" is a crash, anything else Android wrote there of the process,
    // "Fatal signal" from a native one, Go's abort with it, a native crash.
    private static int logged(long pid) {
        try {
            Process p = new ProcessBuilder("/system/bin/logcat", "-d", "-b", "crash", "--pid=" + pid)
                    .redirectErrorStream(true).start();
            int found = NO_CRASH_LOGGED;
            try (BufferedReader r = new BufferedReader(new InputStreamReader(p.getInputStream()))) {
                for (String line; (line = r.readLine()) != null; ) {
                    if (line.isEmpty() || line.startsWith("---------")) continue;
                    if (line.contains("FATAL EXCEPTION")) return ApplicationExitInfo.REASON_CRASH;
                    found = ApplicationExitInfo.REASON_CRASH_NATIVE;
                }
            }
            return p.waitFor() == 0 ? found : -1;
        } catch (Exception e) {
            return -1;
        }
    }
}
