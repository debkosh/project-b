// The shell's own menu (docs/menus-shortcuts.md, "The shell's menu"): the menu bar's tree
// for this window and its profile, which Go sends as state.menu when the shell asks, as
// the menu bar reads it when it opens. Each item is {title, action, enabled, checked},
// {separator: true} or {title, items}; the shell draws it and never adds its own.
import { send } from './channel.js';

// requestMenu asks Go for the menu as it is now, as the shell opens it.
export function requestMenu() {
  send({ t: 'menu' });
}

// requestBarMenu asks Go for the whole menu bar, as a shell that draws it off the Mac does
// (ui/MenuBar.vue): Go answers barMenu, every menu with its submenus, and the Mac's app menu's
// items where these desktops keep them.
export function requestBarMenu() {
  send({ t: 'menu', bar: true });
}

// runMenuItem runs an item: an action, or a list's item, as the menu bar does.
export function runMenuItem(item) {
  send({ t: 'run', text: item.action });
}
