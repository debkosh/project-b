package browserdata

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// testdata/home is made by testdata/make.sh.
const home = "testdata/home"

func found(t *testing.T) map[string]Profile {
	t.Helper()
	out := map[string]Profile{}
	for _, b := range Find(home) {
		for _, p := range b.Profiles {
			out[b.ID+"/"+p.Name] = p
		}
	}
	return out
}

func TestFindListsEachBrowsersProfiles(t *testing.T) {
	bs := Find(home)
	var got []string
	for _, b := range bs {
		for _, p := range b.Profiles {
			got = append(got, strings.TrimSuffix(b.Name+" · "+p.Name, " · "))
		}
	}
	// The System Profile and a Firefox profile whose folder is gone aren't listed; Safari
	// has one profile, with no name, as has Opera, whose profile is its folder. Forks are
	// found as their parents are.
	want := []string{"Chrome · Personal", "Chrome · Work", "Opera", "Firefox · default-release", "Zen · Default (release)", "Safari"}
	if !slices.Equal(got, want) {
		t.Fatalf("found %q, want %q", got, want)
	}
}

// A folder the person chooses is read as its browser's: a known one's folder or profile
// is that browser, another is named after its folder, and a folder of neither family's
// files is no browser.
func TestOpenAFolderChosen(t *testing.T) {
	abs, _ := filepath.Abs(home)
	support := filepath.Join(abs, "Library/Application Support")
	for _, c := range []struct {
		dir, id, name string
		profiles      []string
	}{
		{filepath.Join(support, "Google/Chrome"), "chrome", "Chrome", []string{"Personal", "Work"}},
		{filepath.Join(support, "Google/Chrome/Profile 1"), "chrome", "Chrome", []string{"Work"}},
		{filepath.Join(support, "com.operasoftware.Opera"), "opera", "Opera", []string{""}},
		{filepath.Join(support, "zen"), "zen", "Zen", []string{"Default (release)"}},
		{filepath.Join(support, "Firefox/Profiles/test.default-release"), "firefox", "Firefox", []string{"test.default-release"}},
		{filepath.Join(abs, "Elsewhere/Thorium/User Data"), OtherChromium, "Thorium", []string{"Me"}},
		{filepath.Join(abs, "Elsewhere/Thorium/User Data/Default"), OtherChromium, "Thorium", []string{"Me"}},
	} {
		b, err := Open(abs, c.dir)
		var names []string
		for _, p := range b.Profiles {
			names = append(names, p.Name)
		}
		if err != nil || b.ID != c.id || b.Name != c.name || !slices.Equal(names, c.profiles) {
			t.Errorf("%s: %s %q %q, %v; want %s %q %q", c.dir, b.ID, b.Name, names, err, c.id, c.name, c.profiles)
		}
	}
	// What was opened reads as any profile does.
	b, _ := Open(abs, filepath.Join(abs, "Elsewhere/Thorium/User Data"))
	if c, err := b.Profiles[0].Counts(); err != nil || c.Bookmarks != 1 {
		t.Errorf("Thorium's counts %+v, %v", c, err)
	}
	if _, err := Open(abs, filepath.Join(abs, "Library")); err != ErrNoBrowser {
		t.Errorf("Library opened: %v", err)
	}
}

// titles is a tree as text: a folder's title, then what it holds in brackets.
func titles(bs []Bookmark) string {
	var b bytes.Buffer
	for i, x := range bs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(x.Title)
		if x.Folder() {
			b.WriteString(" [" + titles(x.Children) + "]")
		}
	}
	return b.String()
}

func TestBookmarksKeepTheirFolders(t *testing.T) {
	ps := found(t)
	for key, want := range map[string]string{
		// Empty top folders are left out.
		"chrome/Personal": "Bookmarks bar [Docs, Work [Tracker, Deep [Deep link]]], Other bookmarks [News]",
		"chrome/Work":     "Bookmarks bar [Wiki]",
		// Firefox's tags, separators and saved searches aren't bookmarks.
		"firefox/default-release": "Bookmarks Toolbar [Fx], Bookmarks Menu [Reading [Read]]",
		// Safari's History proxy isn't a folder; its top folders take the names Safari shows.
		"safari/": "Favorites [Apple, Recipes [Soup, Bread]], Reading List [Article]",
	} {
		bs, err := ps[key].Bookmarks()
		if err != nil {
			t.Fatal(key, err)
		}
		if got := titles(bs); got != want {
			t.Errorf("%s: %s, want %s", key, got, want)
		}
	}
	bs, _ := ps["chrome/Personal"].Bookmarks()
	if d := bs[0].Children[0]; d.URL != "https://example.com/docs" || d.Added.Year() != 2024 {
		t.Errorf("Docs is %+v", d)
	}
	bs, _ = ps["firefox/default-release"].Bookmarks()
	if fx := bs[0].Children[0]; fx.URL != "https://example.net/fx" || !fx.Added.Equal(time.UnixMicro(1700000000000000)) {
		t.Errorf("Fx is %+v", fx)
	}
	bs, _ = ps["safari/"].Bookmarks()
	added := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	if soup := bs[0].Children[1].Children[0]; soup.URL != "https://example.com/soup" || !soup.Added.Equal(added) {
		t.Errorf("Soup is %+v", soup)
	}
	if a := bs[1].Children[0]; !a.Added.Equal(added) {
		t.Errorf("the Reading List's Article is %+v", a)
	}
}

func TestHistoryIsWebPagesVisited(t *testing.T) {
	ps := found(t)
	for key, want := range map[string][]string{
		// Not chrome:// pages, hidden ones, or those never visited.
		"chrome/Personal":         {"http://example.org/old", "https://example.com/docs", "https://example.com/tracker"},
		"chrome/Work":             nil,
		"firefox/default-release": {"https://example.net/fx", "https://example.net/read"},
		// Not a file, or a page never visited.
		"safari/": {"http://example.org/old", "https://example.com/soup"},
	} {
		pages, err := ps[key].History()
		if err != nil {
			t.Fatal(key, err)
		}
		var got []string
		for _, p := range pages {
			got = append(got, p.URL)
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s: %q, want %q", key, got, want)
		}
		c, err := ps[key].Counts()
		if err != nil || c.Pages != len(want) {
			t.Errorf("%s counts %+v, %v", key, c, err)
		}
	}
	pages, _ := ps["chrome/Personal"].History()
	for _, p := range pages {
		if p.URL == "https://example.com/docs" && (p.Visits != 12 || p.Title != "Docs" || p.Last.Year() != 2024) {
			t.Errorf("Docs is %+v", p)
		}
	}
	// A Safari page's title is its last visit's that has one; its last visit is the latest.
	pages, _ = ps["safari/"].History()
	for _, p := range pages {
		if p.URL == "https://example.com/soup" && (p.Visits != 3 || p.Title != "Soup" || !p.Last.Equal(appleTime(815200000))) {
			t.Errorf("Soup is %+v", p)
		}
	}
}

func TestCounts(t *testing.T) {
	c, err := found(t)["chrome/Personal"].Counts()
	want := Counts{Bookmarks: 4, Folders: 2, Pages: 3, Extensions: 2, Passwords: 3}
	if err != nil || c != want {
		t.Fatalf("%+v, %v; want %+v", c, err, want)
	}
}

func TestExtensionsAreThoseItsPersonInstalled(t *testing.T) {
	ps := found(t)
	got, err := ps["chrome/Personal"].Extensions()
	if err != nil {
		t.Fatal(err)
	}
	want := []Extension{
		{ID: "pponmlkjihgfedcbaponmlkjihgfedcb", Name: "my-extension", Skip: SkipUnpacked},
		{ID: "nngceckbapebfimnlniiiahkandclblb", Name: "Old Notes", Version: "1.9", Skip: SkipManifestV2},
		{ID: "abcdefghijklmnopabcdefghijklmnop", Name: "Other Store Helper", Version: "3.0", Skip: SkipNotInStore},
		{ID: "oepjjojhhaclfhmopocgmdfbeiffpheh", Name: "Project B test extension", Version: "1.0"},
		{ID: "ddkjiahejlhfcafbddmgiahcphecmpfh", Name: "uBlock Origin Lite", Version: "2025.1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	for _, key := range []string{"chrome/Work", "firefox/default-release", "safari/"} {
		if e, err := ps[key].Extensions(); len(e) != 0 || err != nil {
			t.Errorf("%s has %+v, %v", key, e, err)
		}
	}
}

func TestSafariIsLockedUntilItsFolderCanBeRead(t *testing.T) {
	ps := found(t)
	if ps["safari/"].Locked() || ps["chrome/Personal"].Locked() {
		t.Fatal("locked with its files there to read")
	}
	// macOS's Full Disk Access refuses as a folder with no permissions does.
	dir := filepath.Join(t.TempDir(), "Library/Safari")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := Profile{Browser: "safari", Dir: dir}
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	if !p.Locked() {
		t.Error("not locked when its folder can't be read")
	}
	os.Chmod(dir, 0o755)
	if p.Locked() {
		t.Error("still locked once it can be read")
	}
}

func TestBplist(t *testing.T) {
	// The fixture's, as Python's plistlib writes them: dictionaries, arrays, strings,
	// dates and booleans.
	b, err := os.ReadFile(filepath.Join(home, "Library/Safari/Bookmarks.plist"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := bplist(b)
	if m, ok := v.(map[string]any); err != nil || !ok || m["WebBookmarkFileVersion"] != int64(1) {
		t.Fatalf("%v, %v", v, err)
	}
	for _, bad := range [][]byte{nil, []byte("bplist00"), b[:len(b)-1], append([]byte("bplist01"), b[8:]...)} {
		if _, err := bplist(bad); err == nil {
			t.Errorf("read %d bytes that aren't a property list", len(bad))
		}
	}
	// A list that holds itself.
	self := []byte("bplist00\xa1\x00\x08\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00\x00\x00\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x0a")
	if _, err := bplist(self); err == nil {
		t.Error("read a list that holds itself")
	}
}

// keychain is the login keychain as the test home's Chrome has it.
func keychain(service, account string) ([]byte, error) {
	if service == "Chrome Safe Storage" && account == "Chrome" {
		return []byte("test-safe-storage"), nil
	}
	return nil, ErrNoSecret
}

// Saved passwords: a Chromium profile's, opened with the keychain's key, a site's alone,
// from Login Data and Login Data For Account; a Firefox profile's with key4.db's key; a
// Firefox fork's behind a primary password only with it; Safari's none.
func TestLogins(t *testing.T) {
	ps := found(t)
	short := func(ls []Login) []string {
		var out []string
		for _, l := range ls {
			out = append(out, l.Origin+" "+l.Username+" "+l.Password)
		}
		slices.Sort(out)
		return out
	}
	got, err := ps["chrome/Personal"].Logins(Keys{Secret: keychain})
	want := []string{"https://account.example.com/ erin synced!", "https://example.com/login alice s3cret!", "https://example.org/ bob hunter2"}
	if err != nil || !slices.Equal(short(got), want) {
		t.Errorf("Chrome: %q, %v; want %q", short(got), err, want)
	}
	if got[0].Created.Year() != 2024 || got[0].LastUsed.IsZero() {
		t.Errorf("Chrome's times %v %v", got[0].Created, got[0].LastUsed)
	}
	if _, err := ps["chrome/Personal"].Logins(Keys{}); err != ErrNoSecret {
		t.Errorf("Chrome with no keychain: %v", err)
	}
	wrong := func(string, string) ([]byte, error) { return []byte("not it"), nil }
	if l, err := ps["chrome/Personal"].Logins(Keys{Secret: wrong}); err == nil {
		t.Errorf("Chrome with the wrong key: %q", short(l))
	}

	got, err = ps["firefox/default-release"].Logins(Keys{})
	want = []string{"https://example.com alice s3cret!", "https://example.net frank correct horse"}
	if err != nil || !slices.Equal(short(got), want) || got[0].Created.IsZero() {
		t.Errorf("Firefox: %q, %v; want %q", short(got), err, want)
	}
	if ps["firefox/default-release"].HasPrimaryPassword() || !ps["zen/Default (release)"].HasPrimaryPassword() {
		t.Error("only Zen's profile has a primary password")
	}
	if _, err := ps["zen/Default (release)"].Logins(Keys{PrimaryPassword: "nope"}); err != ErrPrimaryPassword {
		t.Errorf("Zen with the wrong primary password: %v", err)
	}
	got, err = ps["zen/Default (release)"].Logins(Keys{PrimaryPassword: "letmein"})
	if err != nil || !slices.Equal(short(got), []string{"https://zen.example.com zed z3n"}) {
		t.Errorf("Zen: %q, %v", short(got), err)
	}
	if got, err := ps["safari/"].Logins(Keys{}); got != nil || err != nil {
		t.Errorf("Safari: %v, %v", got, err)
	}
	for key, want := range map[string]int{"chrome/Personal": 3, "firefox/default-release": 2, "zen/Default (release)": 1, "chrome/Work": 0} {
		if c, _ := ps[key].Counts(); c.Passwords != want {
			t.Errorf("%s counts %d passwords; want %d", key, c.Passwords, want)
		}
	}
}

// NSS's older sealing, SHA1-and-3DES, opens as Firefox's key derivation has it: sealed here
// with the same derivation, and the derivation checked against a value worked by hand.
func TestSHA1And3DESKey(t *testing.T) {
	key := sha1And3DESKey(bytes.Repeat([]byte{1}, 20), []byte("entry-salt"))
	if len(key) != 40 {
		t.Fatalf("%d bytes", len(key))
	}
	block, _ := des.NewTripleDESCipher(key[:24])
	plain := []byte("password-check\x02\x02")
	sealed := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, key[32:]).CryptBlocks(sealed, plain)
	got, err := cbcOpen(des.NewTripleDESCipher, key[:24], key[32:], sealed)
	if err != nil || string(got) != "password-check" {
		t.Fatalf("%q, %v", got, err)
	}
}

// Files a person exported: passwords as Chrome's, Firefox's and the Passwords app's CSV,
// and bookmarks as Netscape's bookmark file, folders in folders.
func TestReadExportedFiles(t *testing.T) {
	for name, csv := range map[string]string{
		"Chrome":    "name,url,username,password,note\nExample,https://example.com/,alice,s3cret!,\n",
		"Firefox":   "\"url\",\"username\",\"password\",\"httpRealm\",\"formActionOrigin\",\"guid\",\"timeCreated\",\"timeLastUsed\",\"timePasswordChanged\"\n\"https://example.com\",\"alice\",\"s3cret!\",,\"https://example.com\",\"{1}\",\"1700000000000\",\"1700000100000\",\"1700000000000\"\n\"https://auth.example.com\",\"gina\",\"basic\",\"Realm\",,\"{2}\",\"1\",\"1\",\"1\"\n",
		"Passwords": "\ufeffTitle,URL,Username,Password,Notes,OTPAuth\nexample.com (alice),https://example.com/,alice,s3cret!,,\nnothing,https://empty.example.com/,nobody,,,\n",
	} {
		got, err := ReadPasswordsCSV(strings.NewReader(csv))
		if err != nil || len(got) != 1 || got[0].Username != "alice" || got[0].Password != "s3cret!" || !strings.HasPrefix(got[0].Origin, "https://example.com") {
			t.Errorf("%s: %+v, %v", name, got, err)
		}
	}
	if _, err := ReadPasswordsCSV(strings.NewReader("just,some,columns\n1,2,3\n")); err != ErrNotAFile {
		t.Errorf("a CSV of no passwords: %v", err)
	}

	page := `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<META HTTP-EQUIV="Content-Type" CONTENT="text/html; charset=UTF-8">
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
    <DT><H3 ADD_DATE="1700000000" PERSONAL_TOOLBAR_FOLDER="true">Bookmarks bar</H3>
    <DL><p>
        <DT><A HREF="https://example.com/docs" ADD_DATE="1700000000">Docs &amp; notes</A>
        <DT><H3>Recipes</H3>
        <DL><p>
            <DT><A HREF="https://example.com/soup">Soup</A>
        </DL><p>
        <DT><A HREF="javascript:alert(1)">A bookmarklet</A>
    </DL><p>
    <DT><A HREF="https://example.org/news">News</A>
</DL><p>
`
	got, err := ReadBookmarksHTML(strings.NewReader(page))
	if err != nil || titles(got) != "Bookmarks bar [Docs & notes, Recipes [Soup]], News" {
		t.Fatalf("%s, %v", titles(got), err)
	}
	if got[0].Children[0].Added.Year() != 2023 || got[0].Children[0].URL != "https://example.com/docs" {
		t.Errorf("Docs is %+v", got[0].Children[0])
	}
	if _, err := ReadBookmarksHTML(strings.NewReader("<html>not bookmarks</html>")); err != ErrNotAFile {
		t.Errorf("a page of no bookmarks: %v", err)
	}
}
