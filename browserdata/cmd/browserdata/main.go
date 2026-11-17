// Command browserdata prints what the package finds, for checking it against a real
// browser: each browser and profile with what it holds, or one profile in full as JSON.
//
//	browserdata                       the browsers, their profiles and counts
//	browserdata -dump chrome/Default  one profile, by its browser and folder, as JSON
//	browserdata -icon out.png chrome  the browser's icon, 128 pixels square
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sudiptadeb/project-b/browserdata"
)

func main() {
	userHome, _ := os.UserHomeDir()
	home := flag.String("home", userHome, "the home folder to look in")
	dump := flag.String("dump", "", "a profile to print in full: browser/folder")
	icon := flag.String("icon", "", "write the icon of the browser named after the flags to this PNG")
	flag.Parse()
	log.SetFlags(0)
	browsers := browserdata.Find(*home)

	switch {
	case *icon != "":
		for _, b := range browsers {
			if b.ID == flag.Arg(0) {
				png, err := browserdata.Icon(b.App, 128)
				if err != nil {
					log.Fatal(err)
				}
				if err := os.WriteFile(*icon, png, 0o644); err != nil {
					log.Fatal(err)
				}
				return
			}
		}
		log.Fatalf("no browser %q found", flag.Arg(0))
	case *dump != "":
		for _, b := range browsers {
			for _, p := range b.Profiles {
				if b.ID+"/"+filepath.Base(p.Dir) != *dump {
					continue
				}
				bookmarks, berr := p.Bookmarks()
				history, herr := p.History()
				search, serr := p.Search()
				out := map[string]any{"profile": p, "bookmarks": bookmarks, "history": history, "search": search}
				for k, err := range map[string]error{"bookmarksError": berr, "historyError": herr, "searchError": serr} {
					if err != nil {
						out[k] = err.Error()
					}
				}
				e := json.NewEncoder(os.Stdout)
				e.SetIndent("", "  ")
				e.Encode(out)
				return
			}
		}
		log.Fatalf("no profile %q found", *dump)
	default:
		for _, b := range browsers {
			fmt.Printf("%s (%s)\n", b.Name, b.App)
			for _, p := range b.Profiles {
				c, err := p.Counts()
				fmt.Printf("  %s/%s %q: %d bookmarks in %d folders, %d pages, search %s", b.ID, filepath.Base(p.Dir), p.Name, c.Bookmarks, c.Folders, c.Pages, c.Search)
				if err != nil {
					fmt.Printf(" (%v)", err)
				}
				fmt.Println()
			}
		}
	}
}
