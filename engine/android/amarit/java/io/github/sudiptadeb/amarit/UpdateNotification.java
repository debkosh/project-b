package io.github.sudiptadeb.amarit;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;

import org.json.JSONException;
import org.json.JSONObject;

/**
 * The system's notification for an app updater that runs elsewhere, such as
 * the Go updater amarit/appupdate: the app hands it each State, as the JSON
 * the updater makes, and gets the person's taps back as commands.
 *
 * <pre>
 * UpdateNotification.commands = name -> updater.send(name);
 * updater.onChange(json -> UpdateNotification.show(context, json));
 * </pre>
 *
 * Downloading shows the progress with Cancel; verifying, an indeterminate
 * bar; ready, the version with Install; a download that failed in a way a
 * retry can help, Try again. Every other state takes the notification away.
 *
 * Plain Java with platform APIs only, so a build without Gradle takes it as
 * source: this file, res/values/amarit_notification.xml, and the receiver in
 * the manifest, {@code <receiver android:name="io.github.sudiptadeb.amarit.UpdateNotification$Receiver"
 * android:exported="false" />}. The words are those string resources, which
 * an app overrides. The app asks for POST_NOTIFICATIONS on Android 13 and later.
 */
public final class UpdateNotification {
    private UpdateNotification() {}

    /** The person's taps: "cancel", "retry" or "relaunch". */
    public interface Commands {
        void command(String name);
    }

    /** Where taps go. A tap while the app's process is gone opens the app. */
    public static volatile Commands commands;

    public static final String CHANNEL = "amarit_progress";
    public static final int ID = 0x414E;
    static final String EXTRA = "io.github.sudiptadeb.amarit.COMMAND";

    /** What the notification says for one state. */
    public static final class Shown {
        public final int title;
        public final int text;
        public final long done, total;
        public final boolean indeterminate, ongoing;
        public final String command;
        public final int commandLabel;

        Shown(int title, int text, long done, long total, boolean indeterminate, boolean ongoing, String command, int commandLabel) {
            this.title = title;
            this.text = text;
            this.done = done;
            this.total = total;
            this.indeterminate = indeterminate;
            this.ongoing = ongoing;
            this.command = command;
            this.commandLabel = commandLabel;
        }
    }

    /** The table in amarit's docs/appupdate.md, for a notification; null takes it away. */
    public static Shown shown(JSONObject state) {
        long total = state.optLong("total");
        switch (state.optString("phase")) {
            case "downloading":
                return new Shown(R.string.amarit_notification_downloading, 0, state.optLong("done"), total,
                        total <= 0, true, "cancel", R.string.amarit_notification_cancel);
            case "verifying":
                return new Shown(R.string.amarit_notification_verifying, 0, 0, 0, true, true, "cancel", R.string.amarit_notification_cancel);
            case "ready":
                return new Shown(R.string.amarit_notification_ready, R.string.amarit_notification_ready_text, 0, 0, false, false,
                        "relaunch", R.string.amarit_notification_install);
            case "failed":
                switch (state.optString("kind")) {
                    case "network":
                    case "stalled":
                    case "damaged":
                    case "unchecked":
                    case "install":
                        return new Shown(R.string.amarit_notification_failed, R.string.amarit_notification_failed_text, 0, 0, false, false,
                                "retry", R.string.amarit_notification_retry);
                    default:
                        return null;
                }
            default:
                return null;
        }
    }

    /** Shows stateJson, appupdate's State, as the notification. */
    public static void show(Context context, String stateJson) {
        NotificationManager nm = context.getSystemService(NotificationManager.class);
        if (nm == null) return;
        Shown s;
        String version;
        try {
            JSONObject state = new JSONObject(stateJson);
            s = shown(state);
            version = state.optString("version");
        } catch (JSONException e) {
            return;
        }
        if (s == null) {
            nm.cancel(ID);
            return;
        }
        nm.createNotificationChannel(new NotificationChannel(CHANNEL,
                context.getString(R.string.amarit_notification_channel), NotificationManager.IMPORTANCE_LOW));
        String label = context.getApplicationInfo().loadLabel(context.getPackageManager()).toString();
        Notification.Builder b = new Notification.Builder(context, CHANNEL)
                .setSmallIcon(s.ongoing ? android.R.drawable.stat_sys_download : android.R.drawable.stat_sys_download_done)
                .setContentTitle(context.getString(s.title, label, version))
                .setOngoing(s.ongoing)
                .setOnlyAlertOnce(true)
                .setAutoCancel(!s.ongoing);
        Intent launch = context.getPackageManager().getLaunchIntentForPackage(context.getPackageName());
        if (launch != null) {
            b.setContentIntent(PendingIntent.getActivity(context, 0, launch, PendingIntent.FLAG_IMMUTABLE));
        }
        if (s.text != 0) b.setContentText(context.getString(s.text, label, version));
        if (s.ongoing) {
            int max = s.total > 0 ? 1000 : 0;
            int at = s.total > 0 ? (int) (s.done * 1000 / s.total) : 0;
            b.setProgress(max, at, s.indeterminate);
        }
        if (s.command != null) {
            Intent tap = new Intent(context, Receiver.class).putExtra(EXTRA, s.command);
            PendingIntent pi = PendingIntent.getBroadcast(context, s.command.hashCode(), tap,
                    PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
            b.addAction(new Notification.Action.Builder(null, context.getString(s.commandLabel), pi).build());
        }
        nm.notify(ID, b.build());
    }

    /** Takes a notification's tap to commands, or opens the app when it has none. */
    public static final class Receiver extends BroadcastReceiver {
        @Override
        public void onReceive(Context context, Intent intent) {
            String cmd = intent.getStringExtra(EXTRA);
            if (cmd == null) return;
            Commands to = commands;
            if (to != null) {
                to.command(cmd);
                return;
            }
            Intent launch = context.getPackageManager().getLaunchIntentForPackage(context.getPackageName());
            if (launch != null) {
                launch.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
                context.startActivity(launch);
            }
        }
    }
}
