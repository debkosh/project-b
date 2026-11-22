// The Mac's notifications (notify_darwin.h), through UNUserNotificationCenter.

#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>

#include "notify_darwin.h"

#include "_cgo_export.h"

static void OnMain(dispatch_block_t block) {
  if (NSThread.isMainThread) {
    block();
  } else {
    dispatch_async(dispatch_get_main_queue(), block);
  }
}

// macOS's notification centre needs an app bundle: a bare binary posts nothing.
static BOOL HasBundle(void) {
  return NSBundle.mainBundle.bundleIdentifier != nil;
}

static NSString* String(const char* s) {
  return [NSString stringWithUTF8String:s] ?: @"";
}

// The category of a note's buttons, by their ids, so notes with the same buttons share one.
static NSString* const kCategoryPrefix = @"oskit.";

// Shows notes while the app is in front too, and takes their clicks.
@interface OskitNotifyDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation OskitNotifyDelegate

- (void)userNotificationCenter:(UNUserNotificationCenter*)center
       willPresentNotification:(UNNotification*)notification
         withCompletionHandler:
             (void (^)(UNNotificationPresentationOptions))completionHandler {
  completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList |
                    UNNotificationPresentationOptionSound);
}

- (void)userNotificationCenter:(UNUserNotificationCenter*)center
    didReceiveNotificationResponse:(UNNotificationResponse*)response
             withCompletionHandler:(void (^)(void))completionHandler {
  NSString* identifier = response.notification.request.identifier;
  NSString* action = response.actionIdentifier;
  if ([action isEqualToString:UNNotificationDismissActionIdentifier]) {
    completionHandler();
    return;
  }
  if ([action isEqualToString:UNNotificationDefaultActionIdentifier]) {
    action = @"";
  }
  dispatch_async(dispatch_get_main_queue(), ^{
    oskitNotifyClicked((char*)identifier.UTF8String, (char*)action.UTF8String);
  });
  completionHandler();
}

@end

static UNUserNotificationCenter* Center(void) {
  static OskitNotifyDelegate* delegate;
  UNUserNotificationCenter* center = UNUserNotificationCenter.currentNotificationCenter;
  if (!delegate) {
    delegate = [[OskitNotifyDelegate alloc] init];
    center.delegate = delegate;
  }
  return center;
}

void oskit_notify_listen(void) {
  if (!HasBundle()) {
    return;
  }
  OnMain(^{
    Center();
  });
}

// Adds category to the centre's own, keeping every other, then runs then.
static void AddCategory(UNUserNotificationCenter* center,
                        UNNotificationCategory* category,
                        dispatch_block_t then) {
  [center getNotificationCategoriesWithCompletionHandler:^(NSSet<UNNotificationCategory*>* existing) {
    for (UNNotificationCategory* c in existing) {
      if ([c.identifier isEqualToString:category.identifier]) {
        then();
        return;
      }
    }
    NSMutableSet* all = [existing mutableCopy];
    [all addObject:category];
    [center setNotificationCategories:all];
    then();
  }];
}

void oskit_notify_show(const char* id,
                       const char* title,
                       const char* subtitle,
                       const char* body,
                       const char** action_ids,
                       const char** action_labels,
                       int n_actions) {
  if (!HasBundle()) {
    return;
  }
  UNMutableNotificationContent* content = [[UNMutableNotificationContent alloc] init];
  content.title = String(title);
  content.subtitle = String(subtitle);
  content.body = String(body);
  content.threadIdentifier = String(subtitle);
  UNNotificationCategory* category = nil;
  if (n_actions > 0) {
    NSMutableArray<UNNotificationAction*>* actions = [NSMutableArray array];
    NSMutableArray<NSString*>* ids = [NSMutableArray array];
    for (int i = 0; i < n_actions; i++) {
      [actions addObject:[UNNotificationAction actionWithIdentifier:String(action_ids[i])
                                                              title:String(action_labels[i])
                                                            options:UNNotificationActionOptionForeground]];
      [ids addObject:String(action_ids[i])];
    }
    NSString* name = [kCategoryPrefix stringByAppendingString:[ids componentsJoinedByString:@"\n"]];
    category = [UNNotificationCategory categoryWithIdentifier:name
                                                      actions:actions
                                            intentIdentifiers:@[]
                                                      options:0];
    content.categoryIdentifier = name;
  }
  UNNotificationRequest* request = [UNNotificationRequest requestWithIdentifier:String(id)
                                                                        content:content
                                                                        trigger:nil];
  OnMain(^{
    UNUserNotificationCenter* center = Center();
    [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                          completionHandler:^(BOOL granted, NSError* error) {
                            if (!granted) {
                              return;
                            }
                            dispatch_block_t add = ^{
                              [center addNotificationRequest:request withCompletionHandler:nil];
                            };
                            if (category) {
                              AddCategory(center, category, add);
                            } else {
                              add();
                            }
                          }];
  });
}

void oskit_notify_remove(const char* id) {
  if (!HasBundle()) {
    return;
  }
  NSArray* ids = @[ String(id) ];
  OnMain(^{
    [Center() removeDeliveredNotificationsWithIdentifiers:ids];
    [Center() removePendingNotificationRequestsWithIdentifiers:ids];
  });
}

void oskit_notify_answer(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    oskitNotifyAnswered(1);
  });
}
