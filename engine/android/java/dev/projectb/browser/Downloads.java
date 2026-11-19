package dev.projectb.browser;

import android.app.DownloadManager;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.ContentResolver;
import android.content.BroadcastReceiver;
import android.content.ContentValues;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.SharedPreferences;
import android.graphics.drawable.Icon;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.os.SystemClock;
import android.provider.MediaStore;
import android.util.Log;
import android.webkit.MimeTypeMap;

import org.chromium.base.ContextUtils;
import org.jni_zero.CalledByNative;
import org.jni_zero.JNINamespace;
import org.jni_zero.NativeMethods;

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
        // Made public under its own name, or, when the shared Downloads has that name
        // already, perhaps from another app, which this app can't see, under the next free
        // one: big (1).bin, big (2).bin, as the phone names its own copies. Android says so
        // only now, as the copy stops being pending.
        String name = file.getName();
        int dot = name.lastIndexOf('.');
        String stem = dot > 0 ? name.substring(0, dot) : name;
        String ext = dot > 0 ? name.substring(dot) : "";
        for (int n = 0; n < 100; n++) {
            values.clear();
            values.put(MediaStore.Downloads.IS_PENDING, 0);
            if (n > 0) values.put(MediaStore.Downloads.DISPLAY_NAME, stem + " (" + n + ")" + ext);
            try {
                resolver.update(uri, values, null, null);
                return uri;
            } catch (RuntimeException e) {
                Log.w(TAG, "the shared Downloads has " + name + (n > 0 ? " (" + n + ")" : "") + " already");
            }
        }
        resolver.delete(uri, null, null);
        return null;
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

    // The shared copy a download's file became, or null: Share sends that, as the app's own
    // copy is gone once it is made.
    static Uri sharedCopy(String path) {
        String shared = published().getString(path, null);
        return shared == null ? null : Uri.parse(shared);
    }

    // Deletes a download's shared copy, and forgets what the path became.
    @CalledByNative
    static boolean delete(String path) {
        Uri uri = sharedCopy(path);
        if (uri == null) return false;
        try {
            ContextUtils.getApplicationContext().getContentResolver().delete(uri, null, null);
        } catch (RuntimeException e) {
            Log.w(TAG, "could not delete " + path, e);
            return false;
        }
        published().edit().remove(path).apply();
        return true;
    }

    // Opens the shared copy, once the copy publish started is made: Open right after a
    // download ends would otherwise find nothing yet and do nothing. GoSystem's.
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

    // Shows the phone's own list of downloads, where the shared copies are. GoSystem's.
    static void reveal() {
        Context context = ContextUtils.getApplicationContext();
        try {
            context.startActivity(new Intent(DownloadManager.ACTION_VIEW_DOWNLOADS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK));
        } catch (Exception e) {
            Log.w(TAG, "no downloads app", e);
        }
    }

    // The notification of each download (pb_abi.h show_download), by Go's id for it: its
    // progress while it runs, once a second at most, as faster kept the shade from ever
    // settling for anything that reads it,
    // with the time left and Pause and Cancel, or Resume and Cancel while paused; once done,
    // a tap opens the shared copy, posted once that copy is made; cancelled, it goes;
    // failed, it says so. PBDownloadState: 0 progress, 1 done, 2 cancelled, 3 failed. Both
    // channels are quiet, as Chrome's are. The app never asks for notifications here: the
    // download bar offers to, once, and the welcome's last step asks (docs/daily-driver.md).
    private static final String RUNNING = "downloads_running";
    private static final String DONE = "downloads_done";
    // Every download's notification has this tag, so its end replaces its progress.
    private static final String TAG_NOTIFICATION = "downloads";
    // The action, and its extra, the path, of a tap on a finished download's notification.
    static final String OPEN = "dev.projectb.browser.OPEN_DOWNLOAD";
    // The action of a notification's Pause, Resume or Cancel, with Go's id and the
    // PBDownloadCommand.
    private static final String COMMAND = "dev.projectb.browser.DOWNLOAD_COMMAND";
    private static final int CANCEL = 1, PAUSE = 2, RESUME = 3;
    private static final Map<Long, Long> sShownAt = new HashMap<>();
    // Each running download's speed, for its time left: when and how much it had at the
    // last notification, and the bytes a second, smoothed.
    private static final Map<Long, long[]> sLast = new HashMap<>();
    private static final Map<Long, Double> sRate = new HashMap<>();
    private static boolean sListening;

    private static NotificationManager notifications(Context context) {
        NotificationManager manager = context.getSystemService(NotificationManager.class);
        if (manager.getNotificationChannel(RUNNING) == null) {
            manager.createNotificationChannel(
                    new NotificationChannel(RUNNING, "Downloads in progress", NotificationManager.IMPORTANCE_LOW));
            manager.createNotificationChannel(
                    new NotificationChannel(DONE, "Completed downloads", NotificationManager.IMPORTANCE_LOW));
            // The one channel all downloads had until they were two.
            manager.deleteNotificationChannel("downloads");
        }
        if (!sListening) {
            sListening = true;
            ContextUtils.registerNonExportedBroadcastReceiver(context, new BroadcastReceiver() {
                @Override
                public void onReceive(Context c, Intent intent) {
                    DownloadsJni.get().command(intent.getLongExtra("id", 0), intent.getIntExtra("command", 0));
                }
            }, new IntentFilter(COMMAND));
        }
        return manager;
    }

    @CalledByNative
    static void show(long id, String name, String path, int state, long received, long total, boolean paused) {
        Context context = ContextUtils.getApplicationContext();
        NotificationManager manager = notifications(context);
        int tag = (int) id;
        if (state == 2) {
            forget(id);
            manager.cancel(TAG_NOTIFICATION, tag);
            return;
        }
        if (state == 0) {
            long now = SystemClock.elapsedRealtime();
            Long at = sShownAt.get(id);
            if (at != null && now - at < 1000 && !paused) return;
            sShownAt.put(id, now);
            String sizes = total > 0 ? sizeOf(received) + " of " + sizeOf(total) : sizeOf(received);
            String left = paused ? null : timeLeft(id, now, received, total);
            Notification.Builder b = new Notification.Builder(context, RUNNING)
                    .setSmallIcon(android.R.drawable.stat_sys_download)
                    .setContentTitle(name)
                    .setContentText(paused ? "Paused · " + sizes : left != null ? sizes + " · " + left : sizes)
                    .setProgress(total > 0 ? 100 : 0, total > 0 ? (int) (received * 100 / total) : 0, total <= 0)
                    .setOngoing(!paused)
                    .setOnlyAlertOnce(true)
                    .setContentIntent(app(context))
                    .addAction(action(context, id, paused ? RESUME : PAUSE, paused ? "Resume" : "Pause"))
                    .addAction(action(context, id, CANCEL, "Cancel"));
            manager.notify(TAG_NOTIFICATION, tag, b.build());
            return;
        }
        forget(id);
        if (state == 3) {
            manager.notify(TAG_NOTIFICATION, tag, new Notification.Builder(context, DONE)
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
            manager.notify(TAG_NOTIFICATION, tag, new Notification.Builder(context, DONE)
                    .setSmallIcon(android.R.drawable.stat_sys_download_done)
                    .setContentTitle(name)
                    .setContentText(total > 0 ? "Downloaded · " + sizeOf(total) : "Downloaded")
                    .setAutoCancel(true)
                    .setContentIntent(tap)
                    .build());
        });
    }

    private static void forget(long id) {
        sShownAt.remove(id);
        sLast.remove(id);
        sRate.remove(id);
    }

    // A notification's button, which tells Go through PB_EVENT_DOWNLOAD_ACTION, as the
    // bar's and the Downloads page's do; nothing comes to the front.
    private static Notification.Action action(Context context, long id, int command, String label) {
        Intent intent = new Intent(COMMAND).setPackage(context.getPackageName())
                .putExtra("id", id).putExtra("command", command);
        PendingIntent pending = PendingIntent.getBroadcast(context, (int) id * 4 + command, intent,
                PendingIntent.FLAG_IMMUTABLE | PendingIntent.FLAG_UPDATE_CURRENT);
        int icon = command == CANCEL ? android.R.drawable.ic_menu_close_clear_cancel
                : command == PAUSE ? android.R.drawable.ic_media_pause : android.R.drawable.ic_media_play;
        return new Notification.Action.Builder(Icon.createWithResource(context, icon), label, pending).build();
    }

    // The time left at the speed so far, in Chrome's words ("5 secs left", "1 min left",
    // "2 hours left"), once there is a speed to go by, or null. The speed is smoothed over
    // the notifications, a second or more apart, so one slow moment doesn't swing it.
    private static String timeLeft(long id, long now, long received, long total) {
        long[] last = sLast.put(id, new long[] {now, received});
        if (total <= 0 || last == null || now <= last[0]) return null;
        double rate = (received - last[1]) * 1000.0 / (now - last[0]);
        Double was = sRate.get(id);
        if (was != null) rate = 0.7 * was + 0.3 * rate;
        sRate.put(id, rate);
        if (was == null || rate <= 0) return null;
        long secs = Math.max(1, Math.round((total - received) / rate));
        if (secs < 60) return secs + (secs == 1 ? " sec left" : " secs left");
        long mins = Math.round(secs / 60.0);
        if (mins < 60) return mins + (mins == 1 ? " min left" : " mins left");
        long hours = Math.round(secs / 3600.0);
        if (hours < 24) return hours + (hours == 1 ? " hour left" : " hours left");
        long days = Math.round(secs / 86400.0);
        return days + (days == 1 ? " day left" : " days left");
    }

    // A tap that brings the app forward.
    private static PendingIntent app(Context context) {
        Intent intent = new Intent(context, BrowserActivity.class).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP);
        return PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_IMMUTABLE);
    }

    @NativeMethods
    interface Natives {
        // A notification's Pause, Resume or Cancel, to Go (PB_EVENT_DOWNLOAD_ACTION).
        void command(long id, int command);
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
