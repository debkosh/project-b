//go:build !android

package open

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/sudiptadeb/oskit"
	"github.com/sudiptadeb/oskit/internal/portal"
)

// desktopID is the app's desktop file, by which mimeapps.list names it, as SetDesktopID gave.
var desktopID string

// SetDesktopID names the app's desktop file, "<app id>.desktop", for a copy outside a Flatpak;
// inside one it is the Flatpak's id.
func SetDesktopID(id string) { desktopID = id }

func desktopFile() string {
	if id := os.Getenv("FLATPAK_ID"); id != "" {
		return id + ".desktop"
	}
	return desktopID
}

// linux opens through the OpenURI portal, handing it the file itself, open, so the desktop
// reaches it from inside a Flatpak's sandbox. The portal's answer, whether the person chose an
// app, nobody waits for. No portal sets the default browser: the person does, in the desktop's
// settings, which write mimeapps.list.
type linux struct{}

// New is this OS's Opener.
func New() Opener { return linux{} }

func (linux) OpenFile(path string) { go openURI("OpenFile", path) }

// Reveal is OpenURI's OpenDirectory: the file's folder, the file selected.
func (linux) Reveal(path string) { go openURI("OpenDirectory", path) }

func openURI(method, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	_, p, err := portal.Desktop()
	if err != nil {
		return
	}
	p.Call("org.freedesktop.portal.OpenURI."+method, 0, "", dbus.UnixFD(f.Fd()), map[string]dbus.Variant{})
}

// IsDefaultBrowser is whether the desktop opens https links with the app: the first app
// mimeapps.list's Default Applications give x-scheme-handler/https, where GNOME's and KDE's
// settings set the web browser. A Flatpak needs to read that one file of the person's settings
// (--filesystem=xdg-config/mimeapps.list:ro), at the path it has outside.
func (linux) IsDefaultBrowser() bool {
	list, err := os.ReadFile(filepath.Join(configHome(), "mimeapps.list"))
	app := defaultApp(string(list), "x-scheme-handler/https")
	return err == nil && app != "" && app == desktopFile()
}

// AskDefaultBrowser answers what is so, later, as the other OSes do: the app can't make itself
// the default on Linux.
func (l linux) AskDefaultBrowser(answered func(bool)) {
	go func() {
		is := l.IsDefaultBrowser()
		oskit.Post(func() { answered(is) })
	}()
}

// OnOpenURLs: the desktop starts the app with links and files on its command line, and a copy
// already running gets them from the next one, so nothing comes this way.
func (linux) OnOpenURLs(func([]string)) {}

// configHome is the person's settings folder outside the Flatpak, whose own XDG_CONFIG_HOME is
// the app's folder in ~/.var/app; Flatpak gives the outside one as HOST_XDG_CONFIG_HOME.
func configHome() string {
	if dir := os.Getenv("HOST_XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" && os.Getenv("FLATPAK_ID") == "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// defaultApp is the desktop file a mimeapps.list makes the default for the type: the first of
// those its Default Applications section lists for it, or "".
func defaultApp(list, mimeType string) string {
	section := ""
	for _, line := range strings.Split(list, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || section != "Default Applications" || strings.TrimSpace(key) != mimeType {
			continue
		}
		first, _, _ := strings.Cut(value, ";")
		return strings.TrimSpace(first)
	}
	return ""
}
