//go:build !linux || android

package main

// shareClipboard is Linux's: the Mac and Android copy a secret through their own system.
func shareClipboard() {}
