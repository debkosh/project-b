// Command expect writes testdata/expect.json: what each MP4 named holds, by its file name
// without ".mp4", as mp4check reads it. testdata/make.sh runs it over what ffmpeg makes of
// each test stream.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"projectb/streams/mp4check"
)

func main() {
	out := map[string]*mp4check.File{}
	for _, path := range os.Args[1:] {
		f, err := mp4check.Read(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, path+":", err)
			os.Exit(1)
		}
		out[strings.TrimSuffix(filepath.Base(path), ".mp4")] = f
	}
	b, _ := json.MarshalIndent(out, "", " ")
	os.Stdout.Write(append(b, '\n'))
}
