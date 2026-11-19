package dev.projectb.browser;

/**
 * The Go core. Loading this library is what starts Go's runtime, and that has to happen in the main
 * process only, so only BrowserActivity touches this class. An activity with no android:process
 * never runs in a renderer or a GPU process. Content's own loader doesn't know about this library,
 * so nothing loads it on a child's way up.
 */
final class GoCore {
    static {
        System.loadLibrary("project_b_go");
    }

    private GoCore() {}

    /**
     * The Go core's start, before content's: it reads the command line, the program first, and
     * claims the profile it names. Null when this copy quits; else the switches Go added, for
     * content's command line.
     */
    static native String[] init(String[] commandLine);

    /** The address of the table of Go's functions, for the content library. */
    static native long table();

    /** An answer to a question Go asked of Android (GoSystem), on the main thread. */
    static native void answered(int question, boolean yes);

    /** Opens a link another app sent, as a new tab. Only after Go started. */
    static native void openUrl(String url);
}
