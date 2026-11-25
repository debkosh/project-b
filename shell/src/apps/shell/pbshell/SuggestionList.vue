<script setup>
import { computed } from 'vue';
import { state } from '@/sdk/vue.js';

// Go's one suggestion list (docs/daily-driver.md), as the standard shell draws it wherever
// it is asked for: under the address field, or under the new tab's box. Rows keep Go's
// order; a heading goes before each run of open tabs, history, actions and bookmarks. The
// first row is what Enter does, and is chosen until the arrow keys move; one that completes
// the field (inline) has no heading, as it is what the field shows. With nothing typed
// (first), Go's history rows are the sites visited most.
const props = defineProps({ items: Array, highlight: Number, id: String, first: Boolean });
const emit = defineEmits(['choose', 'hover']);

const headings = computed(() => ({
  tab: 'Open tabs',
  history: props.first ? 'Frequently visited' : 'History',
  action: 'Actions',
  bookmark: 'Bookmarks',
}));

const rows = computed(() =>
  props.items.map((row, i) => ({
    row,
    i,
    heading: headings.value[row.kind] && !(i === 0 && row.inline) && (props.items[i - 1]?.kind !== row.kind || (i === 1 && props.items[0].inline)) ? headings.value[row.kind] : '',
  })),
);


const favicon = (row) => (row.kind === 'tab' ? state.tabs.find((t) => t.id === row.tab)?.favicon : '');
</script>

<template>
  <ul :id="id" v-shell class="suggestions" role="listbox" @mousedown.prevent>
    <template v-for="r in rows" :key="r.i">
      <li v-if="r.heading" class="heading" role="presentation">{{ r.heading }}</li>
      <li
        :id="`${id}-${r.i}`"
        role="option"
        :data-kind="r.row.kind"
        :aria-selected="r.i === highlight"
        :class="{ active: r.i === highlight }"
        @mouseenter="emit('hover', r.i)"
        @click="emit('choose', r.i)"
      >
        <img v-if="favicon(r.row)" class="icon" :src="favicon(r.row)" alt="" width="16" height="16" />
        <svg v-else class="icon" viewBox="0 0 16 16" aria-hidden="true">
          <path v-if="r.row.kind === 'address'" d="M3 8h10M9 4l4 4-4 4" />
          <template v-else-if="r.row.kind === 'search'"><circle cx="7" cy="7" r="4.25" /><path d="m10.25 10.25 3.25 3.25" /></template>
          <template v-else-if="r.row.kind === 'history'"><circle cx="8" cy="8" r="5.5" /><path d="M8 5v3l2 1.5" /></template>
          <path v-else-if="r.row.kind === 'action'" d="M9 2 4 9h4l-1 5 5-7H8z" />
          <path v-else-if="r.row.kind === 'bookmark'" d="m8 2.5 1.7 3.5 3.8.5-2.8 2.6.7 3.8L8 11.1l-3.4 1.8.7-3.8-2.8-2.6 3.8-.5z" />
          <template v-else><rect x="2.5" y="4" width="11" height="9" rx="1.5" /><path d="M2.5 6.5h11" /></template>
        </svg>
        <span class="title">{{ r.row.title || r.row.url }}</span>
        <span v-if="r.row.detail && r.row.kind !== 'action'" class="detail">{{ r.row.detail }}</span>
        <span v-if="r.row.kind === 'tab'" class="tag">Switch to tab</span>
        <span v-else-if="r.row.kind === 'action' && r.row.detail" class="key">{{ r.row.detail }}</span>
        <span v-else-if="r.i === highlight" class="key" aria-hidden="true">↩</span>
      </li>
    </template>
  </ul>
</template>

<style scoped>
/* The design system's floating surface. */
.suggestions {
  margin: 0;
  padding: 4px;
  list-style: none;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  box-shadow: var(--shadow-float);
  color: var(--text);
  font-size: calc(13rem / 16);
}

.heading {
  padding: 8px 10px 4px;
  color: var(--muted);
  font-size: calc(12rem / 16);
  font-weight: 500;
}

[role='option'] {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 32px;
  padding: 0 10px;
  border-radius: 4px;
  white-space: nowrap;
  cursor: pointer;
}

[role='option'].active {
  background: var(--accent-tint);
}

.icon {
  flex: none;
  width: 16px;
  height: 16px;
  fill: none;
  stroke: var(--icon);
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-linejoin: round;
}

img.icon {
  border-radius: 3px;
}

.title,
.detail {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.title {
  flex: 0 1 auto;
}

.detail {
  flex: 0 10 auto;
  color: var(--muted);
}

.tag,
.key {
  flex: none;
  margin-left: auto;
  padding-left: 12px;
  color: var(--muted);
  font-size: calc(12rem / 16);
}

.tag {
  height: 18px;
  padding: 0 6px;
  border: 1px solid var(--line);
  border-radius: 4px;
  display: flex;
  align-items: center;
  font-size: calc(11rem / 16);
}

/* Touch: suggestions a finger hits (docs/large-screens.md). */
@media (pointer: coarse) and (not (any-pointer: fine)) {
  [role='option'] {
    height: 44px;
  }
}
</style>
