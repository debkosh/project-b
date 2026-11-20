package update

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// BundleOf returns the .app the executable runs from, or "" when it runs from a build
// directory.
func BundleOf(executable string) string {
	for d := filepath.Dir(executable); d != "/" && d != "."; d = filepath.Dir(d) {
		if strings.HasSuffix(d, ".app") {
			return d
		}
	}
	return ""
}

// Version returns the version the installed bundle says it is.
func Version(app string) (string, error) {
	v, _, err := bundleInfo(app)
	return v, err
}

// bundleInfo reads what the bundle says about itself in Contents/Info.plist: its version
// and the name of its executable. The plist is the one tools/package.sh writes.
func bundleInfo(app string) (version, executable string, err error) {
	f, err := os.Open(filepath.Join(app, "Contents", "Info.plist"))
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	values := map[string]string{}
	dec := xml.NewDecoder(f)
	key := ""
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", fmt.Errorf("Info.plist: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		var text string
		switch se.Name.Local {
		case "key":
			dec.DecodeElement(&text, &se)
			key = text
		case "string":
			dec.DecodeElement(&text, &se)
			if key != "" {
				values[key] = text
			}
			key = ""
		default:
			key = ""
		}
	}
	version, executable = values["CFBundleShortVersionString"], values["CFBundleExecutable"]
	if version == "" || executable == "" {
		return "", "", errors.New("Info.plist has no version or executable")
	}
	return version, executable, nil
}

// unpack extracts the archive into dir with ditto, which keeps what the signature covers,
// and returns the one bundle inside.
func unpack(archive, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	keepFileLimit()
	if out, err := exec.Command("/usr/bin/ditto", "-x", "-k", archive, dir).CombinedOutput(); err != nil {
		return "", fmt.Errorf("unpack: %s", firstLine(out, err))
	}
	apps, _ := filepath.Glob(filepath.Join(dir, "*.app"))
	if len(apps) != 1 {
		return "", fmt.Errorf("the archive holds %d apps, not one", len(apps))
	}
	return apps[0], nil
}

// keepFileLimit gives the programs the app starts its own limit on open files. Go raises
// its own at start but starts children with the limit it found, 256 under launchd, and a
// child of the app also inherits the files the app holds open, which can be most of that:
// ditto then failed with "Too many open files". Setting the limit, even to what it is,
// tells Go to leave it to the children as it is.
var keepFileLimit = sync.OnceFunc(func() {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err == nil {
		syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim)
	}
})

func firstLine(out []byte, err error) string {
	if line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n"); line != "" {
		return line
	}
	return err.Error()
}
