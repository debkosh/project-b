package dev.projectb.browser;

import android.app.ActivityManager;
import android.app.ApplicationExitInfo;
import android.content.Context;
import android.os.Build;

import org.chromium.base.ContextUtils;

import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.List;
import java.util.Locale;
import java.util.TimeZone;

/**
 * What Android says of the app's processes that ended, newest first, for a crash report: the
 * browser and its children, renderers, the GPU and utilities. A renderer runs under a user id
 * of its own, so the app can't read its lines in logcat, its crash among them; and Chromium
 * can't tell its crash from Android ending it for memory while the page is in use, which it
 * calls OOM_PROTECTED. Android 11 and later keep a record of each process's end that says:
 * ended by a signal (11 a segmentation fault, 6 an abort, 9 a kill), or for low memory, and how
 * much memory it had. Android keeps no tombstone with the record of an isolated process.
 */
final class ProcessEnds {
    private ProcessEnds() {}

    // A report stays one to read.
    private static final int SHOWN = 16;

    static String text() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.R) return "(Android keeps them from 11)";
        Context context = ContextUtils.getApplicationContext();
        List<ApplicationExitInfo> exits = context.getSystemService(ActivityManager.class)
                .getHistoricalProcessExitReasons(context.getPackageName(), 0, 0);
        SimpleDateFormat utc = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.ROOT);
        utc.setTimeZone(TimeZone.getTimeZone("UTC"));
        StringBuilder b = new StringBuilder();
        int shown = 0, unneeded = 0;
        for (ApplicationExitInfo e : exits) {
            // Children Android ends once nothing binds them, as after the browser itself ended:
            // the most of the list, and none of them a crash.
            if (e.getReason() == ApplicationExitInfo.REASON_OTHER
                    && "isolated not needed".equals(e.getDescription())) {
                unneeded++;
                continue;
            }
            if (shown++ == SHOWN) break;
            b.append(utc.format(new Date(e.getTimestamp()))).append(" UTC, pid ").append(e.getPid())
                    .append(", ").append(e.getProcessName()).append(": ").append(reason(e.getReason()))
                    .append(e.getReason() == ApplicationExitInfo.REASON_SIGNALED
                                    || e.getReason() == ApplicationExitInfo.REASON_CRASH_NATIVE
                            ? ", signal " : ", status ")
                    .append(e.getStatus())
                    .append(", importance ").append(e.getImportance())
                    .append(", pss ").append(e.getPss() / 1024).append(" MB, rss ")
                    .append(e.getRss() / 1024).append(" MB");
            if (e.getDescription() != null) b.append(", \"").append(e.getDescription()).append('"');
            b.append('\n');
        }
        if (unneeded > 0) {
            b.append("(and ").append(unneeded).append(" children Android ended as no longer needed)\n");
        }
        return b.length() == 0 ? "(Android has none kept)" : b.toString();
    }

    // ApplicationExitInfo's REASON_ number, in words.
    private static String reason(int r) {
        switch (r) {
            case ApplicationExitInfo.REASON_EXIT_SELF: return "exited by itself";
            case ApplicationExitInfo.REASON_SIGNALED: return "ended by a signal";
            case ApplicationExitInfo.REASON_LOW_MEMORY: return "ended by Android for low memory";
            case ApplicationExitInfo.REASON_CRASH: return "crashed (Java)";
            case ApplicationExitInfo.REASON_CRASH_NATIVE: return "crashed (native)";
            case ApplicationExitInfo.REASON_ANR: return "not responding (ANR)";
            case ApplicationExitInfo.REASON_INITIALIZATION_FAILURE: return "failed to start";
            case ApplicationExitInfo.REASON_PERMISSION_CHANGE: return "a permission changed";
            case ApplicationExitInfo.REASON_EXCESSIVE_RESOURCE_USAGE: return "used too much";
            case ApplicationExitInfo.REASON_USER_REQUESTED: return "stopped by the person";
            case ApplicationExitInfo.REASON_USER_STOPPED: return "stopped by the person";
            case ApplicationExitInfo.REASON_DEPENDENCY_DIED: return "a process it needs ended";
            case ApplicationExitInfo.REASON_OTHER: return "ended by Android";
            case ApplicationExitInfo.REASON_FREEZER: return "ended while frozen";
            case ApplicationExitInfo.REASON_PACKAGE_STATE_CHANGE: return "the app was turned off";
            case ApplicationExitInfo.REASON_PACKAGE_UPDATED: return "the app was updated";
            default: return "reason " + r;
        }
    }
}
