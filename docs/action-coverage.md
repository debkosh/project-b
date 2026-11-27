# Action coverage

Every action a person can take, where the app offers it, and the tests that do it. Written by
`tools/action_coverage.py` from the code; `tools/test.sh mock` fails while an action has no
test and no reason in `tools/action-coverage-exempt.json` (roles/tester.md).

**235 actions and messages, 228 with a test, 7 exempt with a reason, 0 with none; 28 of Go's actions done by no test of the real app.**

## Go's actions: the menu bar, keys and the shells' own menus

| Action | Offered in | Real app | Tests |
|---|---|---|---|
| `address.focus` Open Location… | menu bar, keys (arshell, chrome, firefox, project-b), arshell, page:welcome | yes | `drive_first_app.py` |
| `app.about` About Project B | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `menu_test.go`, `barmenu_test.go`, `shellmenu_test.go`, `menu-bar.cjs` and 1 more |
| `app.checkUpdate` Check for Updates… | menu bar, keys (arshell, chrome, firefox, project-b) | **no** | `barmenu_test.go`, `imports.cjs` |
| `app.hide` Hide Project B | menu bar, keys (arshell, chrome, firefox, project-b) | **no** | `barmenu_test.go`, `commands_test.go` |
| `app.hideOthers` Hide Others | menu bar, keys (arshell, chrome, firefox, project-b) | **no** | exempt: macOS's own: AppKit hides the other apps |
| `app.menuBar` Open the Menu Bar |  | **no** | `commands_test.go`, `menu-bar.cjs` |
| `app.quit` Quit Project B | menu bar | **no** | `menu_test.go`, `barmenu_test.go`, `commands_test.go`, `shellmenu_test.go` and 2 more |
| `app.settings` Settings… | menu bar, arshell, fxshell, mnshell, mwshell, pbshell, tmshell, mkshell | yes | `app_test.go`, `menu_test.go`, `settings_prepare_test.go`, `welcome_test.go` and 18 more |
| `app.showAll` Show All | menu bar, keys (arshell, chrome, firefox, project-b) | **no** | exempt: macOS's own: AppKit shows the hidden apps |
| `arshell.commandBar` Open Command Bar | menu bar, keys (arshell), arshell | yes | `app_test.go`, `settings.cjs`, `arshell.cjs`, `drive_first_app.py` |
| `arshell.copyAddress` Copy Address | menu bar, keys (arshell, chrome, firefox, project-b), arshell | yes | `arshell.cjs`, `check_clipboard.py`, `drive_first_app.py` |
| `arshell.favorite` Add to Favorites | menu bar, arshell | yes | `arshell.cjs`, `drive_first_app.py` |
| `arshell.newSpace` New Space… | menu bar, arshell | yes | `drive_first_app.py` |
| `arshell.sidebar` Show or Hide Sidebar | menu bar, keys (arshell, chrome, firefox, project-b), arshell | **no** | `commands_test.go`, `arshell.cjs` |
| `arshell.space1` Space 1 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `app_test.go`, `arshell.cjs`, `drive_first_app.py` |
| `arshell.space2` Space 2 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `arshell.space3` Space 3 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `arshell.space4` Space 4 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `arshell.space5` Space 5 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `arshell.space6` Space 6 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `arshell.space7` Space 7 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `arshell.space8` Space 8 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `arshell.space9` Space 9 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `arshell.spaceNext` Next Space | menu bar, keys (arshell), arshell | **no** | `app_test.go`, `arshell.cjs` |
| `arshell.spacePrevious` Previous Space | menu bar, keys (arshell), arshell | **no** | `arshell.cjs` |
| `arshell.splitAdd` Add to Split View | menu bar, keys (arshell, chrome, firefox, project-b), arshell | **no** | `settings.cjs`, `arshell.cjs` |
| `arshell.splitRemove` Separate Split View | menu bar, keys (arshell, chrome, firefox, project-b), arshell | **no** | `arshell.cjs` |
| `bookmarks.show` Bookmark Manager | menu bar, keys (arshell, chrome, firefox, project-b), mwshell, pbshell, page:welcome | yes | `pages_test.go`, `font-scale.cjs`, `pages.cjs`, `mwshell.cjs` and 1 more |
| `bookmarks.toggle` Bookmark This Page… | menu bar, keys (arshell, chrome, firefox, project-b) | **no** | `pages_test.go`, `pages.cjs` |
| `dev.viewSource` View Source | menu bar, keys (arshell, chrome, firefox, project-b), page:welcome | yes | `app_test.go`, `drive_first_app.py` |
| `downloads.show` Downloads | menu bar, keys (arshell, chrome, firefox, project-b), mwshell, page:welcome, mkshell | yes | `download_folder_settings_test.go`, `menu_test.go`, `pages_test.go`, `welcome_test.go` and 10 more |
| `edit.copy` Copy | menu bar, keys (arshell, chrome, firefox, project-b), mnshell, ui | yes | `contextmenu_test.go`, `menu_test.go`, `barmenu_test.go`, `shellmenu_test.go` and 3 more |
| `edit.cut` Cut | menu bar, keys (arshell, chrome, firefox, project-b) | **no** | `contextmenu_test.go`, `pbshell.cjs` |
| `edit.delete` Delete | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `download-details.cjs`, `pages.cjs`, `passkey-settings.cjs`, `password-settings.cjs` and 4 more |
| `edit.paste` Paste | menu bar, keys (arshell, chrome, firefox, project-b) | **no** | `contextmenu_test.go`, `menu_test.go`, `pbshell.cjs` |
| `edit.pasteAndMatchStyle` Paste and Match Style | menu bar, keys (arshell, chrome, firefox, project-b) | **no** | exempt: macOS's own: AppKit's editing command, sent to the focused field |
| `edit.redo` Redo | menu bar, keys (arshell, chrome, firefox, project-b), arshell, mnshell, ui | yes | `app_test.go`, `contextmenu_test.go`, `drive_first_app.py` |
| `edit.selectAll` Select All | menu bar, keys (arshell, chrome, firefox, project-b) | **no** | `contextmenu_test.go`, `barmenu_test.go` |
| `edit.undo` Undo | menu bar, keys (arshell, chrome, firefox, project-b), arshell, mnshell, ui | **no** | `contextmenu_test.go` |
| `find.next` Find Next | menu bar, keys (arshell, chrome, firefox, project-b), arshell, mnshell, ui | yes | `find_test.go`, `shellmenu_test.go`, `drive_first_app.py` |
| `find.open` Find… | menu bar, keys (arshell, chrome, firefox, project-b), mwshell, mkshell | yes | `find_test.go`, `shellmenu_test.go`, `shell-contract.cjs`, `pbshell.cjs` and 1 more |
| `find.previous` Find Previous | menu bar, keys (arshell, chrome, firefox, project-b), arshell, mnshell, ui | yes | `drive_first_app.py` |
| `focus.nextPane` Focus Next Pane | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `focus.previousPane` Focus Previous Pane | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `history.show` Show All History | menu bar, keys (arshell, chrome, firefox, project-b), mwshell, page:welcome | yes | `pages_test.go`, `font-scale.cjs`, `pages.cjs`, `drive_first_app.py` |
| `nav.back` Back | menu bar, keys (arshell, chrome, firefox, project-b), arshell, fxshell, mnshell, ui | yes | `download-details.cjs`, `first-app.cjs`, `imports.cjs`, `shell-contract.cjs` and 6 more |
| `nav.forward` Forward | menu bar, keys (arshell, chrome, firefox, project-b), arshell, fxshell, mnshell, ui | yes | `first-app.cjs`, `shell-contract.cjs`, `mwshell.cjs`, `pbshell.cjs` and 2 more |
| `nav.reload` Reload This Page | menu bar, keys (arshell, chrome, firefox, project-b), arshell, fxshell, mnshell, ui | yes | `menu_test.go`, `commands_test.go`, `settings.cjs`, `mwshell.cjs` and 1 more |
| `nav.reloadHard` Force Reload This Page | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `nav.stop` Stop | menu bar, keys (arshell, chrome, firefox, project-b), arshell, fxshell, mnshell, mwshell, mkshell | yes | `app_test.go`, `drive_first_app.py` |
| `page.print` Print… | menu bar, keys (arshell, chrome, firefox, project-b), mwshell | **no** | `contextmenu_test.go`, `print_test.go`, `shellmenu_test.go`, `mwshell.cjs` |
| `profile.picker` Profiles… | menu bar | yes | `barmenu_test.go`, `drive_first_app.py`, `check.py` |
| `tab.close` Close Tab | menu bar, keys (arshell, chrome, firefox, project-b), arshell, pbshell | yes | `commands_test.go`, `shellmenu_test.go`, `settings.cjs`, `shell-contract.cjs` and 3 more |
| `tab.duplicate` Duplicate Tab | menu bar, keys (arshell, chrome, firefox, project-b), pbshell | yes | `app_test.go`, `settings_messages_test.go`, `suggest_test.go`, `pbshell-groups.cjs` and 1 more |
| `tab.moveToNewWindow` Move Tab to New Window | menu bar, keys (arshell, chrome, firefox, project-b), pbshell | yes | `app_test.go`, `drive_first_app.py` |
| `tab.new` New Tab | menu bar, keys (arshell, chrome, firefox, project-b), arshell, fxshell, mnshell, pbshell, tmshell, page:welcome | yes | `app_test.go`, `menu_test.go`, `barmenu_test.go`, `commands_test.go` and 16 more |
| `tab.next` Select Next Tab | menu bar, keys (arshell, chrome, firefox, project-b), page:welcome | yes | `menu_test.go`, `drive_first_app.py` |
| `tab.pin` Pin Tab | menu bar, keys (arshell, chrome, firefox, project-b), arshell, pbshell | yes | `pins_test.go`, `arshell.cjs`, `pbshell-pins.cjs`, `drive_first_app.py` |
| `tab.previous` Select Previous Tab | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `tab.recentNext` Select Recent Tab | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `tab.recentPrevious` Select Recent Tab Backward | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `tab.reopenClosed` Reopen Closed Tab | menu bar, keys (arshell, chrome, firefox, project-b), page:welcome | yes | `session_test.go`, `drive_first_app.py` |
| `tab.select1` Tab 1 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `commands_test.go`, `shell-contract.cjs`, `fxshell.cjs`, `mnshell.cjs` and 2 more |
| `tab.select2` Tab 2 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `mwshell.cjs`, `drive_first_app.py` |
| `tab.select3` Tab 3 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `tab.select4` Tab 4 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `tab.select5` Tab 5 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `tab.select6` Tab 6 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `tab.select7` Tab 7 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `tab.select8` Tab 8 | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `drive_first_app.py` |
| `tab.selectLast` Last Tab | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `mnshell.cjs`, `drive_first_app.py` |
| `tmshell.focusDown` Focus pane below | menu bar, keys (arshell, chrome, firefox, project-b), tmshell | **no** | `tmshell.cjs` |
| `tmshell.focusLeft` Focus pane left | menu bar, keys (arshell, chrome, firefox, project-b), tmshell | **no** | `tmshell.cjs` |
| `tmshell.focusRight` Focus pane right | menu bar, keys (arshell, chrome, firefox, project-b), tmshell | **no** | `tmshell.cjs` |
| `tmshell.focusUp` Focus pane above | menu bar, keys (arshell, chrome, firefox, project-b), tmshell | **no** | `tmshell.cjs` |
| `tmshell.resizeDown` Move divider down | menu bar, keys (arshell, chrome, firefox, project-b), tmshell | **no** | `tmshell.cjs` |
| `tmshell.resizeLeft` Move divider left | menu bar, keys (arshell, chrome, firefox, project-b), tmshell | **no** | `tmshell.cjs` |
| `tmshell.resizeRight` Move divider right | menu bar, keys (arshell, chrome, firefox, project-b), tmshell | **no** | `tmshell.cjs` |
| `tmshell.resizeUp` Move divider up | menu bar, keys (arshell, chrome, firefox, project-b), tmshell | **no** | `tmshell.cjs` |
| `tmshell.splitDown` Split down | menu bar, keys (arshell, chrome, firefox, project-b), tmshell | **no** | `app_test.go`, `tmshell.cjs` |
| `tmshell.splitRight` Split right | menu bar, keys (arshell, chrome, firefox, project-b), tmshell | **no** | `app_test.go`, `settings.cjs`, `tmshell.cjs` |
| `tmshell.zoom` Zoom pane | menu bar, keys (arshell, chrome, firefox, project-b), tmshell | **no** | `tmshell.cjs` |
| `view.fullScreen` Full Screen | menu bar, keys (arshell, chrome, firefox, project-b), fxshell | yes | `app_test.go`, `commands_test.go`, `fxshell.cjs`, `mwshell.cjs` and 1 more |
| `view.tabsSide` Tabs on Side | menu bar, keys (arshell, chrome, firefox, project-b) | **no** | `shellmenu_test.go`, `mnshell.cjs`, `pbshell-bookmarks.cjs`, `pbshell-groups.cjs` and 2 more |
| `view.tabsTop` Tabs on Top | menu bar, keys (arshell, chrome, firefox, project-b) | yes | `suggest_test.go`, `mnshell.cjs`, `pbshell-bookmarks.cjs`, `pbshell-drag.cjs` and 4 more |
| `view.zoomIn` Zoom In | menu bar, keys (arshell, chrome, firefox, project-b), fxshell | yes | `fxshell.cjs`, `drive_first_app.py` |
| `view.zoomOut` Zoom Out | menu bar, keys (arshell, chrome, firefox, project-b), fxshell | yes | `fxshell.cjs`, `drive_first_app.py` |
| `view.zoomReset` Actual Size | menu bar, keys (arshell, chrome, firefox, project-b), fxshell | yes | `fxshell.cjs`, `drive_first_app.py` |
| `window.bringAllToFront` Bring All to Front | menu bar, keys (arshell, chrome, firefox, project-b) | **no** | exempt: macOS's own: AppKit orders the app's windows |
| `window.close` Close Window | menu bar, keys (arshell, chrome, firefox, project-b), ui | yes | `drive_first_app.py` |
| `window.minimize` Minimize | menu bar, keys (arshell, chrome, firefox, project-b), ui | **no** | `menu_test.go`, `barmenu_test.go` |
| `window.new` New Window | menu bar, keys (arshell, chrome, firefox, project-b), fxshell | yes | `app_test.go`, `shellmenu_test.go`, `fxshell.cjs`, `drive_first_app.py` |
| `window.zoom` Zoom | menu bar, keys (arshell, chrome, firefox, project-b), ui | **no** | `tmshell.cjs` |

## What the shells send Go when a person does something

| Action | Offered in | Real app | Tests |
|---|---|---|---|
| `activateTab` | arshell, fxshell, mkshell, mnshell, mwshell, pbshell, tmshell | yes | `thumbs_test.go`, `settings.cjs`, `shell-contract.cjs`, `mwshell.cjs` and 2 more |
| `addKey` | page:settings | **no** | `app_test.go`, `settings_messages_test.go` |
| `addToGroup` | arshell, mkshell, mwshell, pbshell | yes | `drive_first_app.py` |
| `allowNotifications` | mkshell, mwshell | **no** | `download_offer_test.go` |
| `answerDialog` | mkshell | **no** | `dialogs_test.go`, `passkeys_own_test.go`, `passkeys_test.go` |
| `answerPasskey` | mkshell | **no** | `passkeys_test.go`, `passkey-settings.cjs` |
| `askNotifications` | page:welcome | yes | `drive_android.py` |
| `back` | arshell, mkshell | **no** | `fullscreen_test.go` |
| `backCloses` | mkshell, mwshell | **no** | exempt: not a person's action: the Mobile shell tells the engine whether the back key closes something of its own |
| `bookmark` | mkshell | **no** | `pages_test.go`, `mwshell.cjs`, `pbshell.cjs` |
| `cancelImport` | ui | **no** | `imports_extensions_test.go` |
| `cancelUpdate` | fxshell, page:settings | **no** | `settings_messages_test.go` |
| `captureTab` | mwshell | **no** | `thumbs_test.go` |
| `checkUpdate` | page:settings | **no** | `settings_messages_test.go` |
| `chooseDownloadFolder` | page:settings | **no** | `download_folder_settings_test.go` |
| `chooseFolder` | ui | **no** | `imports_test.go` |
| `clear` | page:history | **no** | `pages_test.go` |
| `clearAllSiteData` | page:settings | **no** | `site_data_settings_test.go` |
| `clearData` | page:settings | **no** | `clear_data_settings_test.go` |
| `clearDataCounts` | page:settings | **no** | `clear_data_settings_test.go` |
| `clearDownloads` | page:downloads | **no** | `downloads_test.go`, `pages_test.go` |
| `clearSiteData` | page:settings | **no** | `site_data_settings_test.go` |
| `close` | page:downloads | **no** | `pages_test.go` |
| `closeGroup` | arshell, mkshell, mwshell | **no** | `app_test.go` |
| `closeTab` | arshell, fxshell, mkshell, mnshell, mwshell, pbshell, tmshell | **no** | `app_test.go`, `fullscreen_test.go`, `pins_test.go`, `session_test.go` and 6 more |
| `contextMenuChoose` | mkshell | **no** | `contextmenu_test.go` |
| `contextMenuClose` | mkshell | **no** | `contextmenu_test.go` |
| `copyText` | arshell, mwshell | **no** | `suggest_test.go`, `arshell.cjs` |
| `create` | page:picker | **no** | `picker_test.go` |
| `createGroup` | arshell, pbshell | yes | `shell-contract.cjs`, `pbshell-drag.cjs`, `tmshell.cjs`, `drive_first_app.py` |
| `createWorkspace` | arshell, page:settings | yes | `app_test.go`, `lifecycle_test.go`, `pins_test.go`, `drive_first_app.py` |
| `delete` | page:history | **no** | `pages_test.go` |
| `deleteDownload` | page:downloads | **no** | `pages_test.go` |
| `deleteWorkspace` | page:settings | **no** | `app_test.go`, `pins_test.go`, `pbshell-workspaces.cjs` |
| `dismissCrash` | arshell, fxshell, mkshell, mnshell, mwshell, pbshell | **no** | `crash-report.cjs` |
| `dismissMissing` | page:settings | **no** | `settings_messages_test.go` |
| `done` | page:welcome | **no** | `welcome_test.go` |
| `downloadAction` | mkshell, page:downloads | **no** | `downloads_test.go`, `pages_test.go` |
| `editBookmark` | page:bookmarks | **no** | `pages_test.go` |
| `editGroup` | arshell, mkshell, mwshell, pbshell | **no** | `app_test.go`, `tmshell.cjs` |
| `editWorkspace` | arshell, page:settings | **no** | `app_test.go` |
| `findBrowsers` | page:settings, page:welcome, ui | **no** | `imports_extensions_test.go`, `imports_test.go` |
| `findClose` | mkshell | **no** | `find_test.go` |
| `findNext` | mkshell | **no** | `find_test.go` |
| `findText` | mkshell | **no** | `find_test.go` |
| `focusShell` | arshell, fxshell, mkshell, mnshell, mwshell, pbshell, tmshell, ui | **no** | exempt: not a person's action: a shell asks for the keyboard back when its bar or panel opens |
| `focusTab` | mkshell | **no** | `app_test.go`, `first-app.cjs`, `shell-contract.cjs`, `fxshell.cjs` and 2 more |
| `forgetSite` | mwshell | **no** | `suggest_test.go` |
| `forgetSiteSetting` | page:settings | **no** | `settings_messages_test.go` |
| `forward` | arshell, mkshell | yes | `drive_first_app.py` |
| `import` | ui | **no** | `imports_extensions_test.go`, `imports_test.go` |
| `importFile` | ui | **no** | `imports_test.go` |
| `keepCookies` | page:settings | **no** | `quit_erasure_test.go` |
| `layout` | mkshell, page:picker | **no** | `app_test.go`, `first-app.cjs` |
| `makeDefault` | page:settings, page:welcome | yes | `default_browser_settings_test.go`, `drive_android.py` |
| `menuOpened` | mkshell, mwshell | **no** | `settings_prepare_test.go` |
| `more` | page:history | **no** | exempt: not a person's action: the History page asks for its next rows as it scrolls |
| `moveBookmark` | page:bookmarks | **no** | `pages_test.go` |
| `moveTab` | pbshell, tmshell | **no** | `app_test.go` |
| `moveWorkspace` | page:settings | **no** | `app_test.go` |
| `navigate` | arshell, fxshell, mkshell, mnshell, mwshell, pbshell, tmshell | yes | `app_test.go`, `blocking_test.go`, `contextmenu_test.go`, `dialogs_test.go` and 19 more |
| `newFolder` | page:bookmarks | **no** | `pages_test.go`, `mwshell.cjs` |
| `newTab` | arshell, fxshell, mkshell, mnshell, mwshell, pbshell, tmshell | yes | `app_test.go`, `fullscreen_test.go`, `pins_test.go`, `session_test.go` and 21 more |
| `notificationsOffered` | mkshell, mwshell | **no** | `download_offer_test.go` |
| `open` | page:bookmarks, page:history, page:picker | **no** | `events_test.go`, `pages_test.go`, `picker_test.go` |
| `openAccess` | ui | **no** | `imports_test.go` |
| `openHelp` | page:settings | **no** | `help_settings_test.go`, `password-settings.cjs` |
| `openInWorkspace` | page:settings | **no** | `app_test.go` |
| `passkeyDelete` | page:settings | **no** | `passkey-settings.cjs` |
| `passkeysList` | page:settings | **no** | `passkey-settings.cjs` |
| `passwordAt` | pbshell | **no** | `password_fill_test.go`, `password-fill.cjs` |
| `passwordCopy` | page:settings | yes | `password_settings_test.go`, `password-settings.cjs`, `drive_android.py` |
| `passwordDelete` | page:settings | **no** | `password_settings_test.go`, `password-settings.cjs` |
| `passwordDismiss` | mwshell, pbshell | **no** | `password_fill_test.go`, `password-fill.cjs` |
| `passwordNeverRemove` | page:settings | **no** | `password-save.cjs` |
| `passwordPick` | mwshell, pbshell | **no** | `password_fill_test.go`, `password-fill-mobile.cjs`, `password-fill.cjs` |
| `passwordSaveAnswer` | mwshell, ui | yes | `password_fill_test.go`, `password-save-mobile.cjs`, `password-save.cjs`, `drive_android.py` |
| `passwordSaveShow` | mwshell, ui | **no** | `password_fill_test.go`, `password-save-mobile.cjs`, `password-save.cjs` |
| `passwordShow` | page:settings | yes | `password_settings_test.go`, `password-settings.cjs`, `drive_android.py` |
| `passwordsClosed` | page:settings | **no** | `password_settings_test.go`, `password-settings.cjs` |
| `passwordsList` | page:settings | **no** | `password_settings_test.go`, `password-settings.cjs` |
| `passwordsManage` | mwshell, pbshell | **no** | `password_fill_test.go`, `password-fill-mobile.cjs`, `password-fill.cjs`, `password-save.cjs` |
| `ready` | mkshell, page:bookmarks, page:downloads, page:history, page:picker, page:settings, page:welcome | **no** | `events_test.go`, `app_test.go`, `default_browser_settings_test.go`, `extensions_test.go` and 8 more |
| `recording` | page:settings | **no** | `app_test.go` |
| `relaunch` | arshell, fxshell, mkshell, mnshell, mwshell, page:settings, pbshell, tmshell | **no** | `app_test.go` |
| `reload` | arshell, mkshell | yes | `drive_first_app.py` |
| `removeBookmark` | mkshell, page:bookmarks | **no** | `pages_test.go` |
| `removeFromGroup` | arshell, mkshell, mwshell, pbshell | yes | `drive_first_app.py` |
| `removeKey` | page:settings | **no** | `settings_messages_test.go` |
| `resetAction` | page:settings | **no** | `settings_messages_test.go` |
| `resetProfile` | page:settings | **no** | `profile_test.go` |
| `retryUpdate` | fxshell, page:settings | **no** | `settings_messages_test.go` |
| `run` | arshell, fxshell, mkshell, mnshell, mwshell, pbshell, tmshell, ui | yes | `app_test.go`, `settings_prepare_test.go`, `check_clipboard.py`, `drive_first_app.py` |
| `saveBookmark` | mkshell | **no** | `pages_test.go`, `mwshell.cjs` |
| `saveReport` | mwshell, pbshell | **no** | `downloads_test.go` |
| `search` | page:history | **no** | `pages_test.go` |
| `setAddressBarPosition` | page:settings | **no** | `settings_messages_test.go` |
| `setAutoUpdate` | page:settings, page:welcome | **no** | `app_test.go` |
| `setBlockGroup` | page:settings | **no** | `blocking_test.go`, `settings.cjs` |
| `setBlockList` | page:settings | **no** | `blocking_test.go`, `settings.cjs` |
| `setChannel` | page:settings | **no** | `app_test.go` |
| `setFlag` | page:settings | **no** | `settings_messages_test.go` |
| `setFrameColor` | page:settings | **no** | `app_test.go` |
| `setHidden` | page:settings | **no** | `settings_messages_test.go` |
| `setOneShell` | page:settings | **no** | `app_test.go` |
| `setPhoneShell` | page:settings | **no** | `app_test.go` |
| `setPref` | page:settings, page:welcome | yes | `app_test.go`, `blocking_test.go`, `bookmark_settings_test.go`, `download_folder_settings_test.go` and 9 more |
| `setProfileName` | page:settings | **no** | `profile_test.go` |
| `setReserve` | page:settings | **no** | `settings_messages_test.go` |
| `setSearchEngine` | page:settings, page:welcome | **no** | `settings_messages_test.go` |
| `setShell` | page:settings | **no** | `settings.cjs` |
| `setSiteSetting` | page:settings | yes | `settings_messages_test.go`, `sitefeatures_test.go`, `check_basics.py`, `drive_first_app.py` |
| `setSparkOn` | page:welcome | **no** | `welcome_test.go` |
| `setStart` | page:settings | **no** | `settings_messages_test.go` |
| `setTabLayout` | page:settings, pbshell | **no** | `app_test.go`, `large-screens.cjs` |
| `setTheme` | page:settings | **no** | `app_test.go` |
| `setWorkspaces` | page:welcome | **no** | `welcome_test.go` |
| `shareReport` | mkshell, mwshell, pbshell | **no** | `crash-report.cjs` |
| `shareText` | mwshell | **no** | `suggest_test.go` |
| `shown` | mkshell, page:picker | **no** | `app_test.go`, `picker_test.go` |
| `sparkAdd` | page:settings | **no** | `sparks_messages_test.go` |
| `sparkExport` | page:settings | **no** | `sparks_import_test.go` |
| `sparkImport` | page:settings | **no** | `sparks_import_test.go`, `sparks_messages_test.go` |
| `sparkImportCancel` | page:settings | **no** | `sparks_messages_test.go` |
| `sparkImportKeep` | page:settings | **no** | `sparks_import_test.go` |
| `sparkRemove` | page:settings | **no** | `sparks_messages_test.go` |
| `sparkSetOn` | page:settings | **no** | `sparks_messages_test.go` |
| `sparkSource` | page:settings | **no** | `sparks_messages_test.go` |
| `sparkUpdate` | page:settings | **no** | `sparks_messages_test.go` |
| `suggest` | fxshell, mkshell, mnshell, mwshell, tmshell | **no** | `app_test.go`, `suggest_test.go` |
| `syncBackupDrop` | page:settings | **no** | `sync.cjs` |
| `syncCancel` | page:settings | **no** | `sync.cjs` |
| `syncDetails` | page:settings | **no** | `sync_settings_test.go`, `sync.cjs` |
| `syncDisconnect` | page:settings | **no** | `sync.cjs` |
| `syncGetBucket` | page:settings | **no** | `sync_settings_test.go`, `sync.cjs` |
| `syncNow` | page:settings | yes | `sync.cjs`, `drive_android.py` |
| `syncStartFresh` | page:settings | **no** | `sync.cjs` |
| `timeline` | mkshell | **no** | `timeline_test.go` |
| `ungroup` | arshell, mkshell, mwshell | **no** | `tmshell.cjs` |
| `useKeymap` | page:welcome | **no** | `welcome_test.go` |
| `useTemplate` | page:settings | **no** | `settings_messages_test.go` |
| `welcomeAgain` | page:settings | **no** | `welcome_test.go` |
