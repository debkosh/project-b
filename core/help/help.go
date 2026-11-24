// Package help is every page the app links to for more than it says itself, by name: the
// one place their addresses live. The pages are the repository's docs/help, as GitHub shows
// them, so a page that moves changes here once, and help_test.go fails on one that isn't
// there.
package help

// Repo is where GitHub shows the repository's files on main.
const Repo = "https://github.com/sudiptadeb/project-b/blob/main/"

// Pages is each help page by the name the app's pages ask for it by, and its file in the
// repository.
var Pages = map[string]string{
	// Why the desktop's keyring gave no key, and how to mend one that stopped answering
	// (Settings' Learn more beside the keyring's line).
	"keyring": "docs/help/keyring.md",
}

// URL is the address of the help page called name, or "" for none.
func URL(name string) string {
	if file, ok := Pages[name]; ok {
		return Repo + file
	}
	return ""
}
