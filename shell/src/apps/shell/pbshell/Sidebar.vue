<script setup>
import { paint } from '@/ui/hues.js';
import { computed } from 'vue';
import { openPin, send, state, tabLabel, unpin, workspaceOf } from '@/sdk/vue.js';
import { shown } from '@/keys.js';
import { entries, groupLabel } from './groups.js';
import { dragEnd, dragOver, dragStart, drop, dropClass } from './drag.js';
import { openMenu } from './menu.js';
import { toNewTab, view, waiting, workspaces } from './state.js';
import SidebarBookmarks from './SidebarBookmarks.vue';
import WorkspaceButton from './WorkspaceButton.vue';

// The list, flat: the pins and a rule under them, then a group's header and its tabs,
// which carry the group for their rule.
const key = (r) => (r.pinsEnd ? 'pins-end' : r.header ? 'g' + r.header.id : r.tab ? r.tab.id : 'p' + r.pin.id);
const rows = computed(() =>
  entries(state, view.workspace).flatMap((e) => (e.group ? [{ header: e.group }, ...e.tabs.map((tab) => ({ tab, group: e.group }))] : [e])),
);
</script>

<template>
  <nav v-shell class="sidebar" :class="{ closed: !view.sidebar }" aria-label="Tabs">
    <div class="inner">
      <button class="row new-tab" type="button" @click="toNewTab">
        <span class="icon">
          <svg width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M6.44.75v11.39M.75 6.19h11.39"/></svg>
        </span>
        <span class="title">New Tab</span>
        <span v-if="state.keys['tab.new']" class="hint" aria-hidden="true">{{ shown(state.keys['tab.new'], state.platform) }}</span>
      </button>
      <div class="tabs" role="tablist" aria-label="Open tabs">
        <template v-for="r in rows" :key="key(r)">
        <div v-if="r.pinsEnd" class="pins-end" role="separator"></div>
        <!-- A pin with no tab here: muted, as a tab not loaded; a press opens its tab, and its
             × unpins it. -->
        <div
          v-else-if="r.pin && !r.tab"
          class="row tab pin-closed"
          role="button"
          tabindex="0"
          :data-pin="r.pin.id"
          :title="r.pin.title || r.pin.url"
          @click="openPin(r.pin.id)"
          @keydown.enter="openPin(r.pin.id)"
        >
          <span class="icon">
            <img v-if="r.pin.favicon" :src="r.pin.favicon" alt="" width="16" height="16" />
          </span>
          <span class="title muted">{{ r.pin.title || r.pin.url }}</span>
          <button class="close" type="button" :aria-label="`Unpin ${r.pin.title || r.pin.url}`" title="Unpin" @click.stop="unpin(r.pin.id)">
            <svg width="9" height="9" viewBox="0 0 9 9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M7.43.9.75 7.58M.9.75l6.68 6.68"/></svg>
          </button>
        </div>
        <button
          v-else-if="r.header"
          type="button"
          class="row group"
          :data-group="r.header.id"
          :aria-expanded="!r.header.collapsed"
          :title="groupLabel(r.header)"
          @click="send({ t: 'editGroup', group: r.header.id, collapsed: !r.header.collapsed })"
          @contextmenu="openMenu('group', r.header.id, $event)"
          @keydown.shift.f10="openMenu('group', r.header.id, $event)"
        >
          <span class="icon"><span class="swatch" :style="{ background: paint(r.header.color) }"></span></span>
          <span class="title" :class="{ muted: !r.header.name }">{{ groupLabel(r.header) }}</span>
          <span class="chevron" :class="{ collapsed: r.header.collapsed }" aria-hidden="true"></span>
        </button>
        <div
          v-else
          class="row tab"
          :class="[{ active: r.tab.id === state.active, grouped: r.group, pinned: r.pin }, dropClass(r.tab.id)]"
          :style="r.group ? { '--group': paint(r.group.color) } : null"
          role="tab"
          :data-tab="r.tab.id"
          tabindex="0"
          :aria-selected="r.tab.id === state.active"
          @click="send({ t: 'activateTab', id: r.tab.id })"
          @keydown.enter="send({ t: 'activateTab', id: r.tab.id })"
          @keydown.shift.f10="openMenu('tab', r.tab.id, $event)"
          @auxclick.middle="send({ t: 'closeTab', id: r.tab.id })"
          @contextmenu="openMenu('tab', r.tab.id, $event)"
          draggable="true"
          @dragstart="dragStart($event, r.tab.id)"
          @dragover="dragOver($event, r.tab.id, 'y')"
          @drop="drop"
          @dragend="dragEnd"
        >
          <span class="icon">
            <img v-if="r.tab.favicon" :src="r.tab.favicon" alt="" width="16" height="16" />
          </span>
          <span class="title" :class="{ muted: !r.tab.title }">{{ tabLabel(r.tab) }}</span>
          <span v-if="waiting(r.tab)" class="waiting" role="img" aria-label="Waiting for an answer" title="Waiting for an answer"></span>
          <span v-else-if="workspaceOf(r.tab)" class="workspace" role="img" :aria-label="`In ${workspaceOf(r.tab).name}`" :title="workspaceOf(r.tab).name" :style="{ background: paint(workspaceOf(r.tab).color) }"></span>
          <button class="close" type="button" :aria-label="`Close ${tabLabel(r.tab)}`" @click.stop="send({ t: 'closeTab', id: r.tab.id })">
            <svg width="9" height="9" viewBox="0 0 9 9" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M7.43.9.75 7.58M.9.75l6.68 6.68"/></svg>
          </button>
        </div>
        </template>
      </div>
      <div class="rest"></div>
      <SidebarBookmarks v-if="state.prefs.bookmarksSidebar" />
      <WorkspaceButton v-if="workspaces.length" class="foot" />
    </div>
  </nav>
</template>

<style scoped>
/* The sidebar collapses to nothing; the page follows. Frame colour underneath, as for
   every tracked component, with its surface drawn on top. */
.sidebar {
  flex: none;
  width: 220px;
  overflow: hidden;
  background: var(--frame);
  transition: width var(--motion) var(--ease);
}

.sidebar.closed {
  width: 0;
}

.inner {
  display: flex;
  flex-direction: column;
  width: 220px;
  height: 100%;
  padding: 8px;
  box-sizing: border-box;
  background: var(--bar);
}

.tabs {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 4px;
  overflow-y: auto;
  scrollbar-width: none;
}

.rest {
  flex: 1;
}

.row {
  display: flex;
  align-items: center;
  flex: none;
  height: 32px;
  padding: 0 6px 0 10px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
  box-sizing: border-box;
}

.row:hover {
  background: var(--accent-tint);
}

.row:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

/* The selected tab as Settings marks its selected section: --accent-soft. */
.row.active {
  background: var(--accent-soft);
}

.new-tab {
  color: var(--icon);
}

.icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 16px;
  height: 16px;
}

.icon img {
  display: block;
  width: 16px;
  height: 16px;
}

.title {
  flex: 1;
  min-width: 0;
  margin-left: 10px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.pins-end {
  flex: none;
  height: 1px;
  margin: 2px 6px;
  background: var(--line);
}

.title.muted {
  color: var(--muted);
}

/* Where a dragged tab would land: a line in the accent on that side of the row. */
.tab.drop-before {
  box-shadow: 0 -2px 0 var(--accent);
}

.tab.drop-after {
  box-shadow: 0 2px 0 var(--accent);
}

/* A group's header: its colour, its name, and a chevron that folds it. */
.group .swatch {
  width: 10px;
  height: 10px;
  border-radius: 3px;
}

.chevron {
  flex: none;
  width: 6px;
  height: 6px;
  margin: 0 6px 2px;
  border-right: 1.5px solid var(--icon);
  border-bottom: 1.5px solid var(--icon);
  transform: rotate(45deg);
}

.chevron.collapsed {
  margin-bottom: 0;
  transform: rotate(-45deg);
}

/* A grouped tab sits in from the edge, on a rule in its group's colour. */
.tab.grouped {
  position: relative;
  margin-left: 12px;
}

.tab.grouped::before {
  content: '';
  position: absolute;
  left: -8px;
  top: -2px;
  bottom: -2px;
  width: 2px;
  border-radius: 1px;
  background: var(--group);
}

/* A page waiting on a dialog while another tab shows: an accent dot, ringed, in the
   workspace dot's place. */
.waiting {
  flex: none;
  width: 6px;
  height: 6px;
  margin: 0 6px;
  border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 0 2px var(--accent-soft);
}

/* The workspace a tab is in, as a dot in its colour. */
.workspace {
  flex: none;
  width: 6px;
  height: 6px;
  margin: 0 6px;
  border-radius: 50%;
}

.hint {
  flex: none;
  margin-right: 4px;
  color: var(--muted);
  font-size: 13px;
  opacity: 0.6;
}

.close {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 16px;
  height: 16px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--muted);
  cursor: pointer;
  visibility: hidden;
}

.row:hover .close,
.row.active .close,
.row:focus-within .close {
  visibility: visible;
}

.close:hover {
  background: var(--press);
  color: var(--text);
}

/* Touch: rows a finger hits, a close button as large on every row, as a finger has no
   hover to show it, and no shortcut hints, which a finger has no keys for
   (docs/large-screens.md). */
@media (pointer: coarse) and (not (any-pointer: fine)) {
  .row {
    height: 44px;
  }
  .close {
    width: 32px;
    height: 32px;
    visibility: visible;
  }
  .hint {
    display: none;
  }
}
</style>
