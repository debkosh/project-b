package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The core's library for Android starts no program with os/exec in any package of ours:
// before its first child, Go's os package tries pidfd_open, which Android 10's seccomp
// answers with SIGSYS, and in a shared library, as the core is on a phone, that ends the
// app. Save report did so, and Settings' Relaunch would have (relaunch_android.go and
// platform_android.go's run start nothing that way). amarit's update client, which links
// it too, starts nothing on a phone.
func TestAndroidLibraryStartsNoProgramWithOsExec(t *testing.T) {
	list := exec.Command("go", "list", "-tags", "sqlite_fts5", "-deps",
		"-f", "{{.ImportPath}}{{range .Imports}} {{.}}{{end}}", ".")
	list.Env = append(os.Environ(), "GOOS=android", "GOARCH=arm64", "CGO_ENABLED=1")
	out, err := list.Output()
	if err != nil {
		t.Fatalf("go list for Android: %v", err)
	}
	ours := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasPrefix(fields[0], "projectb/") {
			continue
		}
		ours++
		for _, imp := range fields[1:] {
			if imp == "os/exec" {
				t.Errorf("%s imports os/exec on Android: a program it starts there ends the app on Android 10; give Android its own file without it", fields[0])
			}
		}
	}
	if ours < 10 {
		t.Fatalf("found %d of our packages in the Android library: the listing looks wrong", ours)
	}
}
