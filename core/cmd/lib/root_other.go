//go:build !linux || android

package main

import (
	"os"
	"path/filepath"
)

// defaultRoot is the folder for the profiles, ~/.project-b; Linux's is root_linux.go's.
func defaultRoot() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".project-b")
}
