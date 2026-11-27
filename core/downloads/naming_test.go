package downloads

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// The random rule, both ways, on names as servers give them.
func TestWhichNamesLookRandom(t *testing.T) {
	for _, stem := range []string{
		"1697040000", "20261127_093015", "0001", "a1b2c3d4e5f6a7b8", "DEADBEEF00112233", "9f86d081884c7d659a2feaa0c55ad015",
		"550e8400-e29b-41d4-a716-446655440000", "video-550E8400-E29B-41D4-A716-446655440000_720",
		"video", "index", "master", "playlist", "download", "file", "media", "stream", "output", "chunk", "seg", "segment",
		"Video_1080", "master.720", "playlist-2", "seg-00017", "stream_0_1697040000", "index (1)", "media-9f86d081884c7d65",
		"k3J9xQ2m5N8p1R7tZ4vW", "x7Gq2LmZ9pRt4VbN8cYw", "aB3dE6gH9jK2mN5pQ8sT1vW4", "0f3a9c2e7b1d4f68-1080-video",
		"", "---", "..",
	} {
		if !LooksRandom(stem) {
			t.Errorf("%q should look random", stem)
		}
	}
	for _, stem := range []string{
		"My Holiday 2024", "report-final", "IMG_1234", "DSC00012", "lecture-03", "Big.Buck.Bunny.1080p", "invoice_2026_11",
		"quarterly-report", "videoplayback", "setup", "a", "v1", "cat", "Screenshot 2026-11-27 at 09.30.15", "trailer_720p",
		"TheQuickBrownFoxJumps2024", "internationalisation2026x", "episode1", "deadbeat", "video-of-the-year",
		"season-2-episode-4-9f86d081", "résumé", "夏の旅行", "master-plan",
	} {
		if LooksRandom(stem) {
			t.Errorf("%q should not look random", stem)
		}
	}
}

// The better text, in order: the title the page's link carried, the page's title, then the
// page's host and the words of its path. A name that says something stays.
func TestTheOrderOfBetterNames(t *testing.T) {
	all := Naming{Mode: NamesAuto, Linked: "Big Buck Bunny", Title: "Watch films - Example", Page: "https://www.example.com/watch/funny-cats/9f86d081884c7d65?x=1"}
	for _, c := range []struct {
		n        Naming
		original string
		want     string
	}{
		{all, "a1b2c3d4e5f6a7b8.mp4", "Big Buck Bunny.mp4"},
		{Naming{Mode: NamesAuto, Title: all.Title, Page: all.Page}, "a1b2c3d4e5f6a7b8.mp4", "Watch films - Example.mp4"},
		{Naming{Mode: NamesAuto, Page: all.Page}, "master.mp4", "example.com watch funny cats.mp4"},
		{Naming{Mode: NamesAuto, Page: "https://cdn.example.net/"}, "1697040000.zip", "cdn.example.net.zip"},
		{Naming{Mode: NamesAuto}, "master.mp4", "master.mp4"},
		{Naming{Mode: NamesAuto, Page: "file:///Users/someone/page.html"}, "video.mp4", "video.mp4"},
		{all, "holiday-in-goa.mp4", "holiday-in-goa.mp4"},
		{all, "download", "Big Buck Bunny"},
		// An unknown mode is Automatic.
		{Naming{Linked: "A title"}, "index.mp4", "A title.mp4"},
		// Title and file name: every file, random or not, by the link's title or the page's;
		// never by the page's address.
		{Naming{Mode: NamesTitle, Linked: all.Linked, Title: all.Title}, "holiday-in-goa.mp4", "Big Buck Bunny – holiday-in-goa.mp4"},
		{Naming{Mode: NamesTitle, Title: all.Title}, "a1b2c3d4.mp4", "Watch films - Example – a1b2c3d4.mp4"},
		{Naming{Mode: NamesTitle, Page: all.Page}, "a1b2c3d4.mp4", "a1b2c3d4.mp4"},
		// File's own name: never.
		{Naming{Mode: NamesFile, Linked: all.Linked, Title: all.Title, Page: all.Page}, "a1b2c3d4e5f6a7b8.mp4", "a1b2c3d4e5f6a7b8.mp4"},
	} {
		got, renamed := c.n.Name(c.original)
		if got != c.want || renamed != (c.want != c.original) {
			t.Errorf("%+v names %q %q (renamed %v), want %q", c.n, c.original, got, renamed, c.want)
		}
	}
}

// Only the name comes from the page: the ending is always the file's own, and nothing a
// file name may not hold gets through.
func TestOnlyTheNameComesFromThePage(t *testing.T) {
	for _, c := range []struct{ title, original, want string }{
		{"report.exe", "a1b2c3d4e5f6a7b8.mp4", "report.exe.mp4"},
		{"../../etc/passwd", "video.mp4", "etc passwd.mp4"},
		{`C:\Windows\system32\cmd`, "video.mp4", "C Windows system32 cmd.mp4"},
		{"a/b\\c:d*e?f\"g<h>i|j", "video.mp4", "a b c d e f g h i j.mp4"},
		{"  spaced \t out\n title.  ", "video.mp4", "spaced out title.mp4"},
		{"line\x00one\x07\u202etxt.exe", "video.mp4", "line one txt.exe.mp4"},
		{"...", "video.mp4", "video.mp4"},
		{"Invoice", "1697040000", "Invoice"},
		{"Invoice.pdf", "1697040000.html", "Invoice.pdf.html"},
		{strings.Repeat("long title ", 40), "video.mp4", ""},
	} {
		got, _ := Naming{Mode: NamesAuto, Linked: c.title}.Name(c.original)
		if c.want == "" {
			stem := strings.TrimSuffix(got, ".mp4")
			if n := utf8.RuneCountInString(stem); n > nameLimit || n < nameLimit-12 || !strings.HasSuffix(got, ".mp4") || strings.HasSuffix(stem, " ") {
				t.Errorf("a long title made a name of %d characters: %q", n, got)
			}
			continue
		}
		if got != c.want {
			t.Errorf("%q names %q %q, want %q", c.title, c.original, got, c.want)
		}
	}
	for address, want := range map[string]string{
		"https://cdn.test/v/index.m3u8#pb-download":                              "",
		"https://cdn.test/v/index.m3u8#pb-download=Big%20Buck%20Bunny":           "Big Buck Bunny",
		"https://cdn.test/v/index.m3u8?a=1#pb-download=Caf%C3%A9%20%2B%20bar%2F": "Café + bar/",
		"https://cdn.test/v/index.m3u8#pb-download=":                             "",
		"https://cdn.test/v/index.m3u8#other=Title":                              "",
		"https://cdn.test/v/index.m3u8#pb-download=%zz":                          "",
		"https://cdn.test/v/index.m3u8":                                          "",
	} {
		if got := LinkedTitle(address); got != want {
			t.Errorf("%s carries %q, want %q", address, got, want)
		}
	}
	// A quality rides after the title, &q= and a height, or alone; the forms without one
	// read as they did.
	for address, want := range map[string]struct {
		title  string
		height int
	}{
		"https://cdn.test/v/master.m3u8#pb-download":                          {"", 0},
		"https://cdn.test/v/master.m3u8#pb-download=Big%20Buck%20Bunny":       {"Big Buck Bunny", 0},
		"https://cdn.test/v/master.m3u8#pb-download=Big%20Buck%20Bunny&q=720": {"Big Buck Bunny", 720},
		"https://cdn.test/v/master.m3u8?a=1#pb-download=&q=480":               {"", 480},
		"https://cdn.test/v/master.m3u8#pb-download&q=480":                    {"", 0},
		"https://cdn.test/v/master.m3u8#q=480":                                {"", 0},
		// A title's own & and = are percent-encoded, and stay the title's.
		"https://cdn.test/v/master.m3u8#pb-download=R%26q%3D360&q=1080": {"R&q=360", 1080},
		"https://cdn.test/v/master.m3u8#pb-download=R%26q%3D360":        {"R&q=360", 0},
		// Only digits, the last thing in it, are a quality: anything else is the title.
		"https://cdn.test/v/master.m3u8#pb-download=Tom%20&%20Jerry": {"Tom & Jerry", 0},
		"https://cdn.test/v/master.m3u8#pb-download=A&q=best":        {"A&q=best", 0},
		"https://cdn.test/v/master.m3u8#pb-download=A&q=720&more":    {"A&q=720&more", 0},
		"https://cdn.test/v/master.m3u8#pb-download=A&q=0":           {"A&q=0", 0},
		"https://cdn.test/v/master.m3u8#pb-download=A&q=-720":        {"A&q=-720", 0},
		"https://cdn.test/v/master.m3u8#pb-download=A&q=1234567":     {"A&q=1234567", 0},
		"https://cdn.test/v/master.m3u8#pb-download=A&q=360&q=720":   {"A&q=360", 720},
		"https://cdn.test/v/master.m3u8#pb-download=%zz&q=720":       {"", 0},
	} {
		if title, height := LinkedTitle(address), LinkedQuality(address); title != want.title || height != want.height {
			t.Errorf("%s carries %q at %d, want %q at %d", address, title, height, want.title, want.height)
		}
	}
}
