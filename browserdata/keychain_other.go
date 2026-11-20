//go:build !darwin || !cgo

package browserdata

// KeychainSecret needs macOS.
func KeychainSecret(string, string) ([]byte, error) { return nil, ErrNotOnThisSystem }

// KeychainItems needs macOS.
func KeychainItems() ([]KeychainItem, error) { return nil, ErrNotOnThisSystem }
