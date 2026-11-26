package platform

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The Go core's Android code starts no program with os/exec, nor os.StartProcess, which it
// uses: before its first child, Go's os package tries pidfd_open, Android 10's seccomp
// answers with SIGSYS, and in a shared library, as the core is on a phone, that ends the app.
// Save report did so on Android 10 (run, in platform_android.go, starts the phone's programs
// instead).
func TestAndroidCodeStartsNoProgramWithOsExec(t *testing.T) {
	core, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	err = filepath.WalkDir(core, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, "_android.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		checked++
		for _, spec := range f.Imports {
			if name, _ := strconv.Unquote(spec.Path.Value); name == "os/exec" {
				rel, _ := filepath.Rel(core, path)
				t.Errorf("%s imports os/exec: on Android 10 a program it starts ends the app; start it with syscall.ForkExec, as platform_android.go's run does", rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("found no _android.go file under core: the walk looks in the wrong place")
	}
}
