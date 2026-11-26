//go:build !darwin

package main

// crashReports is "" off the Mac: Linux, Android and Windows keep no crash reports of their
// own in a folder for the crash notice to name.
func crashReports(string) string { return "" }
