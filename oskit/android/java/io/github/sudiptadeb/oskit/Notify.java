package io.github.sudiptadeb.oskit;

import android.Manifest;
import android.app.Activity;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.os.Build;
import android.os.Handler;
import android.os.Looper;

/**
 * The app's own notes in the shade (oskit/notify). Go calls these through JNI from its own
 * threads; each hops to the main thread. A tap comes back to the activity as an intent, which
 * OsKit.onIntent hands to Go.
 */
final class Notify {
    /** The request code of Android's question about notifications. */
    static final int PERMISSION_REQUEST = 0x05c1;

    private static final String CHANNEL = "oskit.notes";
    private static final String TAP = "io.github.sudiptadeb.oskit.NOTE";
    private static final String EXTRA_ID = "oskit.id";
    private static final String EXTRA_ACTION = "oskit.action";

    private static final Handler sMain = new Handler(Looper.getMainLooper());

    private Notify() {}

    static native void clicked(String id, String action);

    static native void answered(boolean allowed);

    /** Whether the app may post notifications now: always before Android 13. */
    static boolean allowed() {
        Activity activity = OsKit.activity();
        return Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU
                || activity.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS)
                        == PackageManager.PERMISSION_GRANTED;
    }

    /** Asks Android to let the app notify; allowed already, or before 13, it answers at once. */
    static void ask() {
        sMain.post(
                () -> {
                    if (allowed()) {
                        answered(true);
                        return;
                    }
                    OsKit.activity()
                            .requestPermissions(
                                    new String[] {Manifest.permission.POST_NOTIFICATIONS},
                                    PERMISSION_REQUEST);
                });
    }

    static void permissionAnswered() {
        sMain.post(() -> answered(allowed()));
    }

    private static NotificationManager manager() {
        Activity activity = OsKit.activity();
        NotificationManager manager = activity.getSystemService(NotificationManager.class);
        if (manager.getNotificationChannel(CHANNEL) == null) {
            manager.createNotificationChannel(
                    new NotificationChannel(
                            CHANNEL, "Notifications", NotificationManager.IMPORTANCE_DEFAULT));
        }
        return manager;
    }

    /** The intent a tap on note id, or on its button action, starts the activity with. */
    private static PendingIntent tap(String id, String action) {
        Activity activity = OsKit.activity();
        Intent intent =
                new Intent(activity, activity.getClass())
                        .setAction(TAP)
                        .putExtra(EXTRA_ID, id)
                        .putExtra(EXTRA_ACTION, action)
                        .addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP);
        return PendingIntent.getActivity(
                activity,
                (id + "\n" + action).hashCode(),
                intent,
                PendingIntent.FLAG_IMMUTABLE | PendingIntent.FLAG_UPDATE_CURRENT);
    }

    static void show(
            String id,
            String title,
            String subtitle,
            String body,
            String[] actionIds,
            String[] actionLabels) {
        sMain.post(
                () -> {
                    Activity activity = OsKit.activity();
                    Notification.Builder b =
                            new Notification.Builder(activity, CHANNEL)
                                    .setSmallIcon(android.R.drawable.ic_popup_reminder)
                                    .setContentTitle(title)
                                    .setContentText(body)
                                    .setSubText(subtitle)
                                    .setStyle(new Notification.BigTextStyle().bigText(body))
                                    .setContentIntent(tap(id, ""))
                                    .setAutoCancel(true);
                    for (int i = 0; i < actionIds.length; i++) {
                        b.addAction(
                                new Notification.Action.Builder(
                                                null, actionLabels[i], tap(id, actionIds[i]))
                                        .build());
                    }
                    manager().notify(tag(id), 0, b.build());
                });
    }

    static void remove(String id) {
        sMain.post(() -> manager().cancel(tag(id), 0));
    }

    /** The tag a note is posted under: its id, apart from the app's own notifications. */
    private static String tag(String id) {
        return "oskit:" + id;
    }

    static boolean onIntent(Intent intent) {
        if (intent == null || !TAP.equals(intent.getAction())) {
            return false;
        }
        String id = intent.getStringExtra(EXTRA_ID);
        String action = intent.getStringExtra(EXTRA_ACTION);
        // A button's tap leaves its note up, where the note's own tap takes it away.
        if (!action.isEmpty()) {
            remove(id);
        }
        clicked(id, action);
        return true;
    }
}
