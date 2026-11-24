//go:build !linux || android

package main

// noKeyring is Linux's: the Mac's keychain and Android's Keystore have no such answer.
func noKeyring(error) bool { return false }
