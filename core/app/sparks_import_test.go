package app_test

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func sparkSource(name, match string) string {
	return "// ==UserScript==\n// @name " + name + "\n// @match " + match + "\n// ==/UserScript==\n"
}

// Settings' Import sends the files picked, Go reads the sparks in them and asks about any
// the profile already has; the answer keeps them and says what was done. Export all sends
// back a zip of every spark.
func TestSettingsImportsAndExportsSparks(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	got, _ := r.openSettings()
	r.main(func() {
		// A profile with only its own sparks: the built-in ones deleted.
		for _, s := range r.a.Sparks.Builtins() {
			r.a.Sparks.Remove(s.ID)
		}
		r.a.Sparks.Add(sparkSource("Kept", "https://a.test/*"))
	})

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, src := range map[string]string{
		"kept.user.js":         sparkSource("Kept", "https://b.test/*"),
		"__MACOSX/._x.user.js": "junk",
	} {
		f, _ := w.Create(name)
		f.Write([]byte(src))
	}
	w.Close()
	msg, _ := json.Marshal(map[string]any{"t": "sparkImport", "files": []map[string]string{
		{"name": "New.user.js", "data": base64.StdEncoding.EncodeToString([]byte(sparkSource("New", "<all_urls>")))},
		{"name": "Sparks.zip", "data": base64.StdEncoding.EncodeToString(buf.Bytes())},
	}})
	r.settings(string(msg))
	clashes := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "sparkClashes" })["clashes"].([]any)
	clash := clashes[0].(map[string]any)
	if len(clashes) != 1 || clash["name"] != "Kept" || clash["renameTo"] != "Kept 2" || clash["file"] != "Sparks.zip › kept.user.js" {
		t.Fatalf("clashes %v", clashes)
	}
	r.settings(`{"t":"sparkImportKeep","choices":{"1":"rename"}}`)
	done := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "sparkImported" })["imported"].(map[string]any)
	if b, _ := json.Marshal(done); string(b) != `{"added":["New"],"ignored":[],"renamed":[{"from":"Kept","to":"Kept 2"}],"replaced":[],"skipped":[],"unchanged":[]}` {
		t.Fatalf("imported %s", b)
	}
	var names []string
	r.main(func() {
		for _, s := range r.a.Sparks.List() {
			names = append(names, s.Name)
		}
	})
	if len(names) != 3 || names[1] != "New" || names[2] != "Kept 2" {
		t.Fatalf("sparks %v", names)
	}

	r.settings(`{"t":"sparkExport"}`)
	z := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "sparkZip" })
	data, _ := base64.StdEncoding.DecodeString(z["data"].(string))
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || z["count"] != 3.0 || len(zr.File) != 3 || zr.File[2].Name != "Kept 2.user.js" {
		t.Fatalf("zip %v %v", z["count"], err)
	}
}
