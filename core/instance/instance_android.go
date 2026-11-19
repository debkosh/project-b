package instance

// Owner is nothing on Android, where the system runs one copy of the app. A link another app
// sends reaches the app through core/system.
type Owner struct{}

// Claim always owns the folder: Android starts no second copy.
func Claim(dir string, urls []string) (*Owner, error) { return &Owner{}, nil }

// Serve has no other copy to serve.
func (o *Owner) Serve(handover func(urls []string), quit func()) {}

// Quit finds no other copy.
func Quit(dir string) (int, error) { return 0, nil }

// Close has nothing to give up.
func (o *Owner) Close() {}
