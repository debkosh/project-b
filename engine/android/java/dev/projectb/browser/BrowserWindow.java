package dev.projectb.browser;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.content.res.Configuration;
import android.graphics.PixelFormat;
import android.graphics.Rect;
import android.graphics.drawable.ColorDrawable;
import android.net.Uri;
import android.os.Process;
import android.os.SystemClock;
import android.view.ActionMode;
import android.view.InputDevice;
import android.view.Menu;
import android.view.MenuItem;
import android.view.MotionEvent;
import android.view.PointerIcon;
import android.view.SurfaceHolder;
import android.view.SurfaceView;
import android.view.ViewGroup;
import android.view.View;
import android.widget.FrameLayout;

import androidx.core.content.FileProvider;
import androidx.core.view.ViewCompat;
import androidx.core.view.WindowInsetsCompat;

import java.io.File;
import java.net.URLConnection;
import java.util.HashMap;
import java.util.Map;

import org.jni_zero.CalledByNative;
import org.jni_zero.JNINamespace;
import org.jni_zero.NativeMethods;

import org.chromium.components.embedder_support.view.ContentView;
import org.chromium.content_public.browser.ActionModeCallback;
import org.chromium.content_public.browser.ActionModeCallbackHelper;
import org.chromium.content_public.browser.SelectionMenuItem;
import org.chromium.content_public.browser.SelectionPopupController;
import org.chromium.content_public.browser.Visibility;
import org.chromium.content_public.browser.WebContents;
import org.chromium.ui.base.Clipboard;
import org.chromium.ui.base.ViewAndroidDelegate;
import org.chromium.ui.base.WindowAndroid;
import org.chromium.ui.native_theme.OsSettingsProviderAndroidBridge;

/**
 * The window. One SurfaceView at the bottom, where the native compositor draws the shown page and the
 * shell, and above it a ContentView per tab and one for the shell, which draw nothing and only take
 * touches and the keyboard. Each ContentView sits where its page is drawn, so a touch reaches the
 * right WebContents with the right coordinates. Only the shown tab's view is visible.
 */
@JNINamespace("project_b")
final class BrowserWindow extends FrameLayout {
    private static final double AREA_PAGE = 0;

    private final Activity activity;
    private final WindowAndroid windowAndroid;
    private final float density;

    private long nativeWindow;
    // Whether the soft keyboard is up, as the shell last heard.
    private boolean keyboardUp;
    // Where the native compositor draws.
    private SurfaceView surface;
    // The view the shell is in, which refuses a touch the layout says is the page's.
    private ShellHolder shellHolder;
    // The shell: a page's ContentView, or a native shell drawn by Android's own views
    // (docs/native-shell.md). One of the two.
    private ContentView shellView;
    private WebContents shellContents;
    private NativeShell nativeShell;
    private final Map<WebContents, ContentView> pages = new HashMap<>();
    private WebContents shownContents;
    private ContentView shownView;
    // Go's theme for the window's own parts: 0 as the system is, 1 light, 2 dark.
    private int theme;
    // The frame colour for light and for dark, 0xRRGGBB.
    private int frameLight;
    private int frameDark;
    private int pageLight;
    private int pageDark;
    private boolean hasFrame;
    private FrameLayout.LayoutParams pageParams = matchParent();
    // Five numbers a region, x, y, w, h and what a press there does, in order: the first that
    // holds a point decides. None holds it: the page.
    private double[] regions = new double[0];
    // Where Chromium's clipboard keeps the images it copies.
    private final ClipboardImages images;

    BrowserWindow(Activity activity, WindowAndroid windowAndroid) {
        super(activity);
        this.activity = activity;
        this.windowAndroid = windowAndroid;
        density = activity.getResources().getDisplayMetrics().density;
        images = new ClipboardImages(activity);
        // A second finger on the bar during a pinch on the page must stay with the page.
        setMotionEventSplittingEnabled(false);
    }

    /**
     * The soft keyboard coming up or going, told to the shell as {"t":"keyboard","on":...}, as
     * though it said so itself, and Go hands it back to the shell: while it is up, the bars under
     * the page step aside (docs/design-system.md, On a phone). Read from the root's insets each
     * time the window lays out, as it does when the keyboard comes or goes: the insets the
     * window itself is handed have had the keyboard taken out by then on Android 17, and on
     * Android 10 androidx works it out from the root's.
     */
    private void keyboardChanged() {
        WindowInsetsCompat root = ViewCompat.getRootWindowInsets(this);
        boolean up = root != null && root.isVisible(WindowInsetsCompat.Type.ime());
        if (up != keyboardUp && nativeWindow != 0) {
            keyboardUp = up;
            BrowserWindowJni.get().shellMessage(nativeWindow, "{\"t\":\"keyboard\",\"on\":" + up + "}");
        }
    }

    @Override
    protected void onLayout(boolean changed, int left, int top, int right, int bottom) {
        super.onLayout(changed, left, top, right, bottom);
        keyboardChanged();
    }

    /** Content is ready: make the surface, then the native side, which starts the app and Go. */
    void start(long goTable, String blockedHosts, boolean skipWelcome) {
        Clipboard.getInstance().setImageFileProvider(images);
        // How old the process is, for the timeline's time zero.
        long processAgeMs = SystemClock.elapsedRealtime() - Process.getStartElapsedRealtime();
        nativeWindow =
                BrowserWindowJni.get()
                        .start(this, windowAndroid, goTable, processAgeMs, blockedHosts,
                                skipWelcome);
        // Go's first layout and theme arrive inside start, before there is a window to tell,
        // so the frame and our pages' colour are painted now; else they waited for the next
        // change, and a phone started in dark kept the light frame.
        paintFrame();
        surface = new SurfaceView(getContext());
        surface.setZOrderMediaOverlay(true);
        // A SurfaceView defaults to 16 bits per pixel.
        surface.getHolder().setFormat(PixelFormat.OPAQUE);
        surface.getHolder()
                .addCallback(
                        new SurfaceHolder.Callback() {
                            @Override
                            public void surfaceCreated(SurfaceHolder holder) {
                                BrowserWindowJni.get().surfaceCreated(nativeWindow);
                            }

                            @Override
                            public void surfaceChanged(
                                    SurfaceHolder holder, int format, int width, int height) {
                                BrowserWindowJni.get()
                                        .surfaceChanged(
                                                nativeWindow, format, width, height, holder.getSurface());
                            }

                            @Override
                            public void surfaceDestroyed(SurfaceHolder holder) {
                                BrowserWindowJni.get().surfaceDestroyed(nativeWindow);
                            }
                        });
        addView(surface, 0, matchParent());
        // Laid out before content was ready, so the size couldn't be told then.
        if (getWidth() > 0) {
            BrowserWindowJni.get().sizeChanged(nativeWindow, getWidth() / density, getHeight() / density);
        }
    }

    /** Hidden pages are told so, as they are when a tab is hidden. */
    void setActivityVisible(boolean visible) {
        int visibility = visible ? Visibility.VISIBLE : Visibility.HIDDEN;
        if (shellContents != null) {
            shellContents.updateWebContentsVisibility(visibility);
        }
        if (shownContents != null) {
            shownContents.updateWebContentsVisibility(visibility);
        }
    }

    @Override
    protected void onSizeChanged(int w, int h, int oldW, int oldH) {
        super.onSizeChanged(w, h, oldW, oldH);
        if (nativeWindow != 0) {
            BrowserWindowJni.get().sizeChanged(nativeWindow, w / density, h / density);
        }
    }

    @Override
    protected void onWindowVisibilityChanged(int visibility) {
        super.onWindowVisibilityChanged(visibility);
        windowAndroid.onVisibilityChanged(visibility == VISIBLE);
    }

    boolean backKey() {
        return nativeWindow != 0 && BrowserWindowJni.get().backKey(nativeWindow);
    }

    /** Whether the shell has something open that the back key closes. */
    boolean shellOpen() {
        return nativeWindow != 0 && BrowserWindowJni.get().shellOpen(nativeWindow);
    }

    void stop() {
        if (nativeWindow != 0) {
            BrowserWindowJni.get().stop(nativeWindow);
        }
    }

    // The keyboard goes where the finger went. Each view takes focus for itself.
    @Override
    public boolean dispatchTouchEvent(MotionEvent e) {
        if (e.getActionMasked() == MotionEvent.ACTION_DOWN && shellHolder != null) {
            if (inShell(e.getX(), e.getY())) {
                // A native shell's own views take the keyboard as they are touched.
                if (shellView != null) {
                    shellView.requestFocus();
                }
            } else if (shownView != null) {
                shownView.requestFocus();
            }
        }
        return super.dispatchTouchEvent(e);
    }

    /**
     * Holds the shell's view. The same question the Mac window's hitTest: asks: Android offers a
     * finger down to the views from the top, and refusing here hands it to the page below.
     */
    private final class ShellHolder extends FrameLayout {
        ShellHolder(Context context) {
            super(context);
        }

        // The shell's view was told the pointer came in, and not yet that it left.
        private boolean hovering;

        @Override
        public boolean dispatchTouchEvent(MotionEvent e) {
            if (e.getActionMasked() == MotionEvent.ACTION_DOWN && !inShell(e.getX(), e.getY())) {
                return false;
            }
            return super.dispatchTouchEvent(e);
        }

        /**
         * A mouse's hover and wheel go by the same regions a finger does
         * (docs/large-screens.md): over the shell to the shell, and the rest to the page below,
         * which Android tries next when this refuses. The shell's view stays under the pointer
         * all over the window, so Android never tells it the pointer left: this does, as it
         * crosses onto the page, or a shell button would stay lit.
         */
        @Override
        public boolean dispatchGenericMotionEvent(MotionEvent e) {
            if (!e.isFromSource(InputDevice.SOURCE_CLASS_POINTER)) {
                return super.dispatchGenericMotionEvent(e);
            }
            boolean shell = inShell(e.getX(), e.getY());
            switch (e.getActionMasked()) {
                case MotionEvent.ACTION_HOVER_ENTER:
                case MotionEvent.ACTION_HOVER_MOVE:
                    if (!shell) {
                        hover(e, MotionEvent.ACTION_HOVER_EXIT, false);
                        return false;
                    }
                    if (e.getActionMasked() == MotionEvent.ACTION_HOVER_ENTER) {
                        hovering = true;
                    } else {
                        hover(e, MotionEvent.ACTION_HOVER_ENTER, true);
                    }
                    super.dispatchGenericMotionEvent(e);
                    return true;
                case MotionEvent.ACTION_HOVER_EXIT:
                    hover(e, MotionEvent.ACTION_HOVER_EXIT, false);
                    return shell;
                default:
                    // The wheel over the bar is the bar's, even where nothing there scrolls.
                    if (!shell) {
                        return false;
                    }
                    super.dispatchGenericMotionEvent(e);
                    return true;
            }
        }

        // Tells the shell's view the pointer came in or left, once each.
        private void hover(MotionEvent e, int action, boolean in) {
            if (hovering == in) {
                return;
            }
            hovering = in;
            MotionEvent told = MotionEvent.obtain(e);
            told.setAction(action);
            super.dispatchGenericMotionEvent(told);
            told.recycle();
        }

        // The pointer's shape over the page is the page's, as Android asks the next view.
        @Override
        public PointerIcon onResolvePointerIcon(MotionEvent e, int pointerIndex) {
            if (!inShell(e.getX(pointerIndex), e.getY(pointerIndex))) {
                return null;
            }
            return super.onResolvePointerIcon(e, pointerIndex);
        }
    }

    // Nothing drags a phone's window, so whatever isn't the page is the shell.
    private boolean inShell(float pxX, float pxY) {
        double x = pxX / density;
        double y = pxY / density;
        for (int i = 0; i + 4 < regions.length; i += 5) {
            if (x >= regions[i] && x < regions[i] + regions[i + 2]
                    && y >= regions[i + 1] && y < regions[i + 1] + regions[i + 3]) {
                return regions[i + 4] != AREA_PAGE;
            }
        }
        return false;
    }

    /**
     * After the GPU process died: Android destroys a hidden SurfaceView's surface and makes
     * another when it shows again, and the compositor takes that one in surfaceChanged.
     */
    @CalledByNative
    private void recreateSurface() {
        surface.setVisibility(View.GONE);
        surface.setVisibility(View.VISIBLE);
    }

    @CalledByNative
    private void setShell(WebContents contents) {
        shellHolder = new ShellHolder(getContext());
        shellContents = contents;
        shellView = attach(contents, shellHolder, matchParent());
        addShellHolder();
        contents.updateWebContentsVisibility(Visibility.VISIBLE);
    }

    /** The native shell called id, in the shell's place over the pages. */
    @CalledByNative
    private void setNativeShell(String id) {
        shellHolder = new ShellHolder(getContext());
        nativeShell =
                NativeShells.make(
                        id,
                        getContext(),
                        json -> BrowserWindowJni.get().shellMessage(nativeWindow, json));
        shellHolder.addView(nativeShell.getView(), matchParent());
        addShellHolder();
    }

    // Over the pages.
    private void addShellHolder() {
        addView(shellHolder, matchParent());
    }

    /** What Go sends the native shell, as a shell's page hears it through __shellReceive. */
    @CalledByNative
    private void receiveShell(String json) {
        nativeShell.receive(json);
    }

    /** The keyboard for the native shell: its address field. */
    @CalledByNative
    private void focusNativeShell() {
        nativeShell.focus();
    }

    /** The shell leaves, a page's or the native one, before another shell comes. */
    @CalledByNative
    private void removeShell() {
        removeView(shellHolder);
        shellHolder = null;
        shellView = null;
        shellContents = null;
        nativeShell = null;
    }

    /** A new tab's view, hidden until showPage. The engine tells its page it's hidden. */
    @CalledByNative
    private void addPage(WebContents contents) {
        ContentView view = attach(contents, this, copy(pageParams));
        view.setVisibility(View.INVISIBLE);
        pages.put(contents, view);
    }

    @CalledByNative
    private void showPage(WebContents contents) {
        if (shownView != null) {
            shownView.setVisibility(View.INVISIBLE);
        }
        shownContents = contents;
        shownView = pages.get(contents);
        shownView.setVisibility(View.VISIBLE);
    }

    @CalledByNative
    private void removePage(WebContents contents) {
        ContentView view = pages.remove(contents);
        removeView(view);
        if (contents.equals(shownContents)) {
            shownContents = null;
            shownView = null;
        }
    }

    // At the bottom: pages go under the shell, and the surface under them all.
    private ContentView attach(WebContents contents, ViewGroup parent, FrameLayout.LayoutParams params) {
        ContentView view = ContentView.createContentView(getContext(), contents);
        contents.setDelegates(
                "",
                ViewAndroidDelegate.createBasicDelegate(view),
                view,
                windowAndroid,
                WebContents.createDefaultInternalsHolder());
        // A page's text selection: the handles, and Android's floating Copy, Share and Select
        // all, as content_shell sets it up. Without a controller, the first handle a long press
        // on text makes has nothing to draw it, and the browser crashes (a null drawable in
        // ui::TouchHandle's constructor).
        SelectionPopupController selection = SelectionPopupController.fromWebContents(contents);
        selection.setActionModeCallback(defaultActionModeCallback(selection.getActionModeCallbackHelper()));
        view.addOnLayoutChangeListener(
                (v, left, top, right, bottom, oldLeft, oldTop, oldRight, oldBottom) -> {
                    contents.setSize(right - left, bottom - top);
                    BrowserWindowJni.get()
                            .contentsMoved(nativeWindow, contents, left, top, right - left, bottom - top);
                });
        parent.addView(view, 0, params);
        return view;
    }

    // Android's own text selection menu for a page, as content_shell has it: every answer is
    // the helper's.
    private static ActionModeCallback defaultActionModeCallback(ActionModeCallbackHelper helper) {
        return new ActionModeCallback() {
            @Override
            public boolean onCreateActionMode(ActionMode mode, Menu menu) {
                helper.onCreateActionMode(mode, menu);
                return true;
            }

            @Override
            public boolean onPrepareActionMode(ActionMode mode, Menu menu) {
                return helper.onPrepareActionMode(mode, menu);
            }

            @Override
            public boolean onActionItemClicked(ActionMode mode, MenuItem item) {
                return helper.onActionItemClicked(mode, item);
            }

            @Override
            public boolean onDropdownItemClicked(SelectionMenuItem item, boolean closeMenu) {
                return helper.onDropdownItemClicked(item, closeMenu);
            }

            @Override
            public void onDestroyActionMode(ActionMode mode) {
                helper.onDestroyActionMode();
            }

            @Override
            public void onGetContentRect(ActionMode mode, View view, Rect outRect) {
                helper.onGetContentRect(mode, view, outRect);
            }
        };
    }

    @CalledByNative
    private void setLayout(
            double viewW, double viewH, double x, double y, double w, double h, double[] regions) {
        this.regions = regions;
        // Measured for a size the window no longer has. A fresher report follows.
        if (Math.abs(viewW - getWidth() / density) > 1 || Math.abs(viewH - getHeight() / density) > 1) {
            return;
        }
        // Margins, not a size, so the pages follow the window in the same frame when the keyboard
        // opens or the phone turns.
        pageParams = matchParent();
        pageParams.leftMargin = px(x);
        pageParams.topMargin = px(y);
        pageParams.rightMargin = px(viewW - x - w);
        pageParams.bottomMargin = px(viewH - y - h);
        for (ContentView view : pages.values()) {
            view.setLayoutParams(copy(pageParams));
        }
    }

    /**
     * The frame colour, light and dark, which differ only while no colour is picked: the one for
     * the theme shows in the status bar and wherever a camera cutout keeps the window away. Our
     * pages' background, light and dark: the one for the theme shows where nothing draws, so a
     * page of ours opens on its own colour.
     */
    @CalledByNative
    private void setFrameColor(int light, int dark, int page, int pageDarkRgb) {
        frameLight = light;
        frameDark = dark;
        pageLight = page;
        pageDark = pageDarkRgb;
        hasFrame = true;
        paintFrame();
    }

    private void paintFrame() {
        if (!hasFrame || nativeWindow == 0) return;
        int rgb = isDark() ? frameDark : frameLight;
        int color = 0xff000000 | rgb;
        activity.getWindow().setBackgroundDrawable(new ColorDrawable(color));
        activity.getWindow().setStatusBarColor(color);
        BrowserWindowJni.get().pageColorShown(nativeWindow, isDark() ? pageDark : pageLight);
    }

    // As Go's theme says, or the phone's own dark mode as the system is.
    private boolean isDark() {
        return theme == 2
                || (theme == 0
                        && (getResources().getConfiguration().uiMode
                                        & Configuration.UI_MODE_NIGHT_MASK)
                                == Configuration.UI_MODE_NIGHT_YES);
    }

    /**
     * The window's own parts follow Go's theme, as the Mac's title bar does: here that's the
     * navigation bar, white or the dark theme's near-black. The status bar is the frame colour.
     */
    @CalledByNative
    private void setTheme(int theme) {
        this.theme = theme;
        applyTheme();
    }

    /**
     * As the system is, the phone's own dark mode decides, now and when it changes. The pages,
     * ours and the web's, hear it as their preferred colour scheme.
     */
    void applyTheme() {
        boolean dark = isDark();
        paintFrame();
        OsSettingsProviderAndroidBridge.setPreferredColorScheme(dark);
        activity.getWindow().setNavigationBarColor(dark ? 0xff1b1c1e : 0xffffffff);
        int flags = getSystemUiVisibility();
        setSystemUiVisibility(
                dark
                        ? flags & ~View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR
                        : flags | View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR);
    }

    // A page in full screen: the status and navigation bars go, and a swipe from an edge
    // shows them for a moment.
    @CalledByNative
    private void setSystemBarsHidden(boolean hidden) {
        android.view.Window window = activity.getWindow();
        // Into the camera's notch too while hidden, or the window keeps the status bar's room
        // there as a strip in the frame colour.
        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.P) {
            android.view.WindowManager.LayoutParams params = window.getAttributes();
            params.layoutInDisplayCutoutMode = hidden
                    ? android.view.WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES
                    : android.view.WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_DEFAULT;
            window.setAttributes(params);
        }
        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.R) {
            // Laid out across the whole screen while the bars are hidden, and within them again
            // after.
            window.setDecorFitsSystemWindows(!hidden);
            android.view.WindowInsetsController controller = window.getInsetsController();
            if (controller == null) return;
            if (hidden) {
                controller.setSystemBarsBehavior(android.view.WindowInsetsController.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE);
                controller.hide(android.view.WindowInsets.Type.systemBars());
            } else {
                controller.show(android.view.WindowInsets.Type.systemBars());
            }
            return;
        }
        // Laid out as if the bars weren't there, or the window keeps their room as strips.
        int flags = View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY | View.SYSTEM_UI_FLAG_FULLSCREEN
                | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION | View.SYSTEM_UI_FLAG_LAYOUT_STABLE
                | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION;
        window.getDecorView().setSystemUiVisibility(hidden ? flags : 0);
    }

    @CalledByNative
    private void minimize() {
        activity.moveTaskToBack(true);
    }

    @CalledByNative
    private void close() {
        activity.finish();
    }

    /**
     * Hands a downloaded update to Android's package installer, which asks the user and refuses an
     * APK signed with another key. res/xml/updates.xml names the one folder it may read.
     */
    @CalledByNative
    private void installPackage(String path) {
        Intent install =
                new Intent(Intent.ACTION_VIEW)
                        .setDataAndType(
                                FileProvider.getUriForFile(
                                        activity,
                                        activity.getPackageName() + ".updates",
                                        new File(path)),
                                "application/vnd.android.package-archive")
                        .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        activity.startActivity(install);
    }

    /** A file of ours as other apps read it, through the one FileProvider. */
    private Uri shared(String path) {
        return images.uriOf(new File(path));
    }

    /**
     * Hands a file to Android's chooser, to send or save where the user picks, typed by its
     * name: an image as one, so the apps for images are offered, and a report as text.
     */
    @CalledByNative
    private void shareFile(String path) {
        // A finished download is its copy in the shared Downloads, typed as Android keeps it;
        // anything else of the app's, as a crash report, its own file, typed by its name.
        Uri copy = Downloads.sharedCopy(path);
        String type = copy != null
                ? activity.getContentResolver().getType(copy)
                : URLConnection.guessContentTypeFromName(path);
        Intent send =
                new Intent(Intent.ACTION_SEND)
                        .setType(type != null ? type : "text/plain")
                        .putExtra(Intent.EXTRA_STREAM, copy != null ? copy : shared(path))
                        .putExtra(Intent.EXTRA_SUBJECT, new File(path).getName())
                        .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        activity.startActivity(Intent.createChooser(send, null));
    }

    /** Hands text, such as a link, to Android's chooser. */
    @CalledByNative
    private void shareText(String text) {
        Intent send =
                new Intent(Intent.ACTION_SEND)
                        .setType("text/plain")
                        .putExtra(Intent.EXTRA_TEXT, text);
        activity.startActivity(Intent.createChooser(send, null));
    }

    /**
     * Puts an image file of ours on the clipboard, as a content URI, through Chromium's
     * clipboard, which records it as its own copy: our pages paste it then, and other apps
     * read it through the FileProvider while the clip is there.
     */
    @CalledByNative
    private void copyImage(String path) {
        Clipboard.getInstance().setImageUri(shared(path));
    }

    private int px(double css) {
        return (int) Math.round(css * density);
    }

    private static FrameLayout.LayoutParams copy(FrameLayout.LayoutParams p) {
        return new FrameLayout.LayoutParams(p);
    }

    private static FrameLayout.LayoutParams matchParent() {
        return new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT);
    }

    @NativeMethods
    interface Natives {
        long start(
                BrowserWindow self,
                WindowAndroid windowAndroid,
                long goTable,
                long processAgeMs,
                String blockedHosts,
                boolean skipWelcome);

        void surfaceCreated(long nativeBrowserWindow);

        void surfaceChanged(
                long nativeBrowserWindow,
                int format,
                int width,
                int height,
                android.view.Surface surface);

        void surfaceDestroyed(long nativeBrowserWindow);

        void sizeChanged(long nativeBrowserWindow, double width, double height);

        void pageColorShown(long nativeBrowserWindow, int rgb);

        void contentsMoved(
                long nativeBrowserWindow,
                WebContents contents,
                int left,
                int top,
                int width,
                int height);

        boolean backKey(long nativeBrowserWindow);

        boolean shellOpen(long nativeBrowserWindow);

        void shellMessage(long nativeBrowserWindow, String json);

        void stop(long nativeBrowserWindow);
    }
}
