#ifndef PROJECT_B_ENGINE_BROWSER_NOTIFICATIONS_H_
#define PROJECT_B_ENGINE_BROWSER_NOTIFICATIONS_H_

#include <cstdint>
#include <map>
#include <string>

#include "base/memory/raw_ptr.h"
#include "content/public/browser/platform_notification_service.h"
#include "url/gurl.h"

namespace content {
class BrowserContext;
}

namespace project_b {

// The notifications a profile's pages show (docs/daily-driver.md): content's side of them.
// Each one goes to Go first, which says whether it shows and puts it on the system itself
// (core/system); a click on it there reaches Go, which fires the page's click event through
// Click. One the page closes, Go hears of, to take it off the system.
class Notifications : public content::PlatformNotificationService {
 public:
  explicit Notifications(content::BrowserContext* context);
  Notifications(const Notifications&) = delete;
  Notifications& operator=(const Notifications&) = delete;
  ~Notifications() override;

  // A click on a notification a page showed, by its id, in whichever profile showed it:
  // fires the page's click event. One clicked, closed or gone is ignored.
  static void Click(const std::string& id);

  void DisplayNotification(
      const std::string& notification_id,
      const GURL& origin,
      const GURL& document_url,
      const blink::PlatformNotificationData& notification_data,
      const blink::NotificationResources& notification_resources) override;
  void DisplayPersistentNotification(
      const std::string& notification_id,
      const GURL& service_worker_origin,
      const GURL& origin,
      const blink::PlatformNotificationData& notification_data,
      const blink::NotificationResources& notification_resources) override;
  void CloseNotification(const std::string& notification_id) override;
  void ClosePersistentNotification(const std::string& notification_id) override;
  void GetDisplayedNotifications(
      DisplayedNotificationsCallback callback) override;
  void GetDisplayedNotificationsForOrigin(
      const GURL& origin,
      DisplayedNotificationsCallback callback) override;
  void ScheduleTrigger(base::Time timestamp) override;
  base::Time ReadNextTriggerTimestamp() override;
  int64_t ReadNextPersistentNotificationId() override;
  void RecordNotificationUkmEvent(
      const content::NotificationDatabaseData& data) override;

 private:
  // One shown: its origin, and whether a service worker shows it.
  struct Shown {
    GURL origin;
    bool persistent;
  };

  // Asks Go, which shows it on the system, then fires the page's show event.
  void Show(const std::string& id,
            const GURL& origin,
            const GURL& page,
            const blink::PlatformNotificationData& data,
            bool persistent);

  raw_ptr<content::BrowserContext> context_;
  std::map<std::string, Shown> shown_;
  int64_t next_persistent_id_;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_NOTIFICATIONS_H_
