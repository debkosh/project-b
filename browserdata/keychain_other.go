//go:build !darwin || !cgo

package browserdata

// KeychainSecret needs macOS.
func KeychainSecret(string, string) ([]byte, error) { return nil, ErrNotOnThisSystem }
