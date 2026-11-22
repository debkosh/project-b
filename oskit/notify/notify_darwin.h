// The Mac's notifications, in notify_darwin.m. Any thread may call them: each hops to the main
// thread itself. A click reaches Go as oskitNotifyClicked, on the main thread.

#ifndef OSKIT_NOTIFY_NOTIFY_DARWIN_H_
#define OSKIT_NOTIFY_NOTIFY_DARWIN_H_

// Posts a note to the notification centre, replacing one with its id. actions are n_actions
// button ids and labels. macOS asks the person once whether the app may notify; until then,
// and if they say no, it shows nothing. A bare binary, as tests run, has no bundle and posts
// nothing.
void oskit_notify_show(const char* id,
                       const char* title,
                       const char* subtitle,
                       const char* body,
                       const char** action_ids,
                       const char** action_labels,
                       int n_actions);
void oskit_notify_remove(const char* id);
// Answers the question whether the app may notify yes, on the main thread, after the call that
// asked has returned, as oskitNotifyAnswered: a Mac's are the app's own to show.
void oskit_notify_answer(void);
// Becomes the notification centre's delegate, so clicks reach Go, notes posted by others in
// the app (as amarit's) too.
void oskit_notify_listen(void);

#endif  // OSKIT_NOTIFY_NOTIFY_DARWIN_H_
