package app_test

import (
	"path/filepath"
	"testing"

	"projectb/engine"
)

// Save to: Change… asks the engine for a folder on the settings tab's window, and the one
// chosen is where downloads go from then on, kept in the prefs; a panel closed with none,
// or a path that is no folder, changes nothing; and the Downloads folder comes back when
// the pref is "".
func TestSettingsChoosesTheDownloadFolder(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	standard := filepath.Join(dir, "Downloads")
	_, snap := r.openSettings()
	if snap["downloadsFolder"] != standard {
		t.Fatalf("the snapshot's Downloads folder is %v", snap["downloadsFolder"])
	}
	target := func() (path string) {
		r.main(func() {
			path = r.a.Downloads.Target(engine.DownloadRequest{ID: 1, URL: "https://a.test/f.zip", Name: "f.zip"}).Path
		})
		return filepath.Dir(path)
	}
	if target() != standard {
		t.Fatalf("a download goes to %s", target())
	}

	chosen := t.TempDir()
	r.e.Fire(1, 0, "choose", chosen)
	r.settings(`{"t":"chooseDownloadFolder"}`)
	r.e.Settle()
	if target() != chosen {
		t.Fatalf("after Change…, a download goes to %s", target())
	}
	var pref string
	r.main(func() { pref = r.a.Prefs.All().DownloadFolder })
	if pref != chosen {
		t.Fatalf("the pref is %q", pref)
	}

	// None chosen, and no folder, change nothing.
	r.settings(`{"t":"chooseDownloadFolder"}`)
	r.e.Fire(1, 0, "choose", filepath.Join(chosen, "gone"))
	r.settings(`{"t":"chooseDownloadFolder"}`)
	r.e.Settle()
	if target() != chosen {
		t.Fatalf("a download goes to %s", target())
	}

	r.settings(`{"t":"setPref","name":"downloadFolder","value":""}`)
	r.e.Settle()
	if target() != standard {
		t.Fatalf("with the pref \"\", a download goes to %s", target())
	}
}
