//go:build !darwin || !cgo

package browserdata

// Icon is the icon the system shows for the app at path, which needs macOS.
func Icon(string, int) ([]byte, error) { return nil, ErrNotOnThisSystem }
