package instance

// Owner is nothing on Android, where the system runs one copy of the app, but it keeps what
// a desktop copy serves to others: the handover, for links other apps send.
type Owner struct {
	handover func(urls []string)
}

// Claim always owns the folder: Android starts no second copy.
func Claim(dir string, urls []string) (*Owner, error) { return &Owner{}, nil }

// Serve keeps handover. Android hands a link from another app to the running activity
// rather than to a second copy, and the activity passes it to Open.
func (o *Owner) Serve(handover func(urls []string), quit func()) { o.handover = handover }

// Open hands urls over as a second copy's are on a desktop.
func (o *Owner) Open(urls []string) {
	if o.handover != nil {
		o.handover(urls)
	}
}

// Quit finds no other copy.
func Quit(dir string) (int, error) { return 0, nil }

// Close has nothing to give up.
func (o *Owner) Close() {}
