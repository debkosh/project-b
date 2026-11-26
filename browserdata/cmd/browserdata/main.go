// Command browserdata prints what the package finds, for checking it against a real
// browser: each browser and profile with what it holds, or one profile in full as JSON.
//
//	browserdata                       the browsers, their profiles and counts
//	browserdata -dump chrome/Default  one profile, by its browser and folder, as JSON
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/debkosh/project-b/browserdata"
)

func main() {
	userHome, _ := os.UserHomeDir()
	home := flag.String("home", userHome, "the home folder to look in")
	dump := flag.String("dump", "", "a profile to print in full: browser/folder")
	flag.Parse()
	log.SetFlags(0)
	browsers := browserdata.Find(*home)

	switch {
	case *dump != "":
		for _, b := range browsers {
			for _, p := range b.Profiles {
				if b.ID+"/"+filepath.Base(p.Dir) != *dump {
					continue
				}
				bookmarks, berr := p.Bookmarks()
				history, herr := p.History()
				extensions, eerr := p.Extensions()
				out := map[string]any{"profile": p, "bookmarks": bookmarks, "history": history, "extensions": extensions}
				for k, err := range map[string]error{"bookmarksError": berr, "historyError": herr, "extensionsError": eerr} {
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
			fmt.Println(b.Name)
			for _, p := range b.Profiles {
				c, err := p.Counts()
				fmt.Printf("  %s/%s %q: %d bookmarks in %d folders, %d pages, %d extensions", b.ID, filepath.Base(p.Dir), p.Name, c.Bookmarks, c.Folders, c.Pages, c.Extensions)
				if err != nil {
					fmt.Printf(" (%v)", err)
				}
				fmt.Println()
			}
		}
	}
}
