package browserdata

import (
	"path/filepath"
	"strings"
	"unicode"
)

// A Chromium browser keeps its passwords' key in the login keychain as a generic password
// of service "<name> Safe Storage" and account "<name>". Chromium's own code names it so,
// "Chrome" in Google's builds and "Chromium" in others (components/os_crypt's
// keychain_password_mac.mm), and each fork puts its own name there, as a rule its
// product's, which is the name of its folder under Application Support too.

// KeychainItem is a "<name> Safe Storage" item of the login keychain: its service and its
// account, as KeychainItems lists them.
type KeychainItem struct {
	Service string `json:"service"`
	Account string `json:"account"`
}

// safeStorage is the service's ending every Chromium browser's item has.
const safeStorage = " Safe Storage"

// name is the browser's name in the item's service: "Chrome" for "Chrome Safe Storage".
func (i KeychainItem) name() string { return strings.TrimSuffix(i.Service, safeStorage) }

// SafeStorage is which of the keychain's items holds the passwords' key of a profile of a
// browser known only by its folder (OtherChromium), from those KeychainItems listed: the
// one named after its folder or the vendor's folder above it, or else one whose name its
// folder's begins, or begins with; sure when that is one item. Otherwise they are those for the person to choose from: the
// few that half match, or all of them when none does. A profile of a browser this package
// knows has its own names: none, and not sure.
func (p Profile) SafeStorage(items []KeychainItem) (choices []KeychainItem, sure bool) {
	if p.Browser != OtherChromium || len(items) == 0 {
		return nil, false
	}
	folders := chromiumFolderNames(p.Dir)
	for _, f := range folders {
		for _, name := range namesOf(f) {
			for _, i := range items {
				if strings.EqualFold(i.name(), name) {
					return []KeychainItem{i}, true
				}
			}
		}
	}
	// Brave's "Brave" for its folder BraveSoftware/Brave-Browser, Yandex's for
	// YandexBrowser, Opera's for com.operasoftware.OperaGX.
	var half []KeychainItem
	for _, i := range items {
		if halfMatches(i, folders[0]) {
			half = append(half, i)
		}
	}
	if len(half) == 1 {
		return half, true
	}
	if len(half) > 1 {
		return half, false
	}
	return items, false
}

// halfMatches says whether the item's name begins one of the browser folder's names, or
// one of them begins it, in letters and digits alone, three at least. The vendor's folder
// isn't asked: Microsoft's would match Edge's item for any browser of its.
func halfMatches(i KeychainItem, folder string) bool {
	n := squash(i.name())
	for _, name := range namesOf(folder) {
		if s := squash(name); len(n) >= 3 && len(s) >= 3 && (strings.HasPrefix(s, n) || strings.HasPrefix(n, s)) {
			return true
		}
	}
	return false
}

// chromiumFolderNames are the folders a Chromium profile's browser may be named after: the
// browser's folder (the one above the profile when that one has Local State, past "User
// Data", as Arc's is), then the vendor's above it when that one is in Application Support.
func chromiumFolderNames(dir string) []string {
	d := filepath.Clean(dir)
	if exists(filepath.Join(filepath.Dir(d), "Local State")) {
		d = filepath.Dir(d)
	}
	if filepath.Base(d) == "User Data" {
		d = filepath.Dir(d)
	}
	out := []string{filepath.Base(d)}
	if vendor := filepath.Dir(d); filepath.Base(filepath.Dir(vendor)) == "Application Support" {
		out = append(out, filepath.Base(vendor))
	}
	return out
}

// namesOf are the names a folder may stand for: itself, its last part when it is named as
// a bundle id (com.operasoftware.Opera), and with dashes and underscores as spaces
// (Brave-Browser).
func namesOf(folder string) []string {
	out := []string{folder}
	if i := strings.LastIndex(folder, "."); i >= 0 && i < len(folder)-1 {
		out = append(out, folder[i+1:])
	}
	if spaced := strings.NewReplacer("-", " ", "_", " ").Replace(folder); spaced != folder {
		out = append(out, spaced)
	}
	return out
}

// squash is a name in lower case, its letters and digits alone.
func squash(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}
