package shelldata

import (
	"testing"
)

func TestKeepsEachShellsDataAcrossManagers(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if got := string(m.Get("arshell")); got != "null" {
		t.Fatalf("nothing saved should be null, got %s", got)
	}
	if err := m.Set("arshell", []byte(`{"favorites":{"":[{"url":"https://a.test/"}]}}`)); err != nil {
		t.Fatal(err)
	}
	if err := m.Set("tmshell", []byte(`[1]`)); err != nil {
		t.Fatal(err)
	}
	again := New(dir)
	if got := string(again.Get("arshell")); got != `{"favorites":{"":[{"url":"https://a.test/"}]}}` {
		t.Fatalf("arshell: %s", got)
	}
	if got := string(again.Get("tmshell")); got != `[1]` {
		t.Fatalf("tmshell: %s", got)
	}
	if err := m.Set("arshell", []byte(`{nope`)); err == nil {
		t.Fatal("bad JSON should be refused")
	}
}
