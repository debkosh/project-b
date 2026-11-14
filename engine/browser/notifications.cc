#include "project_b/engine/browser/notifications.h"

#include <set>
#include <utility>

#include "base/functional/callback_helpers.h"
#include "base/strings/utf_string_conversions.h"
#include "base/time/time.h"
#include "content/public/browser/notification_event_dispatcher.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/hooks.h"
#include "third_party/blink/public/common/notifications/platform_notification_data.h"
#include "url/origin.h"

namespace project_b {

// Content keeps a service worker's notifications in its database by these ids, across
// runs: starting from the time keeps a new run's clear of an earlier one's.
Notifications::Notifications(content::BrowserContext* context)
    : context_(context),
      next_persistent_id_(base::Time::Now().InMillisecondsSinceUnixEpoch()) {}

Notifications::~Notifications() = default;

void Notifications::Show(const std::string& id,
                         const GURL& origin,
                         const GURL& page,
                         const blink::PlatformNotificationData& data,
                         bool persistent) {
  std::string serialized = url::Origin::Create(origin).Serialize();
  std::string title = base::UTF16ToUTF8(data.title);
  std::string body = base::UTF16ToUTF8(data.body);
  hooks::Args args;
  args.Str(id);
  args.Str(serialized);
  args.Str(page.is_valid() ? page.spec() : std::string());
  args.Str(title);
  args.Str(body);
  PBValue show;
  if (!hooks::Ask(PB_EVENT_NOTIFICATION_SHOW, args, &show) || !show.i) {
    return;
  }
  shown_[id] = {origin, persistent};
  std::string host = url::Origin::Create(origin).host();
  PostSystemNotification(this, id, title, host, body);
  if (!persistent) {
    content::NotificationEventDispatcher::GetInstance()
        ->DispatchNonPersistentShowEvent(id);
  }
}

void Notifications::Clicked(const std::string& id) {
  auto it = shown_.find(id);
  if (it == shown_.end()) {
    return;
  }
  Shown shown = it->second;
  shown_.erase(it);
  auto* dispatcher = content::NotificationEventDispatcher::GetInstance();
  if (shown.persistent) {
    dispatcher->DispatchNotificationClickEvent(context_, id, shown.origin,
                                               std::nullopt, std::nullopt,
                                               base::DoNothing());
  } else {
    dispatcher->DispatchNonPersistentClickEvent(id, base::DoNothing());
  }
  hooks::Args args;
  args.Str(id);
  hooks::Fire(PB_EVENT_NOTIFICATION_CLICKED, args);
}

void Notifications::DisplayNotification(
    const std::string& notification_id,
    const GURL& origin,
    const GURL& document_url,
    const blink::PlatformNotificationData& notification_data,
    const blink::NotificationResources& notification_resources) {
  Show(notification_id, origin, document_url, notification_data, false);
}

void Notifications::DisplayPersistentNotification(
    const std::string& notification_id,
    const GURL& service_worker_origin,
    const GURL& origin,
    const blink::PlatformNotificationData& notification_data,
    const blink::NotificationResources& notification_resources) {
  Show(notification_id, origin, GURL(), notification_data, true);
}

void Notifications::CloseNotification(const std::string& notification_id) {
  if (shown_.erase(notification_id)) {
    RemoveSystemNotification(notification_id);
  }
}

void Notifications::ClosePersistentNotification(
    const std::string& notification_id) {
  CloseNotification(notification_id);
}

void Notifications::GetDisplayedNotifications(
    DisplayedNotificationsCallback callback) {
  std::set<std::string> ids;
  for (const auto& [id, shown] : shown_) {
    ids.insert(id);
  }
  std::move(callback).Run(std::move(ids), false);
}

void Notifications::GetDisplayedNotificationsForOrigin(
    const GURL& origin,
    DisplayedNotificationsCallback callback) {
  std::set<std::string> ids;
  for (const auto& [id, shown] : shown_) {
    if (url::IsSameOriginWith(shown.origin, origin)) {
      ids.insert(id);
    }
  }
  std::move(callback).Run(std::move(ids), false);
}

void Notifications::ScheduleTrigger(base::Time timestamp) {}

base::Time Notifications::ReadNextTriggerTimestamp() {
  return base::Time::Max();
}

int64_t Notifications::ReadNextPersistentNotificationId() {
  return next_persistent_id_++;
}

void Notifications::RecordNotificationUkmEvent(
    const content::NotificationDatabaseData& data) {}

}  // namespace project_b
