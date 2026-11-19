// Package shellfiles holds the shell's built pages inside the Go core, so the engine serves
// them from Go and the app carries no folder of them (docs/layers.md, 5).
//
// dist is filled by the build, before Go is compiled: engine/build_go.py copies the Mac's
// from $OUT/shell, and engine/android/build_go.py a phone's from shell/dist. It is
// gitignored but for .keep, so a checkout that never built the shell still compiles,
// holding no pages.
package shellfiles

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

// Read is one built file by its name under the shell's folder, as the engine asks for it:
// "settings/index.html", "shell/standard/standard.js". false when there is no such file.
func Read(name string) ([]byte, bool) {
	if !fs.ValidPath(name) || name == "." {
		return nil, false
	}
	b, err := files.ReadFile("dist/" + name)
	return b, err == nil
}
