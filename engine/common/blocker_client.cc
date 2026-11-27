#include "project_b/engine/common/blocker_client.h"

#include <tuple>
#include <utility>

#include "base/containers/flat_set.h"
#include "base/containers/lru_cache.h"
#include "base/functional/bind.h"
#include "base/no_destructor.h"
#include "base/synchronization/lock.h"
#include "content/public/renderer/render_thread.h"
#include "mojo/public/cpp/bindings/shared_remote.h"
#include "project_b/engine/mojom/project_b.mojom.h"
#include "services/network/public/cpp/resource_request.h"
#include "services/network/public/mojom/fetch_api.mojom-shared.h"
#include "url/gurl.h"
#include "url/origin.h"

namespace project_b {
namespace blocker_client {

namespace {

// How many answers a renderer keeps: the same tracker on every frame, a page polling the
// same address, ask once.
constexpr size_t kRemembered = 512;

using Key = std::tuple<std::string, std::string, std::string>;

Key KeyOf(const Question& question) {
  return Key(question.url, question.source, question.type);
}

struct State {
  base::Lock lock;
  bool on = false;
  std::string watcher;
  // The sites of the pages where the blocker is off.
  base::flat_set<net::SchemefulSite> off;
  base::LRUCache<Key, int32_t> answers{kRemembered};
  // One more each time the answers are forgotten, so an answer to a question asked before
  // isn't kept after.
  uint64_t generation = 0;
  // Bound on the IO thread, so a question never waits on the main thread, which may be
  // busy running the page; usable from any thread.
  mojo::SharedRemote<mojom::Blocker> remote;
};

State& Get() {
  static base::NoDestructor<State> state;
  return *state;
}

// Lock held.
void Forget(State& state) {
  state.answers.Clear();
  ++state.generation;
}

// The site of the page a request is for: its top frame's, as cookies see it, or else the
// site that asked for it.
std::optional<net::SchemefulSite> PageSite(const network::ResourceRequest& request) {
  if (!request.site_for_cookies.IsNull()) {
    return request.site_for_cookies.site();
  }
  if (request.request_initiator) {
    return net::SchemefulSite(*request.request_initiator);
  }
  return std::nullopt;
}

bool Asked(const GURL& url) {
  return url.SchemeIsHTTPOrHTTPS() || url.SchemeIsWSOrWSS();
}

// A request's kind in adblock's words, from what it is for. A request with no
// destination is a script's fetch or XMLHttpRequest, or, kept alive past its page, a
// beacon or a link's ping.
const char* TypeOf(const network::ResourceRequest& request) {
  if (request.url.SchemeIsWSOrWSS()) {
    return "websocket";
  }
  using Destination = network::mojom::RequestDestination;
  switch (request.destination) {
    case Destination::kScript:
    case Destination::kWorker:
    case Destination::kSharedWorker:
    case Destination::kServiceWorker:
    case Destination::kAudioWorklet:
    case Destination::kPaintWorklet:
      return "script";
    case Destination::kImage:
      return "image";
    case Destination::kStyle:
      return "stylesheet";
    case Destination::kEmpty:
      return request.keepalive ? "ping" : "xhr";
    case Destination::kFrame:
    case Destination::kIframe:
    case Destination::kFencedframe:
      return "sub_frame";
    case Destination::kFont:
      return "font";
    case Destination::kAudio:
    case Destination::kVideo:
    case Destination::kTrack:
      return "media";
    case Destination::kReport:
      return "ping";
    default:
      return "other";
  }
}

}  // namespace

void Start() {
  mojo::SharedRemote<mojom::Blocker> remote;
  content::RenderThread::Get()->BindHostReceiver(
      remote.BindNewPipeAndPassReceiver(content::RenderThread::Get()->GetIOTaskRunner()));
  State& state = Get();
  base::AutoLock hold(state.lock);
  state.remote = std::move(remote);
}

void Set(bool on, std::string watcher) {
  State& state = Get();
  base::AutoLock hold(state.lock);
  state.on = on;
  state.watcher = on ? std::move(watcher) : std::string();
  Forget(state);
}

void Changed() {
  State& state = Get();
  base::AutoLock hold(state.lock);
  Forget(state);
}

void SetOff(const std::vector<std::string>& origins) {
  base::flat_set<net::SchemefulSite> sites;
  for (const std::string& origin : origins) {
    sites.insert(net::SchemefulSite(GURL(origin)));
  }
  State& state = Get();
  base::AutoLock hold(state.lock);
  state.off = std::move(sites);
}

std::string Watcher() {
  State& state = Get();
  base::AutoLock hold(state.lock);
  return state.watcher;
}

std::optional<Question> QuestionFor(const network::ResourceRequest& request) {
  if (!Asked(request.url)) {
    return std::nullopt;
  }
  // Our own pages, and a page that isn't on the web, ask nothing.
  const std::optional<net::SchemefulSite> page = PageSite(request);
  if (!page) {
    return std::nullopt;
  }
  const GURL source = page->GetURL();
  if (!source.SchemeIsHTTPOrHTTPS()) {
    return std::nullopt;
  }
  return Question{request.url.spec(), source.spec(), TypeOf(request), *page};
}

std::optional<Question> RedirectedTo(const Question& first, const GURL& url) {
  if (!Asked(url)) {
    return std::nullopt;
  }
  Question question = first;
  question.url = url.spec();
  return question;
}

bool Asks(const Question& question) {
  State& state = Get();
  base::AutoLock hold(state.lock);
  return state.on && state.remote && !state.off.contains(question.page);
}

std::optional<int32_t> Remembered(const Question& question) {
  State& state = Get();
  base::AutoLock hold(state.lock);
  auto it = state.answers.Get(KeyOf(question));
  if (it == state.answers.end()) {
    return std::nullopt;
  }
  return it->second;
}

void Ask(const Question& question, base::OnceCallback<void(int32_t)> answer) {
  mojo::SharedRemote<mojom::Blocker> remote;
  uint64_t generation;
  {
    State& state = Get();
    base::AutoLock hold(state.lock);
    remote = state.remote;
    generation = state.generation;
  }
  if (!remote) {
    std::move(answer).Run(mojom::kBlockerAllow);
    return;
  }
  remote->Check(question.url, question.source, question.type,
                base::BindOnce(
                    [](Key key, uint64_t generation,
                       base::OnceCallback<void(int32_t)> answer, int32_t verdict) {
                      {
                        State& state = Get();
                        base::AutoLock hold(state.lock);
                        // A redirect is asked each time, so the browser has its
                        // stand-in at hand when the request reaches it.
                        if (state.generation == generation &&
                            verdict != mojom::kBlockerRedirect) {
                          state.answers.Put(std::move(key), verdict);
                        }
                      }
                      std::move(answer).Run(verdict);
                    },
                    KeyOf(question), generation, std::move(answer)));
}

void Hide(const std::string& page_url,
          const std::string& query,
          base::OnceCallback<void(const std::string&)> answer) {
  mojo::SharedRemote<mojom::Blocker> remote;
  {
    State& state = Get();
    base::AutoLock hold(state.lock);
    remote = state.remote;
  }
  if (!remote) {
    return;
  }
  remote->Hide(page_url, query, std::move(answer));
}

}  // namespace blocker_client
}  // namespace project_b
