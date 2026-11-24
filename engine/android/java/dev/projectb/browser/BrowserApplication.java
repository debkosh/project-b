package dev.projectb.browser;

import android.app.Application;
import android.content.Context;

import org.chromium.base.ApplicationStatus;
import org.chromium.base.ContextUtils;
import org.chromium.base.PathUtils;
import org.chromium.base.library_loader.LibraryLoader;
import org.chromium.base.library_loader.LibraryProcessType;
import org.chromium.net.NetworkChangeNotifier;
import org.chromium.ui.accessibility.AccessibilityState;

/** Runs first in every process, the main one and each child service. A child's name has a colon. */
public final class BrowserApplication extends Application {
    private static boolean isMainProcess() {
        return !ContextUtils.getProcessName().contains(":");
    }

    @Override
    protected void attachBaseContext(Context base) {
        super.attachBaseContext(base);
        ContextUtils.initApplicationContext(this);
        LibraryLoader.getInstance()
                .setLibraryProcessType(
                        isMainProcess()
                                ? LibraryProcessType.PROCESS_BROWSER
                                : LibraryProcessType.PROCESS_CHILD);
        if (isMainProcess()) {
            PathUtils.setPrivateDataDirectorySuffix("project_b");
            ApplicationStatus.initialize(this);
        }
    }

    @Override
    public void onCreate() {
        super.onCreate();
        if (isMainProcess()) {
            NetworkChangeNotifier.init();
            NetworkChangeNotifier.setAutoDetectConnectivityState(true);
            // Chromium learns which accessibility services run, a screen reader such as
            // TalkBack among them, once at its first question unless the embedder registers
            // these first, as Chrome does at start. Without them pages never get the screen
            // reader's mode, and a switch or a checkbox reaches TalkBack without its checked
            // state (docs/fragility.md, "TalkBack heard every switch as off").
            AccessibilityState.registerObservers();
        }
    }
}
