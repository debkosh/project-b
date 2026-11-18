<script setup>
import { computed } from 'vue';
import { state } from '@/sdk/vue.js';
import { bookmarksUnder, inFolder, topFolder } from '@/ui/bookmarks.js';
import { host, openTab, sidebarTree, tabFor } from './bookmarks.js';
import SiteMark from './SiteMark.vue';

// The bookmarks under the tabs, while bookmarksSidebar is on (docs/bookmark-folders.md,
// "Where bookmarks show"): the top folder as a tree, each folder folded until it is pressed.
// A bookmark shows its tab when one has it open, marked with a dot, else opens in a new tab.

// The tree, flat, as the rows draw it: each entry with how deep it is.
const rows = computed(() => {
  const out = [];
  const walk = (parent, depth) => {
    for (const e of inFolder(state.bookmarks, parent)) {
      out.push({ e, depth });
      if (e.kind === 'folder' && sidebarTree.folders.has(e.id)) walk(e.id, depth + 1);
    }
  };
  walk('', 0);
  return out;
});

function fold(id) {
  if (sidebarTree.folders.has(id)) sidebarTree.folders.delete(id);
  else sidebarTree.folders.add(id);
}
</script>

<template>
  <section v-if="state.bookmarks.length" class="bookmarks" aria-label="Bookmarks">
    <button type="button" class="head" :aria-expanded="sidebarTree.open" @click="sidebarTree.open = !sidebarTree.open">
      <span class="chevron" :class="{ collapsed: !sidebarTree.open }" aria-hidden="true"></span>
      <span>{{ topFolder }}</span>
    </button>
    <div v-if="sidebarTree.open" class="tree" role="tree" aria-label="Bookmarks">
      <template v-for="r in rows" :key="r.e.id">
        <button
          v-if="r.e.kind === 'folder'"
          type="button"
          class="row folder"
          role="treeitem"
          :aria-expanded="sidebarTree.folders.has(r.e.id)"
          :style="{ paddingLeft: 4 + r.depth * 16 + 'px' }"
          :title="r.e.title"
          @click="fold(r.e.id)"
        >
          <span class="chevron" :class="{ collapsed: !sidebarTree.folders.has(r.e.id) }" aria-hidden="true"></span>
          <svg class="icon" width="16" height="16" viewBox="0 0 16 16" aria-hidden="true"><path d="M2.5 4.5a1 1 0 0 1 1-1h3l1.5 1.5h4.5a1 1 0 0 1 1 1v5.5a1 1 0 0 1-1 1h-9a1 1 0 0 1-1-1z" /></svg>
          <span class="title">{{ r.e.title }}</span>
          <span class="count">{{ bookmarksUnder(state.bookmarks, r.e.id) }}</span>
        </button>
        <button
          v-else
          type="button"
          class="row mark"
          role="treeitem"
          :class="{ active: tabFor(r.e.url) && tabFor(r.e.url).id === state.active }"
          :style="{ paddingLeft: 26 + r.depth * 16 + 'px' }"
          :title="`${r.e.title || host(r.e.url)}\n${r.e.url}`"
          @click="openTab(r.e.url)"
        >
          <SiteMark :url="r.e.url" />
          <span class="title">{{ r.e.title || host(r.e.url) }}</span>
          <span v-if="tabFor(r.e.url)" class="open" role="img" aria-label="Open in a tab"></span>
        </button>
      </template>
    </div>
  </section>
</template>

<style scoped>
/* Under the tabs, parted from them by a hairline; it scrolls on its own, and takes half the
   sidebar at most, so the tabs keep room. */
.bookmarks {
  display: flex;
  flex-direction: column;
  flex: 0 1 auto;
  min-height: 0;
  max-height: 50%;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--line);
}

.head {
  display: flex;
  align-items: center;
  flex: none;
  gap: 6px;
  height: 28px;
  padding: 0 4px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--muted);
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}

.tree {
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow-y: auto;
  scrollbar-width: none;
}

.row {
  display: flex;
  align-items: center;
  flex: none;
  gap: 8px;
  height: 30px;
  padding-right: 6px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.row:hover,
.head:hover {
  background: var(--accent-tint);
}

.row:focus-visible,
.head:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

.row.active {
  background: var(--accent-soft);
}

.icon {
  flex: none;
  fill: none;
  stroke: var(--icon);
  stroke-width: 1.5;
  stroke-linejoin: round;
}

.title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.count {
  flex: none;
  color: var(--muted);
  font-size: 12px;
}

/* A bookmark open in a tab of this window. */
.open {
  flex: none;
  width: 6px;
  height: 6px;
  margin: 0 4px;
  border-radius: 50%;
  background: var(--text);
}

.chevron {
  flex: none;
  width: 6px;
  height: 6px;
  margin: 0 3px 3px;
  border-right: 1.5px solid var(--icon);
  border-bottom: 1.5px solid var(--icon);
  transform: rotate(45deg);
}

.chevron.collapsed {
  margin-bottom: 0;
  transform: rotate(-45deg);
}
</style>
