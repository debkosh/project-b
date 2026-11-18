package browserdata

import (
	"errors"
	"time"
)

// Login is a saved password: the site's origin (scheme://host[:port]), the username and
// the password, and when it was saved and last used, zero when unknown. What reads it keeps
// it in memory only.
type Login struct {
	Origin   string
	Username string
	Password string
	Created  time.Time
	LastUsed time.Time
}

// Keys is what unlocks a profile's saved passwords. Secret reads a Chromium browser's
// "<name> Safe Storage" from the login keychain, which macOS asks the person about
// (KeychainSecret); PrimaryPassword is Firefox's, "" when none is set.
type Keys struct {
	Secret          func(service, account string) ([]byte, error)
	PrimaryPassword string
}

var (
	// ErrPrimaryPassword is what Logins says of a Firefox profile whose primary password
	// was not given, or not this one.
	ErrPrimaryPassword = errors.New("browserdata: the primary password is needed, or isn't this one")
	// ErrNoSecret is what Logins says when the keychain has no key for the browser, or the
	// person refused it.
	ErrNoSecret = errors.New("browserdata: the keychain gave no key for this browser")
)

// Logins reads the profile's saved passwords: a Chromium browser's from Login Data and Login
// Data For Account, decrypted with the key keys.Secret gives; a Firefox one's from
// logins.json, with key4.db's key, unlocked by keys.PrimaryPassword. What isn't a site's
// login (Chromium's "never save" entries, federated ones) is left out. Safari's are in
// iCloud Keychain, which no other app can read: none.
func (p Profile) Logins(keys Keys) ([]Login, error) {
	k, ok := kindOf(p.Browser)
	if !ok {
		return nil, errors.New("browserdata: no browser " + p.Browser)
	}
	switch k.family {
	case chromium:
		return chromiumLogins(p.Dir, k, keys)
	case firefox:
		return firefoxLogins(p.Dir, keys.PrimaryPassword)
	}
	return nil, nil
}

// HasPrimaryPassword says whether the profile is Firefox's and its passwords are behind a
// primary password, which the person must give to bring them over.
func (p Profile) HasPrimaryPassword() bool {
	k, ok := kindOf(p.Browser)
	if !ok || k.family != firefox {
		return false
	}
	_, err := firefoxKey(p.Dir, "")
	return errors.Is(err, ErrPrimaryPassword)
}

// UsesKeychain says whether reading the profile's passwords asks the login keychain for
// their key, which macOS asks the person about: a Chromium browser's.
func (p Profile) UsesKeychain() bool {
	k, ok := kindOf(p.Browser)
	return ok && k.family == chromium
}

// countLogins is how many saved passwords the profile has, without unlocking any.
func countLogins(p Profile) (int, error) {
	k, _ := kindOf(p.Browser)
	switch k.family {
	case chromium:
		return chromiumCountLogins(p.Dir)
	case firefox:
		return firefoxCountLogins(p.Dir)
	}
	return 0, nil
}
