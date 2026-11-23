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

    /** How check id of the person ended: 0 verified, 1 not, 2 no screen lock. */
    static native void personVerified(int id, int verdict);

    /** How scan id of a code ended: its text, or ok false when nothing was read. */
    static native void codeScanned(int id, String text, boolean ok);
}
