// The Mac's opening of files and links, in open_darwin.m. Any thread may call them: each hops
// to the main thread itself. An answer reaches Go as oskitOpenDefaultBrowser and links as
// oskitOpenURLs, on the main thread, after the call that asked has returned.

#ifndef OSKIT_OPEN_OPEN_DARWIN_H_
#define OSKIT_OPEN_OPEN_DARWIN_H_

void oskit_open_file(const char* path);
void oskit_open_reveal(const char* path);
int oskit_open_is_default_browser(void);
void oskit_open_ask_default_browser(void);
// Takes the links and files macOS opens with the app, from now on, as NSApp's delegate.
void oskit_open_receive_links(void);

#endif  // OSKIT_OPEN_OPEN_DARWIN_H_
