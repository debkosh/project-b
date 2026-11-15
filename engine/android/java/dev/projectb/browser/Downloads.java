package dev.projectb.browser;

import android.app.DownloadManager;
import android.content.ContentResolver;
import android.content.ContentValues;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.net.Uri;
import android.os.Environment;
import android.provider.MediaStore;
import android.util.Log;
import android.webkit.MimeTypeMap;

import org.chromium.base.ContextUtils;
import org.jni_zero.CalledByNative;
import org.jni_zero.JNINamespace;

import java.io.File;
import java.io.FileInputStream;
import java.io.InputStream;
import java.io.OutputStream;
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

    @CalledByNative
    static void publish(String path, String mime) {
        sCopier.execute(() -> {
            File file = new File(path);
            ContentResolver resolver = ContextUtils.getApplicationContext().getContentResolver();
            ContentValues values = new ContentValues();
            values.put(MediaStore.Downloads.DISPLAY_NAME, file.getName());
            values.put(MediaStore.Downloads.MIME_TYPE, mimeOf(path, mime));
            values.put(MediaStore.Downloads.RELATIVE_PATH, Environment.DIRECTORY_DOWNLOADS);
            values.put(MediaStore.Downloads.IS_PENDING, 1);
            Uri uri = resolver.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, values);
            if (uri == null) {
                Log.w(TAG, "no room in the shared Downloads for " + file.getName());
                return;
            }
            try (InputStream in = new FileInputStream(file); OutputStream out = resolver.openOutputStream(uri)) {
                byte[] buf = new byte[64 * 1024];
                for (int n; (n = in.read(buf)) > 0; ) out.write(buf, 0, n);
            } catch (Exception e) {
                Log.w(TAG, "could not copy " + file.getName() + " into the shared Downloads", e);
                resolver.delete(uri, null, null);
                return;
            }
            values.clear();
            values.put(MediaStore.Downloads.IS_PENDING, 0);
            resolver.update(uri, values, null, null);
            published().edit().putString(path, uri.toString()).apply();
            file.delete();
        });
    }

    @CalledByNative
    static void open(String path) {
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
    // Opens the shared copy, once the copy publish started is made: Open right after a
    // download ends would otherwise find nothing yet and do nothing.
        try {
            context.startActivity(Intent.createChooser(intent, null).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK));
        sCopier.execute(() -> openNow(path));
    }

    private static void openNow(String path) {
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
}
