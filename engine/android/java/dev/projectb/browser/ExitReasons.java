package dev.projectb.browser;

import android.app.ActivityManager;
import android.app.ApplicationExitInfo;
import android.content.Context;
import android.os.Build;

import org.chromium.base.ContextUtils;
import org.jni_zero.CalledByNative;
import org.jni_zero.JNINamespace;

import java.util.List;

/**
 * Why an earlier run's process ended, as Android 11 and later keep it (pb_abi.h
 * exit_reason): a crash, or Android ending it for its own reasons, as it does to an app in
 * the background that it wants the memory of, or one swiped away.
 */
@JNINamespace("project_b")
final class ExitReasons {
    private ExitReasons() {}

    // ApplicationExitInfo's REASON_ number for the app's process with this id, or -1 when
    // Android doesn't say.
    @CalledByNative
    static int reason(long pid) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.R) return -1;
        Context context = ContextUtils.getApplicationContext();
        List<ApplicationExitInfo> exits = context.getSystemService(ActivityManager.class)
                .getHistoricalProcessExitReasons(context.getPackageName(), (int) pid, 1);
        return exits.isEmpty() ? -1 : exits.get(0).getReason();
    }
}
