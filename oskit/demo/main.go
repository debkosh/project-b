// Command demo shows oskit's capabilities on this OS, one at a time, for a person or deskrig to
// watch: it shows a note with two buttons and prints each click, one per line, as
// "clicked <id> <action>", then quits after the first. "demo remove" takes its note away. mac/build.sh wraps it in an app bundle,
// which macOS's notification centre needs.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/debkosh/project-b/oskit/notify"
)

func main() { runMain(demo) }

func demo() {
	n := notify.New()
	// "demo remove" takes the demo's note away from the notification centre, and quits.
	if len(os.Args) > 1 && os.Args[1] == "remove" {
		n.Remove("demo")
		time.Sleep(time.Second)
		os.Exit(0)
	}
	done := make(chan struct{})
	n.OnClick(func(id, action string) {
		fmt.Printf("clicked %s %q\n", id, action)
		close(done)
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	allowed, err := notify.AskWait(ctx, n)
	fmt.Printf("allowed %v %v\n", allowed, err)
	n.Show(notify.Note{
		ID: "demo", Title: "oskit demo", Subtitle: "notify", Body: "Click me, or a button.",
		Actions: []notify.Action{{ID: "yes", Label: "Yes"}, {ID: "no", Label: "No"}},
	})
	fmt.Println("shown demo")
	select {
	case <-done:
	case <-time.After(2 * time.Minute):
		fmt.Println("no click")
		os.Exit(1)
	}
	n.Remove("demo")
	os.Exit(0)
}
