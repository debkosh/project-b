// pbblock: Project B's blocker, a plain C interface around Brave's adblock-rust
// (docs/blocking.md), from src/lib.rs. Go loads it and hands the engine the hot-path
// functions; nothing in the engine knows of filter lists or of adblock-rust.
//
// A blocker holds one engine per group of lists. Loading a group builds the new set aside
// and swaps it in at once, so a check under way finishes on the rules it began with, and
// the hot path never waits on a load.

#ifndef PROJECT_B_PBBLOCK_H_
#define PROJECT_B_PBBLOCK_H_

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct PBBlocker PBBlocker;

// What a request check answers.
enum {
  PBBLOCK_ALLOW = 0,
  PBBLOCK_BLOCK = 1,
  PBBLOCK_REDIRECT = 2,
};

// Loading, from Go, on any one thread at a time.

PBBlocker* pbblock_new(void);

// Builds one group's engine, in the library's own serialized form, from its lists' texts:
// lists and lens hold n pointers and lengths. 1 with *out and *out_len set, to be freed
// with pbblock_free_data; 0 for nothing.
// trusted holds n flags, 1 for a list whose rules may use the trusted-* scriptlets, or is
// null for none.
int pbblock_compile(const uint8_t* const* lists, const size_t* lens, const uint8_t* trusted, size_t n,
                    uint8_t** out, size_t* out_len);
void pbblock_free_data(uint8_t* data, size_t len);

// Loads a group's serialized engine under its name, in place of the one it had: 1 when it
// loaded, 0 when it isn't one this library reads (damaged, or another version's).
int pbblock_load(const PBBlocker* b, const char* group, const uint8_t* data, size_t len);
void pbblock_unload(const PBBlocker* b, const char* group);

// The scriptlets and stand-ins, as adblock-rust's resources JSON; groups loaded after it
// use them. 1 when it parsed.
int pbblock_resources(const PBBlocker* b, const uint8_t* json, size_t len);

// The hot path: any thread, never blocking; each reads one snapshot of the rules.

// A request: its address, the page's site and its kind in adblock's words. PBBLOCK_ALLOW,
// _BLOCK, or _REDIRECT with *redirect set (freed with pbblock_free); redirect may be null.
int pbblock_request(const PBBlocker* b, const char* url, const char* source,
                    const char* type, char** redirect);
// A frame's parts, as JSON: hide, procedural, exceptions, scripts, generichide.
char* pbblock_page(const PBBlocker* b, const char* url);
// The watcher's question, JSON {"classes": [...], "ids": [...]}; the answer a JSON array of
// selectors.
char* pbblock_hide(const PBBlocker* b, const char* page_url, const char* query);
void pbblock_free(char* s);

#ifdef __cplusplus
}
#endif

#endif  // PROJECT_B_PBBLOCK_H_
