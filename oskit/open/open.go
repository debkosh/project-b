// Package open is opening what the app has in the OS's own apps: a file in the app for its
// type, a file shown in the file manager, and whether the app is the default browser, which
// the OS hands links to.
package open

import (
	"context"

	"github.com/sudiptadeb/oskit"
)

// Opener is the OS's way to open things.
type Opener interface {
	// OpenFile opens the file in the OS's app for it. Reveal shows it in the file manager:
	// Finder on the Mac, the Downloads app on a phone, the desktop's file manager on Linux.
	OpenFile(path string)
	Reveal(path string)
	// IsDefaultBrowser says whether the app is the default browser now. AskDefaultBrowser asks
	// the OS to make it so, which asks the person: macOS's question, Android's sheet; Linux has
	// no way, and answers no. answered runs once, through oskit.Post.
	IsDefaultBrowser() bool
	AskDefaultBrowser(answered func(isDefault bool))
	// OnOpenURLs is how links and files the OS hands a running app reach it. Set once, as the
	// app starts. Linux hands them on the command line instead, and calls it never.
	OnOpenURLs(opened func(urls []string))
}

// AskDefaultBrowserWait is AskDefaultBrowser that waits for the answer, or for ctx to end. Not
// on the main thread.
func AskDefaultBrowserWait(ctx context.Context, o Opener) (bool, error) {
	return oskit.Wait(ctx, func(answered func(bool)) func() {
		o.AskDefaultBrowser(answered)
		return nil
	})
}
