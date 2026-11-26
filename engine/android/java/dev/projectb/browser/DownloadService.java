package dev.projectb.browser;

import android.app.Notification;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.os.Build;
import android.os.IBinder;
import android.util.Log;

/**
 * Keeps the app's process alive while a download runs, as Chrome's download service does
 * (docs/daily-driver.md, Downloads). Swiping the app away from the recent apps, or Android
 * reclaiming a backgrounded app, ended the process and with it every running download. A
 * foreground service tells Android the app is still at work: its notification is the
 * running download's own, with its progress, Pause and Cancel, so there is nothing more to
 * see. It stops when no download runs. Main thread.
 */
public final class DownloadService extends Service {
    private static final String TAG = "pb.downloads";
    // The id its notification goes under: the download's own, as Android's id for a download's
    // notification always is (Downloads.show), with no tag.
    private static int sId;

    // What the service shows, or null once it should stop; the last shown, for a start that
    // comes after its stop (onStartCommand); the running service, once it has started.
    private static Notification sShown;
    private static Notification sLast;
    private static DownloadService sRunning;
    private static boolean sStarting;

    /** Shows n as the running download's notification, the service keeping the app alive. */
    static void show(Context context, int id, Notification n) {
        sShown = n;
        sLast = n;
        sId = id;
        if (sRunning != null) {
            context.getSystemService(NotificationManager.class).notify(id, n);
            return;
        }
        if (sStarting) return;
        try {
            context.startForegroundService(new Intent(context, DownloadService.class));
            sStarting = true;
        } catch (RuntimeException e) {
            // Android 12 and later refuse a foreground service started from the background:
            // the download still shows, without the service.
            Log.w(TAG, "no download service: " + e);
            sShown = null;
            context.getSystemService(NotificationManager.class).notify(id, n);
        }
    }

    /**
     * Stops the service and takes its notification away; the download's next one is its own.
     * One asked to start and not yet started still starts, as Android demands of it, and
     * stops at once (onStartCommand).
     */
    static void stop(Context context) {
        sShown = null;
        if (sRunning != null) {
            sRunning.end();
        } else if (!sStarting) {
            context.getSystemService(NotificationManager.class).cancel(sId);
        }
    }

    private void end() {
        sRunning = null;
        stopForeground(STOP_FOREGROUND_REMOVE);
        stopSelf();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        sStarting = false;
        Notification n = sShown != null ? sShown : sLast;
        if (n == null) {
            stopSelf();
            return START_NOT_STICKY;
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(sId, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC);
        } else {
            startForeground(sId, n);
        }
        sRunning = this;
        if (sShown == null) end();
        return START_NOT_STICKY;
    }

    // Android 15 ends a data sync service after six hours a day: the download goes on while the
    // app is in front, its notification shown on its own again.
    @Override
    public void onTimeout(int startId, int fgsType) {
        Notification n = sShown;
        end();
        sShown = null;
        if (n != null) getSystemService(NotificationManager.class).notify(sId, n);
    }

    @Override
    public void onDestroy() {
        if (sRunning == this) sRunning = null;
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
