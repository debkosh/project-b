//go:build linux && !android

package main

import "projectb/system/platform"

// shareClipboard gives the platform the engine's clipboard, which Linux's CopySecret writes
// through: a password copied from Settings, or a backup code.
func shareClipboard() { platform.New().UseClipboard(native.WriteClipboard) }
