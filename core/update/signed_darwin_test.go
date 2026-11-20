//go:build darwin

package update

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The Security framework's own answers: a bundle signed as the requirement says passes; one
// signed by someone else, or altered, is refused; one that can't be read is a check that
// couldn't be made, which refuses nothing.
func TestSignaturesAreCheckedInProcess(t *testing.T) {
	if err := checkSigned("/System/Applications/Calculator.app", "anchor apple"); err != nil {
		t.Fatalf("Apple's own app: %v", err)
	}
	// A bundle of our own, signed ad hoc: a real executable, so codesign takes it.
	app := fakeApp(t, t.TempDir(), "0.2.0")
	exe := filepath.Join(app, "Contents", "MacOS", "project_b")
	b, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, b, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", app).CombinedOutput(); err != nil {
		t.Fatalf("sign: %v %s", err, out)
	}
	if err := checkSigned(app, ""); err != nil {
		t.Fatalf("a signed bundle: %v", err)
	}
	err = checkSigned(app, `certificate leaf = H"0000000000000000000000000000000000000000"`)
	if !errors.Is(err, errRefused) || !strings.Contains(err.Error(), "isn't signed with Project B's key") {
		t.Fatalf("another key: %v", err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), append(mustRead(t, filepath.Join(app, "Contents", "Info.plist")), ' '), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkSigned(app, ""); !errors.Is(err, errRefused) || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("an altered bundle: %v", err)
	}
	if err := checkSigned(filepath.Join(t.TempDir(), "None.app"), ""); err == nil || errors.Is(err, errRefused) {
		t.Fatalf("a bundle that isn't there must be a check that couldn't be made: %v", err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
