package dev.projectb.browser;

import android.content.Context;
import android.net.Uri;

import androidx.core.content.FileProvider;

import org.chromium.base.Callback;
import org.chromium.base.PathUtils;
import org.chromium.base.task.AsyncTask;
import org.chromium.ui.base.Clipboard;

import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;

/**
 * Where Chromium's clipboard keeps the images it copies, as Chrome's own provider does: in the
 * profile's shared folder, which the FileProvider serves (res/xml/updates.xml). Chromium reads
 * an image of this app back from the clipboard only when it is the one it recorded copying
 * here, so without this a page's own image copy was dropped, and the long-press menu's Copy
 * image pasted into our pages empty.
 */
final class ClipboardImages implements Clipboard.ImageFileProvider {
    private final Context context;
    private Clipboard.ClipboardUriMetadata last;

    ClipboardImages(Context context) {
        this.context = context.getApplicationContext();
    }

    /** An image file of ours as other apps read it. */
    Uri uriOf(File file) {
        return FileProvider.getUriForFile(context, context.getPackageName() + ".updates", file);
    }

    @Override
    public void storeImageAndGenerateUri(byte[] data, String extension, Callback<Uri> callback) {
        new AsyncTask<Uri>() {
            @Override
            protected Uri doInBackground() {
                File folder = new File(PathUtils.getDataDirectory(), "profile/shared");
                File file = new File(folder, "clipboard." + extension);
                folder.mkdirs();
                try (FileOutputStream out = new FileOutputStream(file)) {
                    out.write(data);
                } catch (IOException e) {
                    return null;
                }
                return uriOf(file);
            }

            @Override
            protected void onPostExecute(Uri uri) {
                if (uri != null) callback.onResult(uri);
            }
        }.executeOnExecutor(AsyncTask.THREAD_POOL_EXECUTOR);
    }

    @Override
    public void storeLastCopiedImageMetadata(Clipboard.ClipboardUriMetadata metadata) {
        last = metadata;
    }

    @Override
    public Clipboard.ClipboardUriMetadata getLastCopiedImageMetadata() {
        return last;
    }

    @Override
    public void clearLastCopiedImageMetadata() {
        last = null;
    }
}
