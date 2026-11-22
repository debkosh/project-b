package dev.projectb.browser;

import android.app.Activity;
import android.content.Intent;
import android.content.res.Configuration;
import android.os.Bundle;

import io.github.sudiptadeb.oskit.OsKit;

import org.chromium.base.CommandLine;
import org.chromium.base.PathUtils;
import org.chromium.base.library_loader.LibraryLoader;
import org.chromium.base.library_loader.LibraryProcessType;
import org.chromium.content_public.browser.BrowserStartupController;
import org.chromium.ui.base.ActivityWindowAndroid;
import org.chromium.ui.base.IntentRequestTracker;

/** The main process's one window. Go's managers live exactly as long as it does. */
public final class BrowserActivity extends Activity {
    private BrowserWindow window;
    // Hands the window the answers of what it asked other apps for, such as a file picked.
    private IntentRequestTracker intentRequests;
    private ActivityWindowAndroid windowAndroid;
    // Go listens: until then a tap on a download's notification waits here, by Go's id for it.
    private boolean started;
    private long pendingDownload = -1;

    @Override
    protected void onCreate(Bundle saved) {
        super.onCreate(saved);
        if (!CommandLine.isInitialized()) {
            CommandLine.init(null);
        }
        LibraryLoader.getInstance().ensureInitialized();
        // The Go core starts, then content, as main does on the Mac. Go's profile is the one
        // folder Android gives the app, named outright as the desktop's --user-data-dir does.
        // One process holds it, so the claim is always this copy's and Go never says quit:
        // added is never null here.
        GoSystem.attach(this);
        OsKit.attach(this);
        // A page's passkey requests are ours to answer (docs/passkeys.md).
        Passkeys.register();
        String data = PathUtils.getDataDirectory();
        String[] added =
                GoCore.init(
                        new String[] {
                            "project_b", "--root=" + data, "--user-data-dir=" + data + "/profile"
                        });
        CommandLine.getInstance().appendSwitchesAndArguments(added);

        intentRequests = IntentRequestTracker.createFromActivity(this);
        windowAndroid =
                new ActivityWindowAndroid(
                        this,
                        /* listenToActivityState= */ true,
                        intentRequests,
                        /* insetObserver= */ null,
                        /* occlusionTrackingAllowed= */ true);
        window = new BrowserWindow(this, windowAndroid);
        setContentView(window);

        // The same list the Mac app takes as --block, and its --skip-welcome.
        String blocked = getIntent().getStringExtra("block");
        boolean skipWelcome = getIntent().getBooleanExtra("skip-welcome", false);
        openDownload(getIntent());
        long goTable = GoCore.table();
        // A link from another app or a tap on a page's notification that started the app:
        // oskit keeps it for Go, which takes it once it starts.
        OsKit.onIntent(getIntent());
        BrowserStartupController.getInstance()
                .startBrowserProcessesAsync(
                        LibraryProcessType.PROCESS_BROWSER,
                        /* startGpuProcess= */ true,
                        /* startMinimalBrowser= */ false,
                        /* singleProcess= */ false,
                        new BrowserStartupController.StartupCallback() {
                            @Override
                            public void onSuccess(
                                    BrowserStartupController.StartupMetrics metrics) {
                                window.start(goTable, blocked == null ? "" : blocked, skipWelcome);
                                // What 0.3.9 and before kept of each download's shared copy,
                                // to Go once, now that it listens.
                                Downloads.reportKept();
                                started = true;
                                if (pendingDownload >= 0) {
                                    Downloads.opened(pendingDownload);
                                    pendingDownload = -1;
                                }
                            }

                            @Override
                            public void onFailure() {
                                finish();
                            }
                        });
    }

    // A link from another app while the app runs: the activity is singleTask, so Android
    // brings it forward with the link here.
    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        openDownload(intent);
        OsKit.onIntent(intent);
    }

    // A tap on a finished download's notification: Go opens it as the list's Open does.
    private void openDownload(Intent intent) {
        if (!Downloads.OPEN.equals(intent.getAction())) {
            return;
        }
        long id = intent.getLongExtra(Downloads.OPEN, -1);
        if (started) {
            Downloads.opened(id);
        } else {
            pendingDownload = id;
        }
    }

    @Override
    protected void onStart() {
        super.onStart();
        window.setActivityVisible(true);
    }

    @Override
    protected void onStop() {
        super.onStop();
        window.setActivityVisible(false);
    }

    // Android's sheet asking to be the default browser has been answered, or another app
    // the window asked, such as the file picker for a page's file input.
    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        intentRequests.onActivityResult(requestCode, resultCode, data);
        OsKit.onActivityResult(requestCode);
    }

    // Android's question about notifications has been answered, or one the window asked, such
    // as the camera a page's file input asks for before the picker opens.
    @Override
    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] results) {
        super.onRequestPermissionsResult(requestCode, permissions, results);
        windowAndroid.handlePermissionResult(requestCode, permissions, results);
        OsKit.onRequestPermissionsResult(requestCode);
    }

    // The activity handles uiMode itself, so a theme that follows the system follows it here.
    @Override
    public void onConfigurationChanged(Configuration config) {
        super.onConfigurationChanged(config);
        window.applyTheme();
    }

    @Override
    public void onBackPressed() {
        if (!window.backKey()) {
            moveTaskToBack(true);
        }
    }

    // Go's state can't outlive the views it describes, so the process ends with the window.
    @Override
    protected void onDestroy() {
        window.stop();
        super.onDestroy();
        System.exit(0);
    }
}
