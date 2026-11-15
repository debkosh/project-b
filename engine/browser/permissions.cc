#include "project_b/engine/browser/permissions.h"

#include <map>
#include <optional>
#include <string>
#include <utility>

#include "base/compiler_specific.h"
#include "base/functional/bind.h"
#include "base/functional/callback.h"
#include "base/no_destructor.h"
#include "base/task/sequenced_task_runner.h"
#include "base/time/time.h"
#include "base/strings/string_util.h"
#include "content/public/browser/browser_context.h"
#include "content/public/browser/media_capture_devices.h"
#include "content/public/browser/media_stream_request.h"
#include "content/public/browser/permission_controller.h"
#include "content/public/browser/permission_descriptor_util.h"
#include "content/public/browser/permission_request_description.h"
#include "content/public/browser/permission_result.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/render_process_host.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"
#include "third_party/blink/public/common/permissions/permission_utils.h"
#include "third_party/blink/public/mojom/mediastream/media_stream.mojom.h"
#include "url/gurl.h"
#include "url/origin.h"

namespace project_b {

namespace {

using Callback =
    base::OnceCallback<void(const std::vector<content::PermissionResult>&)>;

// A request waiting for Go: its callback and how many kinds it asked for.
struct Pending {
  Callback callback;
  size_t count;
};

std::map<uint32_t, Pending>& PendingRequests() {
  static base::NoDestructor<std::map<uint32_t, Pending>> pending;
  return *pending;
}

uint32_t g_last_request = 0;

// A kind as Go names it, as the Permissions API does: content's types stay here.
std::string KindOf(blink::PermissionType type) {
  using T = blink::PermissionType;
  switch (type) {
    case T::VIDEO_CAPTURE: return "camera";
    case T::AUDIO_CAPTURE: return "microphone";
    case T::NOTIFICATIONS: return "notifications";
    case T::CLIPBOARD_SANITIZED_WRITE: return "clipboard-write";
    case T::CLIPBOARD_READ_WRITE: return "clipboard-read";
    case T::GEOLOCATION: return "geolocation";
    case T::GEOLOCATION_APPROXIMATE: return "geolocation-approximate";
    case T::MIDI: return "midi";
    case T::MIDI_SYSEX: return "midi-sysex";
    case T::PERSISTENT_STORAGE: return "persistent-storage";
    case T::BACKGROUND_SYNC: return "background-sync";
    case T::PERIODIC_BACKGROUND_SYNC: return "periodic-background-sync";
    case T::BACKGROUND_FETCH: return "background-fetch";
    case T::SENSORS: return "sensors";
    case T::PAYMENT_HANDLER: return "payment-handler";
    case T::IDLE_DETECTION: return "idle-detection";
    case T::WAKE_LOCK_SCREEN: return "screen-wake-lock";
    case T::WAKE_LOCK_SYSTEM: return "system-wake-lock";
    case T::NFC: return "nfc";
    case T::VR: return "xr-spatial-tracking";
    case T::AR: return "xr-ar";
    case T::HAND_TRACKING: return "hand-tracking";
    case T::STORAGE_ACCESS_GRANT: return "storage-access";
    case T::TOP_LEVEL_STORAGE_ACCESS: return "top-level-storage-access";
    case T::CAMERA_PAN_TILT_ZOOM: return "camera-pan-tilt-zoom";
    case T::WINDOW_MANAGEMENT: return "window-management";
    case T::LOCAL_FONTS: return "local-fonts";
    case T::DISPLAY_CAPTURE: return "display-capture";
    case T::CAPTURED_SURFACE_CONTROL: return "captured-surface-control";
    case T::SMART_CARD: return "smart-card";
    case T::WEB_PRINTING: return "web-printing";
    case T::SPEAKER_SELECTION: return "speaker-selection";
    case T::KEYBOARD_LOCK: return "keyboard-lock";
    case T::POINTER_LOCK: return "pointer-lock";
    case T::AUTOMATIC_FULLSCREEN: return "automatic-fullscreen";
    case T::WEB_APP_INSTALLATION: return "web-app-installation";
    case T::LOCAL_NETWORK_ACCESS: return "local-network-access";
    case T::LOCAL_NETWORK: return "local-network";
    case T::LOOPBACK_NETWORK: return "loopback-network";
    case T::PROTECTED_MEDIA_IDENTIFIER: return "protected-media-identifier";
    default: return "unknown";
  }
}

std::string KindOf(const blink::mojom::PermissionDescriptorPtr& descriptor) {
  return KindOf(blink::PermissionDescriptorToPermissionType(descriptor));
}

int64_t TabOf(content::RenderFrameHost* frame) {
  content::WebContents* contents =
      frame ? content::WebContents::FromRenderFrameHost(frame) : nullptr;
  return contents ? hooks::TabOf(contents) : 0;
}

// Go's status as content's.
blink::mojom::PermissionStatus FromGo(int64_t status) {
  switch (status) {
    case PB_PERMISSION_GRANTED:
      return blink::mojom::PermissionStatus::GRANTED;
    case PB_PERMISSION_ASK:
      return blink::mojom::PermissionStatus::ASK;
    default:
      return blink::mojom::PermissionStatus::DENIED;
  }
}

// The device of devices with one of ids, or the first when none is named or found.
std::optional<blink::MediaStreamDevice> Pick(
    const blink::MediaStreamDevices& devices,
    const std::vector<std::string>& ids) {
  for (const auto& id : ids) {
    for (const auto& d : devices) {
      if (d.id == id) {
        return d;
      }
    }
  }
  if (devices.empty()) {
    return std::nullopt;
  }
  return devices.front();
}

// How long a first camera or microphone request waits for content's list of devices: the
// first read starts the capture service, which lists them a moment later.
constexpr int kDeviceTries = 30;
constexpr base::TimeDelta kDeviceWait = base::Milliseconds(100);

// Gives the page the devices it was allowed, once content lists them. The capture service
// starts only when a page first asks, so the list may still be empty then; it waits up to
// three seconds for it before saying there is no such device.
void GiveDevices(content::MediaStreamRequest request,
                 bool audio,
                 bool video,
                 content::MediaResponseCallback callback,
                 int tries) {
  using blink::mojom::MediaStreamRequestResult;
  auto* capture = content::MediaCaptureDevices::GetInstance();
  blink::mojom::StreamDevicesSet set;
  set.stream_devices.emplace_back(blink::mojom::StreamDevices::New());
  blink::mojom::StreamDevices& devices = *set.stream_devices[0];
  if (audio) {
    devices.audio_device = Pick(capture->GetAudioCaptureDevices(),
                                request.requested_audio_device_ids);
  }
  if (video) {
    devices.video_device = Pick(capture->GetVideoCaptureDevices(),
                                request.requested_video_device_ids);
  }
  if ((audio && !devices.audio_device) || (video && !devices.video_device)) {
    if (tries < kDeviceTries) {
      base::SequencedTaskRunner::GetCurrentDefault()->PostDelayedTask(
          FROM_HERE,
          base::BindOnce(&GiveDevices, std::move(request), audio, video,
                         std::move(callback), tries + 1),
          kDeviceWait);
      return;
    }
    std::move(callback).Run(blink::mojom::StreamDevicesSet(),
                            MediaStreamRequestResult::NO_HARDWARE, nullptr);
    return;
  }
  std::move(callback).Run(set, MediaStreamRequestResult::OK, nullptr);
}

}  // namespace

Permissions::Permissions() = default;

Permissions::~Permissions() = default;

// static
void Permissions::Answer(uint32_t request, const int32_t* statuses, size_t count) {
  auto it = PendingRequests().find(request);
  if (it == PendingRequests().end()) {
    return;
  }
  Pending pending = std::move(it->second);
  PendingRequests().erase(it);
  std::vector<content::PermissionResult> results;
  for (size_t i = 0; i < pending.count; i++) {
    results.emplace_back(
        FromGo(i < count ? UNSAFE_BUFFERS(statuses[i]) : PB_PERMISSION_DENIED));
  }
  std::move(pending.callback).Run(results);
}

blink::mojom::PermissionStatus Permissions::StatusOf(
    const blink::mojom::PermissionDescriptorPtr& descriptor,
    int64_t tab,
    const url::Origin& origin) {
  return StatusOf(KindOf(descriptor), tab, origin);
}

// static
blink::mojom::PermissionStatus Permissions::StatusOf(const std::string& kind,
                                                     int64_t tab,
                                                     const url::Origin& origin) {
  hooks::Args args;
  args.Int(tab);
  args.Str(origin.Serialize());
  args.Str(kind);
  PBValue result;
  if (!hooks::Ask(PB_EVENT_PERMISSION_STATUS, args, &result)) {
    return blink::mojom::PermissionStatus::DENIED;
  }
  return FromGo(result.i);
}

void Permissions::RequestPermissionsFromCurrentDocument(
    content::RenderFrameHost* render_frame_host,
    const content::PermissionRequestDescription& request_description,
    Callback callback) {
  url::Origin origin =
      request_description.requesting_origin.is_valid()
          ? url::Origin::Create(request_description.requesting_origin)
          : render_frame_host->GetLastCommittedOrigin();
  std::string kinds;
  for (const auto& permission : request_description.permissions) {
    if (!kinds.empty()) {
      kinds += ",";
    }
    kinds += KindOf(permission);
  }
  uint32_t id = ++g_last_request;
  PendingRequests()[id] = {std::move(callback),
                           request_description.permissions.size()};
  hooks::Args args;
  args.Int(id);
  args.Int(TabOf(render_frame_host));
  args.Str(origin.Serialize());
  args.Str(kinds);
  args.Bool(request_description.user_gesture);
  // Go answers with Answer, perhaps from inside this call.
  hooks::Fire(PB_EVENT_PERMISSION_REQUEST, args);
}

blink::mojom::PermissionStatus Permissions::GetPermissionStatus(
    const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
    const GURL& requesting_origin,
    const GURL& embedding_origin) {
  return StatusOf(permission_descriptor, 0, url::Origin::Create(requesting_origin));
}

content::PermissionResult Permissions::GetPermissionResultForOriginWithoutContext(
    const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
    const url::Origin& requesting_origin,
    const url::Origin& embedding_origin) {
  return content::PermissionResult(
      StatusOf(permission_descriptor, 0, requesting_origin));
}

content::PermissionResult Permissions::GetPermissionResultForCurrentDocument(
    const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
    content::RenderFrameHost* render_frame_host,
    bool should_include_device_status) {
  return content::PermissionResult(
      StatusOf(permission_descriptor, TabOf(render_frame_host),
               render_frame_host->GetLastCommittedOrigin()));
}

content::PermissionResult Permissions::GetPermissionResultForWorker(
    const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
    content::RenderProcessHost* render_process_host,
    const GURL& worker_origin) {
  return content::PermissionResult(
      StatusOf(permission_descriptor, 0, url::Origin::Create(worker_origin)));
}

content::PermissionResult Permissions::GetPermissionResultForEmbeddedRequester(
    const blink::mojom::PermissionDescriptorPtr& permission_descriptor,
    content::RenderFrameHost* render_frame_host,
    const url::Origin& overridden_origin) {
  return content::PermissionResult(StatusOf(
      permission_descriptor, TabOf(render_frame_host), overridden_origin));
}

void Permissions::ResetPermission(blink::PermissionType permission,
                                  const GURL& requesting_origin,
                                  const GURL& embedding_origin) {}

// static
bool Permissions::CheckMediaAccess(content::RenderFrameHost* frame,
                                   const url::Origin& origin,
                                   blink::mojom::MediaStreamType type) {
  std::string kind;
  switch (type) {
    case blink::mojom::MediaStreamType::DEVICE_AUDIO_CAPTURE:
      kind = "microphone";
      break;
    case blink::mojom::MediaStreamType::DEVICE_VIDEO_CAPTURE:
      kind = "camera";
      break;
    default:
      return false;
  }
  return StatusOf(kind, TabOf(frame), origin) ==
         blink::mojom::PermissionStatus::GRANTED;
}

// static
void Permissions::RequestMediaAccess(content::WebContents* contents,
                                     const content::MediaStreamRequest& request,
                                     content::MediaResponseCallback callback) {
  using blink::mojom::MediaStreamRequestResult;
  using blink::mojom::MediaStreamType;
  bool audio = request.audio_type == MediaStreamType::DEVICE_AUDIO_CAPTURE;
  bool video = request.video_type == MediaStreamType::DEVICE_VIDEO_CAPTURE;
  // Only a camera and a microphone, for now: no screen or tab capture.
  bool other = (!audio && request.audio_type != MediaStreamType::NO_SERVICE) ||
               (!video && request.video_type != MediaStreamType::NO_SERVICE);
  content::RenderFrameHost* frame = content::RenderFrameHost::FromID(
      request.render_process_id, request.render_frame_id);
  if (other || (!audio && !video) || !frame) {
    std::move(callback).Run(blink::mojom::StreamDevicesSet(),
                            MediaStreamRequestResult::NOT_SUPPORTED, nullptr);
    return;
  }
  std::vector<blink::PermissionType> types;
  if (audio) {
    types.push_back(blink::PermissionType::AUDIO_CAPTURE);
  }
  if (video) {
    types.push_back(blink::PermissionType::VIDEO_CAPTURE);
  }
  content::PermissionRequestDescription description(
      content::PermissionDescriptorUtil::
          CreatePermissionDescriptorForPermissionTypes(types),
      request.user_gesture);
  contents->GetBrowserContext()
      ->GetPermissionController()
      ->RequestPermissionsFromCurrentDocument(
          frame, std::move(description),
          base::BindOnce(
              [](content::MediaStreamRequest request, bool audio, bool video,
                 content::MediaResponseCallback callback,
                 const std::vector<content::PermissionResult>& results) {
                size_t i = 0;
                bool audio_ok =
                    audio && results.size() > i &&
                    results[i++].status == blink::mojom::PermissionStatus::GRANTED;
                bool video_ok =
                    video && results.size() > i &&
                    results[i].status == blink::mojom::PermissionStatus::GRANTED;
                if ((audio && !audio_ok) || (video && !video_ok)) {
                  std::move(callback).Run(
                      blink::mojom::StreamDevicesSet(),
                      MediaStreamRequestResult::PERMISSION_DENIED, nullptr);
                  return;
                }
                GiveDevices(request, audio, video, std::move(callback), 0);
              },
              request, audio, video, std::move(callback)));
}

}  // namespace project_b
