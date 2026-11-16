package dev.projectb.browser;

import android.app.Activity;
import android.content.Context;
import android.os.Bundle;
import android.os.CancellationSignal;
import android.os.ParcelFileDescriptor;
import android.print.PageRange;
import android.print.PrintAttributes;
import android.print.PrintDocumentAdapter;
import android.print.PrintDocumentInfo;
import android.print.PrintManager;
import android.util.Log;

import org.chromium.base.ApplicationStatus;
import org.jni_zero.CalledByNative;
import org.jni_zero.JNINamespace;

import java.io.FileOutputStream;

/**
 * A page printed (docs/daily-driver.md, Printing): the PDF Chromium made of it, handed to
 * Android's print sheet, which has its own preview, the printers and Save as PDF.
 */
@JNINamespace("project_b")
final class Printing {
    private static final String TAG = "pb.printing";

    private Printing() {}

    @CalledByNative
    static void print(String title, byte[] pdf) {
        Activity activity = ApplicationStatus.getLastTrackedFocusedActivity();
        if (activity == null) {
            Log.w(TAG, "no activity to print from");
            return;
        }
        String name = title == null || title.isEmpty() ? "Page" : title;
        PrintManager printer = (PrintManager) activity.getSystemService(Context.PRINT_SERVICE);
        printer.print(name, new Adapter(name, pdf), null);
    }

    /** Hands the sheet the one PDF, whatever paper it chose: it fits the pages to it. */
    private static final class Adapter extends PrintDocumentAdapter {
        private final String mName;
        private final byte[] mPdf;

        Adapter(String name, byte[] pdf) {
            mName = name;
            mPdf = pdf;
        }

        @Override
        public void onLayout(PrintAttributes oldAttributes, PrintAttributes newAttributes,
                CancellationSignal cancel, LayoutResultCallback callback, Bundle extras) {
            if (cancel.isCanceled()) {
                callback.onLayoutCancelled();
                return;
            }
            PrintDocumentInfo info = new PrintDocumentInfo.Builder(mName + ".pdf")
                    .setContentType(PrintDocumentInfo.CONTENT_TYPE_DOCUMENT)
                    .build();
            callback.onLayoutFinished(info, oldAttributes == null);
        }

        @Override
        public void onWrite(PageRange[] pages, ParcelFileDescriptor destination,
                CancellationSignal cancel, WriteResultCallback callback) {
            try (FileOutputStream out = new FileOutputStream(destination.getFileDescriptor())) {
                out.write(mPdf);
                callback.onWriteFinished(new PageRange[] {PageRange.ALL_PAGES});
            } catch (Exception e) {
                Log.w(TAG, "could not hand the page to the print sheet", e);
                callback.onWriteFailed(e.getMessage());
            }
        }
    }
}
