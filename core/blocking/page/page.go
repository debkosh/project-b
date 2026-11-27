// Package page holds the blocker's script for pages: the watcher, which the engine runs in
// every frame where the blocker is on (docs/blocking.md, Parts of a page).
package page

import _ "embed"

// Watcher is watcher.js, handed to the engine with the blocker's table.
//
//go:embed watcher.js
var Watcher string
