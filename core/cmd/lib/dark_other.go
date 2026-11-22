//go:build !linux || android

package main

// followDark is Linux's: the Mac's and Android's engines follow their own system's dark mode.
func followDark() {}
