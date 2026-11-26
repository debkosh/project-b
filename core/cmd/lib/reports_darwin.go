package main

import "path/filepath"

// crashReports is the folder macOS writes its crash reports to, which the crash notice names.
func crashReports(home string) string { return filepath.Join(home, "Library/Logs/DiagnosticReports") }
