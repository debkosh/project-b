// Package links is every web address the app opens or reads: the one place they live, so
// one that moves changes here once. Each says who uses it.
package links

const (
	// Repo is where GitHub shows the repository's files on main.
	Repo = "https://github.com/sudiptadeb/project-b/blob/main/"

	// Releases is the manifest of both channels, Stable and Dev, on the pre-release tagged
	// channels, which tools/release.sh publishes (docs/release-channels.md): every platform's
	// updater reads it. amarit.json's releases says the same (cmd/lib's main_test.go).
	Releases = "https://github.com/sudiptadeb/project-b/releases/download/channels/releases.json"

	// LinuxVersions names the Flatpak repository's commits by version, published with it on
	// Pages by tools/linux/flatpak-release.sh, in the repository's own folder: Linux's
	// updater reads it.
	LinuxVersions = "https://projectb.debkosh.com/repo/linux.json"

	// Home is the homepage: each window of the web demo (cmd/webdemo) opens on it.
	Home = "https://projectb.debkosh.com/"

	// SyncServer is the sync server to start with (docs/sync.md, "A server to start with"):
	// ours, free to try, and it can read nothing of what it keeps, as no sync server can. It
	// fills the set-up's Address, for a person to change to their own server's.
	SyncServer = "https://memd.debkosh.com/sync"
	// SyncBucket is the page where a person gets a bucket on SyncServer: Settings' "Get a free
	// sync bucket".
	SyncBucket = "https://memd.debkosh.com/synckosh"

	// WebStore is where "Chrome Web Store" in the extension manager goes. An extension is
	// installed from the store's own page.
	WebStore = "https://chromewebstore.google.com/"
)

// HelpPages is each page the app links to for more than it says itself, by the name the app's
// pages ask for it by, and its file in the repository: the repository's docs/help, as GitHub
// shows them. links_test.go fails on one that isn't there.
var HelpPages = map[string]string{
	// Why the desktop's keyring gave no key, and how to mend one that stopped answering
	// (Settings' Learn more beside the keyring's line).
	"keyring": "docs/help/keyring.md",
	// Who can read saved passwords and passkeys on Linux, where the keyring is all that
	// stands before them, and why a Mac and a phone differ (Settings' Learn more beside the
	// line that says so on Passwords and Passkeys).
	"saved-on-linux": "docs/help/saved-on-linux.md",
}

// Help is the address of the help page called name, or "" for none.
func Help(name string) string {
	if file, ok := HelpPages[name]; ok {
		return Repo + file
	}
	return ""
}
