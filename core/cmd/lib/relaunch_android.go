package main

import (
	"projectb/relaunch"
	"projectb/system/platform"
)

// Settings' Relaunch on a phone: Android starts the app again once it has quit.
func init() { relaunch.OnSelf(platform.RelaunchAfterQuit) }
