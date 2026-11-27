package app_test

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"projectb/engine"
	"slices"
	"strings"
	"testing"

	"projectb/app"
	"projectb/profiles"
	"projectb/update"
)

// downloads is the list in the last downloads message that matches.
func (r *rig) downloads(match func(list []any) bool) []any {
	r.t.Helper()
	m := r.waitFor(func(m map[string]any) bool {
		list, ok := m["downloads"].([]any)
		return m["t"] == "downloads" && ok && match(list)
	})
	return m["downloads"].([]any)
}

func first(list []any) map[string]any { return list[0].(map[string]any) }

func startDownloading(t *testing.T, profile, dir string, blocked ...string) *rig {
	return startWith(t, app.Config{ProfileDir: profile, Profiles: profiles.New(t.TempDir()), DownloadsDir: dir, BlockedHosts: blocked,
		Update: update.Config{StateDir: filepath.Join(profile, "updates")}})
}

func TestADownloadGoesToTheFolderAndIsListed(t *testing.T) {
	profile, dir := t.TempDir(), t.TempDir()
	r := startDownloading(t, profile, dir)
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	r.e.Fire(1, 0, "download", "https://a.test/files/report.pdf")
	list := r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	d := first(list)
	if d["name"] != "report.pdf" || d["path"] != filepath.Join(dir, "report.pdf") || d["received"] != float64(1000) {
		t.Fatalf("download = %v", d)
	}
	if _, err := os.Stat(filepath.Join(dir, "report.pdf")); err != nil {
		t.Fatalf("the file should be there: %v", err)
	}

	// The same name again is numbered.
	r.e.Fire(1, 0, "download", "https://a.test/other/report.pdf")
	list = r.downloads(func(l []any) bool { return len(l) == 2 && first(l)["state"] == "done" })
	if first(list)["name"] != "report (1).pdf" {
		t.Fatalf("the second should be numbered: %v", first(list))
	}

	// Open shows a file the engine shows itself, a PDF, in a tab of the app's own, and reveal
	// asks the OS; remove leaves the file.
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"open"}`, first(list)["id"]))
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"reveal"}`, first(list)["id"]))
	r.e.Settle()
	pdf := (&url.URL{Scheme: "file", Path: filepath.Join(dir, "report (1).pdf")}).String()
	if want := []string{filepath.Join(dir, "report (1).pdf")}; len(r.sys.Opened()) != 0 || !slices.Contains(r.urls(), pdf) || !slices.Equal(r.sys.Shown(), want) {
		t.Fatalf("opened %v, tabs %v, shown %v", r.sys.Opened(), r.urls(), r.sys.Shown())
	}
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"remove"}`, first(list)["id"]))
	r.downloads(func(l []any) bool { return len(l) == 1 })
	if _, err := os.Stat(filepath.Join(dir, "report (1).pdf")); err != nil {
		t.Fatalf("removing it from the list keeps the file: %v", err)
	}
}

// On a phone a finished download's file goes to the shared Downloads, leaving its path
// empty: the same name again still gets a path of its own, as each download's copy there is
// found by its path (docs/fragility.md, "Delete on the older of two same-named downloads").
func TestTheSameNameAfterItsFileMovedGetsItsOwnPath(t *testing.T) {
	profile, dir := t.TempDir(), t.TempDir()
	r := startDownloading(t, profile, dir)
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://a.test/files/report.pdf")
	r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	if err := os.Remove(filepath.Join(dir, "report.pdf")); err != nil {
		t.Fatal(err)
	}
	r.e.Fire(1, 0, "download", "https://a.test/files/report.pdf")
	list := r.downloads(func(l []any) bool { return len(l) == 2 && first(l)["state"] == "done" })
	if first(list)["path"] != filepath.Join(dir, "report (1).pdf") {
		t.Fatalf("the second has %v, not a path of its own", first(list)["path"])
	}
}

func TestTheListOutlivesARestart(t *testing.T) {
	profile, dir := t.TempDir(), t.TempDir()
	r := startDownloading(t, profile, dir)
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://a.test/a.zip")
	r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	r.stop()

	r = startDownloading(t, profile, dir)
	defer r.stop()
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	list, _ := snap["downloads"].([]any)
	if len(list) != 1 || first(list)["name"] != "a.zip" {
		t.Fatalf("snapshot downloads = %v", snap["downloads"])
	}
	r.shell(`{"t":"clearDownloads"}`)
	r.downloads(func(l []any) bool { return len(l) == 0 })
}

func TestARunningDownloadCanBeCancelled(t *testing.T) {
	r := startDownloading(t, t.TempDir(), t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://a.test/big.iso")
	list := r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "progress" })
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"cancel"}`, first(list)["id"]))
	r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "cancelled" })
}

func TestABlockedAddressDownloadsNothing(t *testing.T) {
	dir := t.TempDir()
	r := startDownloading(t, t.TempDir(), dir, "blocked.test")
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://blocked.test/x.exe")
	r.e.Fire(1, 0, "download", "https://a.test/ok.txt")
	list := r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	if first(list)["name"] != "ok.txt" {
		t.Fatalf("only the allowed one should be listed: %v", list)
	}
}

func TestSaveLinkAsDownloadsThroughThePanel(t *testing.T) {
	dir := t.TempDir()
	r := startDownloading(t, t.TempDir(), dir)
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.choose(r.rightClick(`{"link":"https://a.test/notes.txt"}`), "link.saveAs")
	list := r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	if first(list)["path"] != filepath.Join(dir, "notes.txt") {
		t.Fatalf("download = %v", first(list))
	}
}

// Ask where to save, Settings' switch: every download asks for the Save panel, starting at
// the folder's, not only Save Link As's.
func TestAskWhereToSaveSendsEveryDownloadThroughThePanel(t *testing.T) {
	dir := t.TempDir()
	r := startDownloading(t, t.TempDir(), dir)
	defer r.stop()
	target := func() (got engine.DownloadTarget) {
		r.main(func() {
			got = r.a.DownloadTarget(engine.DownloadRequest{ID: 1, URL: "https://a.test/x.txt", Name: "x.txt"})
		})
		return got
	}
	if got := target(); got.Prompt {
		t.Fatalf("with it off: %+v", got)
	}
	got, _ := r.openSettings()
	r.settings(`{"t":"setPref","name":"askWhereToSave","value":"true"}`)
	r.waitOn(got, func(m map[string]any) bool {
		return m["t"] == "prefs" && m["prefs"].(map[string]any)["askWhereToSave"] == true
	})
	if got := target(); !got.Prompt || got.Path != filepath.Join(dir, "x.txt") {
		t.Fatalf("with it on: %+v", got)
	}
}

func TestEachDownloadChangeIsShownBySystem(t *testing.T) {
	profile, dir := t.TempDir(), t.TempDir()
	r := startDownloading(t, profile, dir)
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://a.test/files/report.pdf")
	list := r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	r.e.Settle()
	notices := r.e.Notices()
	if len(notices) == 0 {
		t.Fatal("the system was shown nothing of the download")
	}
	if last := notices[len(notices)-1]; last.State != engine.DownloadDone || last.Path != first(list)["path"] {
		t.Fatalf("the last shown = %+v, want it done at %v", last, first(list)["path"])
	}
}

// A download that failed, as one whose connection died across the Mac's sleep, can be
// retried, from where it got to, in its own row, and its address copied.
func TestAFailedDownloadCanBeRetriedAndItsLinkCopied(t *testing.T) {
	r := startDownloading(t, t.TempDir(), t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://a.test/big.iso")
	list := r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "progress" })
	id := first(list)["id"]
	r.e.Fire(1, 0, "downloadsFail", "")
	r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "failed" })

	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"copyLink"}`, id))
	r.e.Settle()
	if got := r.e.Clipboard(); got != "https://a.test/big.iso" {
		t.Fatalf("clipboard = %q", got)
	}
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"retry"}`, id))
	list = r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	if first(list)["id"] != id || first(list)["name"] != "big.iso" {
		t.Fatalf("the retry should finish in the same row: %v", list)
	}
}

// One cancelled, or failed in an earlier run, starts over as a new download of its address,
// in the same row and under the same name.
func TestACancelledDownloadStartsOverOnRetry(t *testing.T) {
	r := startDownloading(t, t.TempDir(), t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://a.test/big.iso")
	list := r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "progress" })
	id := first(list)["id"]
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"cancel"}`, id))
	r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "cancelled" })
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"retry"}`, id))
	list = r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	if first(list)["id"] != id || first(list)["name"] != "big.iso" {
		t.Fatalf("the retry should finish in the same row, by its name: %v", list)
	}
}

// A download that is a stream's playlist is dropped from the engine once it is read, but
// never from inside the engine's news of it: the engine is telling the download's observers
// then, and taking the download away under them crashed the app (2026-11-27). No command
// the app gives a download comes from inside its news.
func TestNoDownloadCommandComesFromInsideADownloadsNews(t *testing.T) {
	r := startDownloading(t, t.TempDir(), t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	// The mock engine's file is no playlist: the row fails, the playlist read and dropped.
	r.e.Fire(1, 0, "download", "https://a.test/watch/film.m3u8")
	d := first(r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "failed" }))
	if d["name"] != "film.mp4" || d["reason"] != "format" || d["noRetry"] != true {
		t.Fatalf("the stream's row: %v", d)
	}
	// And the everyday ones: cancelled, tried again, removed while it runs.
	r.e.Fire(1, 0, "download", "https://a.test/big.iso")
	id := first(r.downloads(func(l []any) bool { return len(l) == 2 && first(l)["state"] == "progress" }))["id"]
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"cancel"}`, id))
	r.downloads(func(l []any) bool { return first(l)["state"] == "cancelled" })
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"retry"}`, id))
	r.downloads(func(l []any) bool { return first(l)["state"] == "progress" })
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"remove"}`, id))
	r.downloads(func(l []any) bool { return len(l) == 1 })
	r.e.Settle()
	if got := r.e.ReentrantDownloadCommands(); len(got) != 0 {
		t.Fatalf("commands given from inside a download's news: %v", got)
	}
}

// Retry starts one over from the page it came from, as the first time: a site may serve its
// files only to its own pages, and the engine has forgotten a cancelled one.
func TestARetryComesFromThePageTheDownloadCameFrom(t *testing.T) {
	r := startDownloading(t, t.TempDir(), t.TempDir())
	defer r.stop()
	r.visit("https://a.test/video")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://files.b.test/v.mp4")
	list := r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "progress" })
	id := first(list)["id"]
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"cancel"}`, id))
	r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "cancelled" })
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"retry"}`, id))
	r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	started := r.e.Started()
	if len(started) != 2 || started[1].URL != "https://files.b.test/v.mp4" || started[1].Referrer != "https://a.test/video" {
		t.Fatalf("the retry should come from the page the download came from: %+v", started)
	}
}

// A download a tab of the window in front begins tells that window's shell, after the list
// that has it, so it can show the download begin; one begun in a window behind tells none.
func TestADownloadBegunInTheWindowInFrontIsAnnounced(t *testing.T) {
	profile, dir := t.TempDir(), t.TempDir()
	r := startDownloading(t, profile, dir)
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	r.e.Fire(1, 0, "download", "https://a.test/files/report.pdf")
	listed := false
	m := r.waitFor(func(m map[string]any) bool {
		if l, ok := m["downloads"].([]any); ok && m["t"] == "downloads" && len(l) == 1 {
			listed = true
		}
		return m["t"] == "downloadStarted"
	})
	if !listed || m["id"] != float64(1) {
		t.Fatalf("downloadStarted = %v, after the list: %v", m, listed)
	}

	second := r.openSecondWindow()
	r.e.Fire(1, 0, "download", "https://a.test/files/other.pdf")
	r.waitFor(func(m map[string]any) bool {
		if m["t"] == "downloadStarted" {
			t.Fatalf("window 1, behind, heard %v", m)
		}
		l, ok := m["downloads"].([]any)
		return m["t"] == "downloads" && ok && len(l) == 2 && first(l)["state"] == "done"
	})
	for {
		select {
		case b := <-second:
			if strings.Contains(string(b), `"downloadStarted"`) {
				t.Fatalf("window 2 heard %s", b)
			}
			continue
		default:
		}
		break
	}
}

// Save report keeps a crash report as a finished download: copied into the folder, listed
// as the app's own with no address, shown by the system as a finished download, handed on
// to be published, and told to the window as a download that began there.
func TestASavedReportIsAFinishedDownload(t *testing.T) {
	profile, dir := t.TempDir(), t.TempDir()
	r := startDownloading(t, profile, dir)
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)

	r.shell(`{"t":"saveReport"}`)
	list := r.downloads(func(l []any) bool { return len(l) == 1 })
	d := first(list)
	name, _ := d["name"].(string)
	if !strings.HasPrefix(name, "project-b-report-") || d["path"] != filepath.Join(dir, name) || d["state"] != "done" ||
		d["url"] != "" || d["from"] != "Crash report" {
		t.Fatalf("download = %v", d)
	}
	if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || !strings.Contains(string(b), "Project B crash report") {
		t.Fatalf("the report should be in the folder: %v", err)
	}
	r.waitFor(func(m map[string]any) bool { return m["t"] == "downloadStarted" && m["id"] == d["id"] })
	r.e.Settle()
	if r.e.Published() != d["path"] {
		t.Fatalf("published %q, want %v", r.e.Published(), d["path"])
	}
	if n := r.e.Notices(); len(n) != 1 || n[0].State != engine.DownloadDone || n[0].Path != d["path"] {
		t.Fatalf("the system was shown %+v", n)
	}

	// It has no address to copy or retry.
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"copyLink"}`, d["id"]))
	r.e.Settle()
	if got := r.e.Clipboard(); got != "" {
		t.Fatalf("clipboard = %q", got)
	}
}

// Retry on a running download stops it and starts it over, in the same row.
func TestARunningDownloadStartsOverOnRetry(t *testing.T) {
	r := startDownloading(t, t.TempDir(), t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://a.test/big.iso")
	r.e.Fire(1, 0, "downloadsStall", "")
	list := r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "progress" })
	id := first(list)["id"]
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"retry"}`, id))
	list = r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	if first(list)["id"] != id || first(list)["name"] != "big.iso" {
		t.Fatalf("the retry should finish in the same row: %v", list)
	}
}

// Open takes what the engine shows itself, a page, to a tab of the app's own, and anything
// else, a zip, to the OS's app for it (oskit/open).
func TestOpenShowsAPageHereAndHandsAZipToTheOS(t *testing.T) {
	profile, dir := t.TempDir(), t.TempDir()
	r := startDownloading(t, profile, dir)
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://a.test/files/notes.html")
	page := first(r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" }))
	r.e.Fire(1, 0, "download", "https://a.test/files/photos.zip")
	zip := first(r.downloads(func(l []any) bool { return len(l) == 2 && first(l)["state"] == "done" }))

	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"open"}`, page["id"]))
	r.shell(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"open"}`, zip["id"]))
	r.e.Settle()
	html := (&url.URL{Scheme: "file", Path: filepath.Join(dir, "notes.html")}).String()
	if !slices.Contains(r.urls(), html) {
		t.Fatalf("the page didn't open in a tab: %v", r.urls())
	}
	if want := []string{filepath.Join(dir, "photos.zip")}; !slices.Equal(r.sys.Opened(), want) {
		t.Fatalf("the OS opened %v; want only the zip", r.sys.Opened())
	}
}
