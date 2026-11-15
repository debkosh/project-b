<script setup>
import { computed } from 'vue';
import { state } from '@/sdk/vue.js';

// Go's one suggestion list (docs/daily-driver.md), as the standard shell draws it wherever
// it is asked for: under the address field, or under the new tab's box. Rows keep Go's
// order; a heading goes before each run of open tabs, history and actions. The first row is
// what Enter does, and is chosen until the arrow keys move.
const props = defineProps({ items: Array, highlight: Number, id: String });
const emit = defineEmits(['choose', 'hover']);

const headings = { tab: 'Open tabs', history: 'History', action: 'Actions' };

const rows = computed(() =>
  props.items.map((row, i) => ({
    row,
    i,
    heading: headings[row.kind] && props.items[i - 1]?.kind !== row.kind ? headings[row.kind] : '',
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
  font-size: 13px;
}

.heading {
  padding: 8px 10px 4px;
  color: var(--muted);
  font-size: 12px;
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
  font-size: 12px;
}

.tag {
  height: 18px;
  padding: 0 6px;
  border: 1px solid var(--line);
  border-radius: 4px;
  display: flex;
  align-items: center;
  font-size: 11px;
}
</style>
