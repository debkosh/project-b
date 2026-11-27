# What waits after a start

A start does only what the first window needs. Background jobs wait for one timer, then
all of them start. Sudipta, 2026-11-24: "we should not do stuff right at the start....
since that could lead to late startup or slow startups... we should wait n seconds before
running housekeeping services", and "lets keep that simple... one timer after which
everything starts which is kind og background job".

## The rule

- One wait, `later.Wait`, ten seconds, on every platform.
- It counts from the end of `app.Start`, once the first window has been asked for, not from
  process start, so a slow profile open doesn't use it up.
- A background job is anything the app does for itself, with no one waiting on it: checking
  for updates, sync's runs. Each goes through `later.Run`.
- Nothing ends the wait early. Something the person does is not a background job and runs
  at once, as before: Check now asks the updater directly, and turning sync on or using a
  saved password or passkey opens the device key when it needs it.

## `core/later`

```go
// Wait is how long after a start the background jobs wait.
var Wait = 10 * time.Second

// Begin starts the clock; app.Start calls it as its last step. post runs a function
// on the main thread.
func Begin(post func(func()))

// Run runs f on the main thread once the wait is over, or soon if it already is.
func Run(f func())

// Stop drops what is still waiting; App.Stop calls it.
func Stop()
```

- **Thread.** `f` always runs on the main thread, through `post`. A job that works
  elsewhere passes a function that only hands over: sync's is `m.Wanted`, which sends
  its own goroutine the load. Nothing slow runs on the main thread.
- **After the wait.** `Run` posts `f` straight away. It never calls `f` inside `Run`, so
  `f` runs at the same point whether it came early or late.
- **Before `Begin`.** `Run` queues `f`, and it runs once the wait is over.
- **A quit within the wait.** `Stop` drops what is still waiting, so it never runs. A job
  asked for after it waits for a `Begin`, which an app that is quitting never calls.
- **Which starts.** Every `app.Start` calls `Begin`: a normal start, one after an update,
  one after a crash. The profile picker copy and a copy that hands over and quits never
  call `app.Start`, so they never call `Begin` or `Run`, and wait for nothing.
- **More than one app in a process.** The tests start many apps in one process, so
  `Begin` starts over: it drops anything still waiting and starts a new clock. So
  `app.Start` asks for its jobs after its own `Begin`, at its end, where they can't be
  taken for the jobs of the app before.
- **Tests.** Tests set `later.Wait = 0`, so a mockengine run waits for nothing.
- `later` has no knowledge of any feature. A job tells it nothing apart from the function it
  passes.

## What goes through it

| Job | Today, at start | Through `later.Run` |
|---|---|---|
| Updates | `a.Update.Start()` at the end of `app.Start`: amarit's first round has no wait (`FirstCheck` is zero), so it fetches the manifest, or `linux.json` and the portal, at once | `later.Run(a.Update.Start)`. On Linux, `flatpak.New` (the portal's UpdateMonitor) moves from `update.New` into `Start` with it |
| Sync | `sync.New` loaded at once, on sync's goroutine | `later.Run(a.Sync.Wanted)`: at a start sync only reads whether it is on, and its first load, which asks the OS for the device key, comes with `Wanted` |

amarit doesn't change: its `FirstCheck` stays zero, and project-b chooses when to call
`Start`.

## What stays at start

These are needed for the first window, or for a start to go right:

- `Update.Settle`, the rollback after a release fails to start three times. `Healthy` comes
  once a window is up.
- Crash: renaming the last `crash.log`.
- The store, prefs, session and timeline writers.
- The one-time `history.jsonl` migration, the downloads list and the shortcuts.
- The ad blocker and Safe Browsing: the groups' compiled engines, loaded from disk before
  the first window's page (`a.Blocking.Load`), the built-in hosts standing in for any group not
  compiled yet. Nothing is downloaded at start: the lists are fetched through
  `later.Run(a.Blocking.Start)`, then every hour ([blocking.md](blocking.md#fetching-them)).
- The built-in sparks, on a profile's first start.
- Extensions, when their flag is on: Chromium loads them as the profile opens.
- What the engine is told about quit erasure.
- On Linux, the dark-mode watch.
- The socket that hands links to the copy already running.

The engine's own code starts no background work at a start. A new job that the first
window doesn't need goes through `later.Run`.
