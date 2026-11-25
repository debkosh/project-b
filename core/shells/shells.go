// Package shells is the list of shells the app is built with: each one's id, the folder
// it builds from, shell/src/apps/shell/<id>/, and the page it loads as, the name the user
// sees, and its own actions. Which one a profile loads is its shell pref (docs/shells.md).
package shells

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"projectb/engine"
)

// Shell is one shell of the registry.
type Shell struct {
	ID string `json:"id"`
	// Name and Description are what people see: in Settings, the welcome, the menus and
	// the homepage. They are written here only, so a shell is renamed here alone; the id
	// never changes, as profiles save it.
	Name        string `json:"name"`
	Description string `json:"description"`
	// Phone is a shell made for a phone: Android loads one in a narrow window, and a
	// desktop offers none of them, though it loads one named outright, as the mock does
	// for a phone-sized test.
	Phone bool `json:"phone,omitempty"`
	// Android is a desktop shell Android offers, and loads in a wide window
	// (docs/large-screens.md).
	Android bool `json:"android,omitempty"`
	// Native is a shell drawn by the platform's own views, not a web page: it has no folder
	// in shell/src/apps/shell, and its page is native/<id>, which the engine makes as that
	// platform's view (docs/native-shell.md).
	Native bool `json:"native,omitempty"`
	// Actions are the shell's own, such as the terminal's splits. Go registers each with
	// the others, enabled only while this shell is the profile's; running one sends the
	// shell action with its id. An id starts with the shell's and a dot.
	Actions []Action `json:"actions,omitempty"`
	// Takes are Go's own actions this shell runs itself while it is the profile's, by id:
	// run by a key, a menu item or anything else, one of them sends the shell action with
	// its id in place of what Go does, as the standard shell opens its box for New Tab.
	Takes []string `json:"takes,omitempty"`
}

// Action is one of a shell's own actions: its id, which never changes once released, and
// the title its menu item shows.
type Action struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// OnAndroid is whether this app runs on Android, where the window's size picks its shell:
// a phone's in a narrow window, a desktop's in a wide one (Narrow).
var OnAndroid = runtime.GOOS == "android"

// Narrow says whether a window w by h dp wide is narrow on Android: under 600 wide or 480
// tall, Android's compact width and height, so a phone on its side is narrow too. was is
// what the window was before: it changes only 40 dp past either line, so a window dragged
// to the edge doesn't flip back and forth.
func Narrow(w, h float64, was bool) bool {
	const width, height, margin = 600, 480, 40
	if was {
		return w < width+margin || h < height+margin
	}
	return w < width-margin || h < height-margin
}

//go:embed registry.json
var registry []byte

// An id is lower-case letters, digits and dashes.
var id = regexp.MustCompile(`^[a-z0-9-]+$`)

// all is the registry, in its order. It is built in, so a bad one is a bug in the build.
var all = func() []Shell {
	var out []Shell
	if err := json.Unmarshal(registry, &out); err != nil {
		panic(fmt.Sprintf("shells: registry.json: %v", err))
	}
	for _, s := range out {
		if !id.MatchString(s.ID) || s.Name == "" || s.Description == "" {
			panic(fmt.Sprintf("shells: registry.json: %+v", s))
		}
		for _, a := range s.Actions {
			if !strings.HasPrefix(a.ID, s.ID+".") || a.Title == "" {
				panic(fmt.Sprintf("shells: registry.json: %s: %+v", s.ID, a))
			}
		}
		for _, t := range s.Takes {
			if t == "" || strings.HasPrefix(t, s.ID+".") {
				panic(fmt.Sprintf("shells: registry.json: %s takes %q", s.ID, t))
			}
		}
	}
	return out
}()

// All is every shell, in the registry's order.
func All() []Shell { return append([]Shell(nil), all...) }

// Listed is the shells the user can choose for a wide window: a desktop's every desktop
// shell, Android's those it offers.
func Listed() []Shell {
	var out []Shell
	for _, s := range all {
		if !s.Phone && (s.Android || !OnAndroid) {
			out = append(out, s)
		}
	}
	return out
}

// Phones is the shells Android offers for a narrow window.
func Phones() []Shell {
	var out []Shell
	for _, s := range all {
		if s.Phone {
			out = append(out, s)
		}
	}
	return out
}

// Takes says whether the shell named id takes Go's action named action, to run it itself.
func Takes(id, action string) bool {
	for _, s := range all {
		if s.ID == id {
			return slices.Contains(s.Takes, action)
		}
	}
	return false
}

// Default is the shell a profile loads in a wide window until it chooses one: the first
// listed.
func Default() string { return Listed()[0].ID }

// DefaultPhone is the shell a profile loads in a narrow window on Android until it
// chooses one.
func DefaultPhone() string { return Phones()[0].ID }

// Fits says whether a profile whose shell pref names id loads it. A desktop loads any
// shell named outright, a phone's too; Android only one it lists, so a profile template
// that names another loads Default there.
func Fits(id string) bool {
	return slices.ContainsFunc(Listed(), func(s Shell) bool { return s.ID == id }) || (!OnAndroid && Has(id))
}

// FitsPhone says whether a profile whose phone shell pref names id loads it.
func FitsPhone(id string) bool { return Phone(id) }

// Page is the page a window loads for the shell called id: native/<id> for a native
// shell, shell/<id> for the rest.
func Page(id string) engine.Page {
	for _, s := range all {
		if s.ID == id && s.Native {
			return engine.NativeShellPage(id)
		}
	}
	return engine.ShellPage(id)
}

// Phone says whether the shell called id is made for a phone.
func Phone(id string) bool {
	for _, s := range all {
		if s.ID == id {
			return s.Phone
		}
	}
	return false
}

// Name is the name people see for the shell called id, "" for no shell.
func Name(id string) string {
	for _, s := range all {
		if s.ID == id {
			return s.Name
		}
	}
	return ""
}

// Has says whether id is a shell's.
func Has(id string) bool {
	for _, s := range all {
		if s.ID == id {
			return true
		}
	}
	return false
}
