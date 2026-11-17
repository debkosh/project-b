package dev.projectb.browser;

import android.Manifest;
import android.app.Activity;
import android.app.DownloadManager;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.ContentResolver;
import android.content.ContentValues;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.os.Handler;
import android.os.Looper;
import android.os.SystemClock;
import android.provider.MediaStore;
import android.util.Log;
import android.webkit.MimeTypeMap;
import android.widget.Toast;

import org.chromium.base.ApplicationStatus;
import org.chromium.base.ContextUtils;
import org.jni_zero.CalledByNative;
import org.jni_zero.JNINamespace;

import java.io.File;
import java.io.FileInputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.Executor;
import java.util.concurrent.Executors;

/**
 * A finished download, handed to the phone (docs/daily-driver.md): copied into the shared
 * Downloads through MediaStore, where the Files app and other apps find it, and the app's own
 * copy removed. What each file became is kept, so Open in the downloads list opens the shared
 * copy after a restart too.
 */
@JNINamespace("project_b")
final class Downloads {
    private static final String TAG = "pb.downloads";
    // Where each download went: the app's path, as Go lists it, to the shared copy's address.
    private static final String PUBLISHED = "pb.downloads.published";
    // One at a time, off the main thread: a copy can take a while.
    private static final Executor sCopier = Executors.newSingleThreadExecutor();

    private Downloads() {}

    private static SharedPreferences published() {
        return ContextUtils.getApplicationContext().getSharedPreferences(PUBLISHED, Context.MODE_PRIVATE);
    }

    private static String mimeOf(String path, String mime) {
        if (mime != null && !mime.isEmpty() && !mime.equals("application/octet-stream")) return mime;
        String ext = MimeTypeMap.getFileExtensionFromUrl(Uri.fromFile(new File(path)).toString());
        String guessed = ext == null ? null : MimeTypeMap.getSingleton().getMimeTypeFromExtension(ext.toLowerCase());
        return guessed != null ? guessed : "application/octet-stream";
    }

    // Copies the file into the shared Downloads under its own name, and answers where it
    // went, or null.
    private static Uri copyToShared(File file, String mime) {
        ContentResolver resolver = ContextUtils.getApplicationContext().getContentResolver();
        ContentValues values = new ContentValues();
        values.put(MediaStore.Downloads.DISPLAY_NAME, file.getName());
        values.put(MediaStore.Downloads.MIME_TYPE, mimeOf(file.getPath(), mime));
        values.put(MediaStore.Downloads.RELATIVE_PATH, Environment.DIRECTORY_DOWNLOADS);
        values.put(MediaStore.Downloads.IS_PENDING, 1);
        Uri uri = resolver.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, values);
        if (uri == null) {
            Log.w(TAG, "no room in the shared Downloads for " + file.getName());
            return null;
        }
        try (InputStream in = new FileInputStream(file); OutputStream out = resolver.openOutputStream(uri)) {
            byte[] buf = new byte[64 * 1024];
            for (int n; (n = in.read(buf)) > 0; ) out.write(buf, 0, n);
        } catch (Exception e) {
            Log.w(TAG, "could not copy " + file.getName() + " into the shared Downloads", e);
            resolver.delete(uri, null, null);
            return null;
        }
        values.clear();
        values.put(MediaStore.Downloads.IS_PENDING, 0);
        resolver.update(uri, values, null, null);
        return uri;
    }

    @CalledByNative
    static void publish(String path, String mime) {
        sCopier.execute(() -> {
            File file = new File(path);
            Uri uri = copyToShared(file, mime);
            if (uri == null) return;
            published().edit().putString(path, uri.toString()).apply();
            file.delete();
        });
    }

    // A file of ours, such as a crash report, copied into the shared Downloads for the user
    // to find in the Files app, as Android's share sheet may offer nowhere to save it. A
    // short toast says it went to Downloads, or that it could not: a long name would be cut.
    @CalledByNative
    static void save(String path) {
        sCopier.execute(() -> {
            File file = new File(path);
            boolean saved = copyToShared(file, "text/plain") != null;
            Context context = ContextUtils.getApplicationContext();
            String said = saved ? "Report saved to Downloads" : "Couldn't save the report";
            new Handler(Looper.getMainLooper()).post(() -> Toast.makeText(context, said, Toast.LENGTH_LONG).show());
        });
    }

    // Opens the shared copy, once the copy publish started is made: Open right after a
    // download ends would otherwise find nothing yet and do nothing.
    @CalledByNative
    static void open(String path) {
        sCopier.execute(() -> openNow(path));
    }

    private static void openNow(String path) {
        Context context = ContextUtils.getApplicationContext();
        String shared = published().getString(path, null);
        if (shared == null) {
            Log.w(TAG, "nothing shared for " + path + " yet");
            return;
        }
        Uri uri = Uri.parse(shared);
        Intent intent = new Intent(Intent.ACTION_VIEW)
                .setDataAndType(uri, context.getContentResolver().getType(uri))
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_GRANT_READ_URI_PERMISSION);
        try {
            context.startActivity(Intent.createChooser(intent, null).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK));
        } catch (Exception e) {
            Log.w(TAG, "nothing opens " + path, e);
        }
    }

    // Shows the phone's own list of downloads, where the shared copies are.
    @CalledByNative
    static void reveal() {
        Context context = ContextUtils.getApplicationContext();
        try {
            context.startActivity(new Intent(DownloadManager.ACTION_VIEW_DOWNLOADS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK));
        } catch (Exception e) {
            Log.w(TAG, "no downloads app", e);
        }
    }

    // The notification of each download (pb_abi.h show_download), by Go's id for it: its
    // progress while it runs, at most a few times a second as Android drops faster updates;
    // once done, a tap opens the shared copy, posted once that copy is made; cancelled, it
    // goes; failed, it says so. PBDownloadState: 0 progress, 1 done, 2 cancelled, 3 failed.
    private static final String CHANNEL = "downloads";
    // The action, and its extra, the path, of a tap on a finished download's notification.
    static final String OPEN = "dev.projectb.browser.OPEN_DOWNLOAD";
    private static final Map<Long, Long> sShownAt = new HashMap<>();
    private static boolean sAsked;

    @CalledByNative
    static void show(long id, String name, String path, int state, long received, long total, boolean paused) {
        Context context = ContextUtils.getApplicationContext();
        NotificationManager manager = context.getSystemService(NotificationManager.class);
        if (manager.getNotificationChannel(CHANNEL) == null) {
            manager.createNotificationChannel(new NotificationChannel(CHANNEL, "Downloads", NotificationManager.IMPORTANCE_LOW));
        }
        askToNotify(context);
        int tag = (int) id;
        if (state == 2) {
            sShownAt.remove(id);
            manager.cancel(CHANNEL, tag);
            return;
        }
        if (state == 0) {
            long now = SystemClock.elapsedRealtime();
            Long at = sShownAt.get(id);
            if (at != null && now - at < 500 && !paused) return;
            sShownAt.put(id, now);
            Notification.Builder b = new Notification.Builder(context, CHANNEL)
                    .setSmallIcon(android.R.drawable.stat_sys_download)
                    .setContentTitle(name)
                    .setContentText(paused ? "Paused" : total > 0 ? sizeOf(received) + " of " + sizeOf(total) : sizeOf(received))
                    .setProgress(total > 0 ? 100 : 0, total > 0 ? (int) (received * 100 / total) : 0, total <= 0)
                    .setOngoing(!paused)
                    .setOnlyAlertOnce(true)
                    .setContentIntent(app(context));
            manager.notify(CHANNEL, tag, b.build());
            return;
        }
        sShownAt.remove(id);
        if (state == 3) {
            manager.notify(CHANNEL, tag, new Notification.Builder(context, CHANNEL)
                    .setSmallIcon(android.R.drawable.stat_notify_error)
                    .setContentTitle(name)
                    .setContentText("Download failed")
                    .setAutoCancel(true)
                    .setContentIntent(app(context))
                    .build());
            return;
        }
        // Done: a tap opens the file as Open does. It goes through the app's activity, which
        // opens it: Android lets only the app itself hand its shared copy to another app.
        PendingIntent tap = PendingIntent.getActivity(context, tag,
                new Intent(context, BrowserActivity.class).setAction(OPEN).putExtra(OPEN, path)
                        .addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
                PendingIntent.FLAG_IMMUTABLE | PendingIntent.FLAG_UPDATE_CURRENT);
        sCopier.execute(() -> {
            manager.notify(CHANNEL, tag, new Notification.Builder(context, CHANNEL)
                    .setSmallIcon(android.R.drawable.stat_sys_download_done)
                    .setContentTitle(name)
                    .setContentText(total > 0 ? "Downloaded · " + sizeOf(total) : "Downloaded")
                    .setAutoCancel(true)
                    .setContentIntent(tap)
                    .build());
        });
    }

    // A tap that brings the app forward.
    private static PendingIntent app(Context context) {
        Intent intent = new Intent(context, BrowserActivity.class).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP);
        return PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_IMMUTABLE);
    }

    // Android 13 and later show an app's notifications only once the person allows them:
    // asked for at the first download, once a run.
    private static void askToNotify(Context context) {
        if (sAsked || Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) return;
        if (context.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED) return;
        Activity activity = ApplicationStatus.getLastTrackedFocusedActivity();
        if (activity == null) return;
        sAsked = true;
        activity.requestPermissions(new String[] {Manifest.permission.POST_NOTIFICATIONS}, 7);
    }

    // A size as the phone gives it: 1000 to a kilobyte, one decimal place under ten.
    private static String sizeOf(long n) {
        if (n < 1000) return n + " B";
        String[] units = {"KB", "MB", "GB", "TB"};
        double v = n;
        int u = -1;
        do {
            v /= 1000;
            u++;
        } while (v >= 1000 && u < units.length - 1);
        return (v < 10 ? String.format(java.util.Locale.ROOT, "%.1f", v) : String.valueOf(Math.round(v))) + " " + units[u];
    }
}
