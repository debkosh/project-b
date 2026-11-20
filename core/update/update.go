// Package update keeps the installed app current through amarit's updater
// (github.com/sudiptadeb/amarit/appupdate): it checks the release manifest on a timer,
// downloads with progress, checks the release, stages it, and installs it when told. The
// installer is the platform's: a signed bundle swapped in place on a Mac (macapp), the
// system's package installer on Android. This package is what the app's pages see of it:
// its state in the words a person reads, and the commands they send.
//
// The update folder is shared by every copy of the app on one root: the staged release,
// the settings (automatic updates and the channel), and a lock one copy at a time holds
// while it checks and downloads.
package update

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sudiptadeb/amarit"
	"github.com/sudiptadeb/amarit/appupdate"

	"projectb/timeline"
)

// CheckInterval is the time between automatic checks.
const CheckInterval = time.Minute

// The channels a copy can follow (docs/release-channels.md): Stable is each release made by
// the full run, Dev each finished branch as it lands.
const (
	Stable = "stable"
	Dev    = "dev"
)

// Phases of an update, as the pages show them. Idle is empty, so an idle state has nothing
// to show. A release being checked after its download shows as Downloading, at its end.
const (
	Idle        = ""
	Downloading = "downloading"
	Ready       = "ready"
	Failed      = "failed"
)

// What a check the person asked for found, beside the phase. Nothing is empty.
const (
	Checking    = "checking"
	UpToDate    = "current"
	CheckFailed = "failed"
)

// State is what the shell shows: the bubble, and the settings panel.
type State struct {
	Phase   string `json:"phase"`
	Version string `json:"version,omitempty"` // the release being downloaded, ready, or failed
	Error   string `json:"error,omitempty"`   // why it failed, in words a person can read
	// Retry says whether Try again can help a failure; one that can't waits for a newer
	// release, or for Check for updates.
	Retry bool `json:"retry,omitempty"`
	// Done and Total are how much of the release has downloaded, in bytes, while Downloading.
	Done  int64 `json:"done,omitempty"`
	Total int64 `json:"total,omitempty"`
	// Check is how a check the person asked for went: Checking while it runs, then UpToDate,
	// or CheckFailed with CheckError. A check that finds a release moves Phase instead. Only
	// the settings panel shows these; the bubble shows Phase alone.
	Check      string `json:"check,omitempty"`
	CheckError string `json:"checkError,omitempty"`
	// Notes are Version's release notes, as markdown, once fetched, while it downloads or is
	// ready: About shows them as "Coming in Version".
	Notes string `json:"notes,omitempty"`
}

// Config says what this build is and where it lives.
type Config struct {
	Project  string // the manifest's project name
	Version  string // the running version; empty is a development build, which never updates
	Releases string // the manifest URL
	// AutoUpdate is the project's word that copies check on their own (amarit.json's
	// auto_update); a person's switch wins over it.
	AutoUpdate bool
	// StateDir is the update folder: the root's, shared by every copy on the root, or for a
	// copy on a folder named outright, that folder's own.
	StateDir string
	// Installer makes the platform's installer, given how to end the app, which it calls
	// once a bundle is in place. Nil: this build doesn't update.
	Installer func(quit func()) appupdate.Installer
	// Settle, when set, counts this start for the rollback, early: true means the previous
	// release was put back and started, and this process should end.
	Settle func(*amarit.Client) (bool, error)
	// Present, when set, shows the updater the system's way too: the Mac's Dock and
	// notification.
	Present func(*appupdate.Updater)
}

// Manager is the app's updater. Its commands may be sent from any thread; its listeners
// are called on the main thread.
type Manager struct {
	cfg    Config
	post   func(func())
	client *amarit.Client
	u      *appupdate.Updater

	mu    sync.Mutex
	state State
	raw   appupdate.State
	// notes are a known release's notes, fetched once per version (notesFor).
	notes, notesFor string

	onChanged        []func(State)
	onAutoChanged    []func(bool)
	onChannelChanged []func(string)
}

// New returns a manager. post runs a function on the main thread; quit ends the app.
func New(cfg Config, post func(func()), quit func()) *Manager {
	m := &Manager{cfg: cfg, post: post}
	// The client keeps the settings every copy shares, a development build's too; only a
	// release with an installer checks and installs.
	// A development build has no version: its client keeps the settings and never checks.
	opts := []amarit.Option{amarit.ProjectFile(projectFile(cfg))}
	if cfg.Version != "" {
		opts = append(opts, amarit.Version(cfg.Version))
	}
	c, err := amarit.NewClient(filepath.Join(cfg.StateDir, "amarit"), Stable, opts...)
	if err != nil {
		timeline.Errorf("update: %v; updates are off", err)
		return m
	}
	m.client = c
	m.raw = appupdate.State{Channel: c.Channel(), AutoUpdate: c.AutoUpdate()}
	if cfg.Version == "" || cfg.Installer == nil {
		return m
	}
	m.u = appupdate.New(appupdate.Config{
		Client:    c,
		Installer: cfg.Installer(func() { post(quit) }),
		Dir:       cfg.StateDir,
		Interval:  CheckInterval,
		Logf:      timeline.Infof,
	})
	m.raw = m.u.State()
	m.state = words(m.raw)
	m.u.OnChange(func(s appupdate.State) { post(func() { m.changed(s) }) })
	if cfg.Present != nil {
		cfg.Present(m.u)
	}
	return m
}

// projectFile is amarit.json as amarit reads it: the project, where its manifest is, and
// whether copies check on their own.
func projectFile(cfg Config) []byte {
	// A configuration with no project or manifest, a test's, still names them: amarit wants
	// both, and a client with no version never reads the address.
	project, releases := cfg.Project, cfg.Releases
	if project == "" {
		project = "project-b"
	}
	if releases == "" {
		releases = "https://example.invalid/releases.json"
	}
	b, _ := json.Marshal(map[string]any{"project": project, "releases": releases, "auto_update": cfg.AutoUpdate})
	return b
}

// Settle counts this start for the rollback: true means the release before this one was
// put back and started, and the app should end now. Early in the app's start.
func (m *Manager) Settle() bool {
	if m.client == nil || m.cfg.Settle == nil {
		return false
	}
	back, err := m.cfg.Settle(m.client)
	if err != nil {
		timeline.Errorf("update: %v", err)
	}
	if back {
		timeline.Infof("update: %s didn't start; the release before it is back", m.cfg.Version)
	}
	return back
}

// Healthy says this release works, once a window is up: its starts stop counting.
func (m *Manager) Healthy() {
	if m.client == nil {
		return
	}
	if err := m.client.Healthy(); err != nil {
		timeline.Errorf("update: %v", err)
	}
}

func (m *Manager) changed(s appupdate.State) {
	m.mu.Lock()
	was := m.raw
	m.raw, m.state = s, words(s)
	coming := m.state.Phase == Downloading || m.state.Phase == Ready
	if coming && m.notesFor == m.state.Version {
		m.state.Notes = m.notes
	}
	fetch := coming && m.notesFor != m.state.Version
	if fetch {
		m.notesFor = m.state.Version
	}
	st := m.state
	m.mu.Unlock()
	if fetch {
		go m.fetchNotes(st.Version)
	}
	if s.AutoUpdate != was.AutoUpdate {
		for _, f := range m.onAutoChanged {
			f(s.AutoUpdate)
		}
	}
	if s.Channel != was.Channel {
		for _, f := range m.onChannelChanged {
			f(s.Channel)
		}
	}
	for _, f := range m.onChanged {
		f(st)
	}
}

// notesURL is where a release's notes are: beside the manifest, by version (tools/release.sh).
func (m *Manager) notesURL(version string) string {
	if !strings.HasSuffix(m.cfg.Releases, "/releases.json") {
		return ""
	}
	return strings.TrimSuffix(m.cfg.Releases, "releases.json") + "notes-" + version + ".md"
}

// fetchNotes gets a known release's notes, and shows them once they arrive. A release with
// none, or a failure to get them, shows no "Coming in": nothing else depends on them.
func (m *Manager) fetchNotes(version string) {
	url := m.notesURL(version)
	if url == "" {
		return
	}
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		timeline.Infof("update: %s's notes: %v", version, err)
		return
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil || resp.StatusCode != http.StatusOK {
		timeline.Infof("update: %s's notes: %s", version, resp.Status)
		return
	}
	m.post(func() {
		m.mu.Lock()
		if m.notesFor != version {
			m.mu.Unlock()
			return
		}
		m.notes = string(b)
		raw := m.raw
		m.mu.Unlock()
		m.changed(raw)
	})
}

// State returns what is happening now.
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// Running is the version this build is: empty for a development build.
func (m *Manager) Running() string { return m.cfg.Version }

// Auto says whether copies check for updates on their own. Channel is the channel they
// follow.
// Each reads the folder, so another copy's change holds here at once.
func (m *Manager) Auto() bool {
	if m.client == nil {
		return m.cfg.AutoUpdate
	}
	return m.client.AutoUpdate()
}

func (m *Manager) Channel() string {
	if m.client == nil || m.client.Channel() == "" {
		return Stable
	}
	return m.client.Channel()
}

// OnChanged, OnAutoChanged and OnChannelChanged subscribe to the state, automatic updates
// and the channel changing. Called on the main thread.
func (m *Manager) OnChanged(f func(State))    { m.onChanged = append(m.onChanged, f) }
func (m *Manager) OnAutoChanged(f func(bool)) { m.onAutoChanged = append(m.onAutoChanged, f) }
func (m *Manager) OnChannelChanged(f func(string)) {
	m.onChannelChanged = append(m.onChannelChanged, f)
}

// Start begins the checks; Stop ends them. Close ends them and any work running, and returns
// once a download has stopped and its files are gone; the app calls it as it quits.
func (m *Manager) Start() {
	if m.u != nil {
		timeline.Infof("update: %s %s", m.cfg.Project, m.cfg.Version)
		m.u.Start()
	}
}

func (m *Manager) Stop() {
	if m.u != nil {
		m.u.Stop()
	}
}

func (m *Manager) Close() {
	if m.u != nil {
		m.u.Close()
	}
}

// CheckNow checks at once, whether or not automatic checks are on, and tries again a
// release refused or cancelled before.
func (m *Manager) CheckNow() {
	if m.u == nil {
		m.mu.Lock()
		m.state.Check, m.state.CheckError = CheckFailed, "a development build doesn't update"
		st := m.state
		m.mu.Unlock()
		for _, f := range m.onChanged {
			f(st)
		}
		return
	}
	m.u.Check()
}

// Retry, Cancel and Relaunch are the person's buttons; each does nothing in a phase that
// doesn't take it.
func (m *Manager) Retry() {
	if m.u != nil {
		m.u.Retry()
	}
}

func (m *Manager) Cancel() {
	if m.u != nil {
		m.u.Cancel()
	}
}

func (m *Manager) Relaunch() {
	if m.u != nil {
		m.u.Relaunch()
	}
}

// SetAuto turns automatic checks on or off, and SetChannel picks the channel, for every
// copy sharing the folder.
// A development build keeps them too, through the client alone, as it has no updater to
// tell.
func (m *Manager) SetAuto(on bool) {
	switch {
	case m.u != nil:
		if err := m.u.SetAutoUpdate(on); err != nil {
			timeline.Errorf("update: %v", err)
		}
	case m.client != nil:
		if err := m.client.SetAutoUpdate(on); err != nil {
			timeline.Errorf("update: %v", err)
		}
		m.settingsChanged()
	}
}

func (m *Manager) SetChannel(ch string) {
	if ch != Stable && ch != Dev {
		return
	}
	switch {
	case m.u != nil:
		if err := m.u.SetChannel(ch); err != nil {
			timeline.Errorf("update: %v", err)
		}
	case m.client != nil:
		if err := m.client.SetChannel(ch); err != nil {
			timeline.Errorf("update: %v", err)
		}
		m.settingsChanged()
	}
}

// settingsChanged tells the listeners of a setting changed with no updater to say so.
func (m *Manager) settingsChanged() {
	m.mu.Lock()
	s := m.raw
	m.mu.Unlock()
	s.Channel, s.AutoUpdate = m.client.Channel(), m.client.AutoUpdate()
	m.changed(s)
}

// words is amarit's state as the pages show it, a failure in project-b's words. A check the
// person asked for that couldn't read the manifest, or found another copy's download still
// stopping, is the check's answer, which the bubble doesn't show.
func words(s appupdate.State) State {
	st := State{Version: s.Version}
	switch s.Phase {
	case appupdate.Checking:
		st.Check = Checking
	case appupdate.UpToDate:
		st.Check = UpToDate
	case appupdate.Downloading:
		st.Phase, st.Done, st.Total = Downloading, s.Done, s.Total
	case appupdate.Verifying:
		st.Phase, st.Done, st.Total = Downloading, s.Total, s.Total
	case appupdate.Ready:
		st.Phase = Ready
	case appupdate.Failed:
		if s.Kind == appupdate.Offline || s.Kind == appupdate.Busy {
			st.Version = ""
			st.Check, st.CheckError = CheckFailed, why(s)
			break
		}
		st.Phase, st.Error, st.Retry = Failed, why(s), s.Kind.Retryable()
	}
	return st
}

// why is a failure in words a person reads; the error itself goes to the timeline.
func why(s appupdate.State) string {
	switch s.Kind {
	case appupdate.Offline:
		return "the update list couldn't be read: " + s.Error
	case appupdate.Network:
		return "the download failed (" + s.Error + "); Try again"
	case appupdate.Stalled:
		return "the download stopped arriving; Try again"
	case appupdate.Damaged:
		return "the download arrived damaged: it didn't match its checksum; Try again"
	case appupdate.Unchecked:
		return "its signature couldn't be checked (" + s.Error + "); Try again"
	case appupdate.Refused:
		return refused(s.Error) + ", so it won't be tried again"
	case appupdate.NotWritable:
		return "Project B can't be replaced where it is: move it to Applications"
	case appupdate.Busy:
		return "another download is still stopping; check again in a moment"
	case appupdate.InstallFailed:
		return "couldn't put the new app in place (" + s.Error + ")"
	}
	return s.Error
}

// refused names a refusal: the signature's own answer, or the bundle not the version the
// manifest promised.
func refused(e string) string {
	switch {
	case strings.Contains(e, "expected key"):
		return "it isn't signed with Project B's key"
	case strings.Contains(e, "signature is broken"):
		return "its signature is broken"
	}
	return "it was refused (" + e + ")"
}
