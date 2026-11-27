// A renderer's side of the blocker (docs/blocking.md): what the browser told it, whether
// there is a blocker, its watcher's script and the sites where it is off; the last answers
// it had; and its pipe to the browser, where the blocker is asked. A page's requests are
// asked from whatever thread makes them, a frame's, a worker's or a synchronous load's, so
// everything here may be used on any thread.

#ifndef PROJECT_B_ENGINE_COMMON_BLOCKER_CLIENT_H_
#define PROJECT_B_ENGINE_COMMON_BLOCKER_CLIENT_H_

#include <cstdint>
#include <optional>
#include <string>
#include <vector>

#include "base/functional/callback.h"
#include "net/base/schemeful_site.h"

class GURL;

namespace network {
struct ResourceRequest;
}

namespace project_b {
namespace blocker_client {

// Opens the pipe to the browser's blocker. On the main thread, as the renderer starts.
void Start();

// What the browser says (project_b.mojom.Renderer). Each of the first two forgets the
// answers kept, as the rules they came from may have gone.
void Set(bool on, std::string watcher);
void Changed();
void SetOff(const std::vector<std::string>& origins);

// The watcher's script while there is a blocker, else "".
std::string Watcher();

// A request's question: its address, its page's site, as an address, and its kind in
// adblock's words, with the page's site itself, which says whether the blocker is off.
struct Question {
  std::string url;
  std::string source;
  std::string type;
  net::SchemefulSite page;
};

// The question of a request as it starts: none when it isn't asked at all, as for a
// request that isn't http(s) or ws(s), one of a page that isn't, or of no page.
std::optional<Question> QuestionFor(const network::ResourceRequest& request);
// The same request's question once it is redirected to url: none when it isn't asked.
std::optional<Question> RedirectedTo(const Question& first, const GURL& url);
// Whether it is asked now: there is a blocker, and it isn't off for the page.
bool Asks(const Question& question);

// The answer kept for the same question, if any: an allow or a block, never a redirect.
std::optional<int32_t> Remembered(const Question& question);
// Asks the browser, keeps the answer, and calls back with it on the calling sequence.
void Ask(const Question& question, base::OnceCallback<void(int32_t)> answer);

// The watcher's question for the page at page_url, and its answer, a JSON array of
// selectors, on the calling sequence.
void Hide(const std::string& page_url,
          const std::string& query,
          base::OnceCallback<void(const std::string&)> answer);

}  // namespace blocker_client
}  // namespace project_b

#endif  // PROJECT_B_ENGINE_COMMON_BLOCKER_CLIENT_H_
