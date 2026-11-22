//go:build linux && !android

package relaunch

// launcher starts the app in the Flatpak: zypak around our binary (tools/linux/project-b.sh).
// A copy the update portal starts runs it from the new release's /app.
const launcher = "/app/bin/project-b"

// Args is the command line that starts the copy on profile id again after an update, for the
// Flatpak portal's Spawn (amarit/flatpak): it starts at once, so it waits for the profile the
// copy it replaces still holds (--after-update) instead of handing itself over to it.
func (c Copies) Args(id string) []string {
	return []string{launcher, "--root=" + c.List.Root(), "--profile=" + id, "--after-update"}
}
