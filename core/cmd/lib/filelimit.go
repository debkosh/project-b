package main

import "syscall"

// Programs the app starts get its own limit on open files. Go raises its own at start but
// starts children with the limit it found, 256 under launchd, and they inherit the files the
// app holds open, so ditto, codesign or a relaunch helper could open none. Setting the limit,
// even to what it is, tells Go to leave it to the children as it is.
func init() {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err == nil {
		syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim)
	}
}
