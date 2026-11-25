<script setup>
import { nextTick, ref, watch } from 'vue';
import { host } from './bookmarks.js';
import SiteMark from './SiteMark.vue';
import { folderMark } from '@/ui/tint.js';
import { useTints } from '@/ui/useTints.js';

// The command box narrowed to bookmarks (CommandBox.vue): the folders on the left, the one
// chosen marked, and on the right its bookmarks that hold what is typed, then those
// elsewhere after a rule, each saying its folder when it isn't the one chosen. The keys are
// the box's; this draws, and says what a press chose. The keys it has are in its foot.
const props = defineProps({ id: String, folders: Array, at: Number, results: Array, highlight: Number, typed: Boolean });
const emit = defineEmits(['folder', 'hover', 'choose']);

// What the keys choose stays in sight.
const root = ref(null);
const tints = useTints();
const folderColour = (id) => (tints.value, folderMark(id, 'bar'));
watch(
  () => [props.at, props.highlight],
  () => nextTick(() => root.value?.querySelectorAll('.chosen, .active').forEach((el) => el.scrollIntoView({ block: 'nearest' }))),
);
</script>

<template>
  <div ref="root" v-shell class="finder" @mousedown.prevent>
    <div class="columns">
      <ul class="folders" role="listbox" aria-label="Folders">
        <li
          v-for="(f, i) in folders"
          :key="f.id"
          role="option"
          :aria-selected="i === at"
          :class="{ chosen: i === at }"
          :style="{ paddingLeft: 10 + Math.max(0, f.depth - 1) * 12 + 'px' }"
          @click="emit('folder', i)"
        >
          <svg v-if="i === 0" class="icon" viewBox="0 0 16 16" aria-hidden="true"><path d="m8 2.5 1.7 3.5 3.8.5-2.8 2.6.7 3.8L8 11.1l-3.4 1.8.7-3.8-2.8-2.6 3.8-.5z" /></svg>
          <svg v-else class="icon" viewBox="0 0 16 16" aria-hidden="true" :style="{ stroke: folderColour(f.id) }"><path d="M2.5 4.5a1 1 0 0 1 1-1h3l1.5 1.5h4.5a1 1 0 0 1 1 1v5.5a1 1 0 0 1-1 1h-9a1 1 0 0 1-1-1z" /></svg>
          <span class="title">{{ f.title }}</span>
        </li>
      </ul>
      <ul :id="id" class="results" role="listbox" aria-label="Bookmarks">
        <template v-for="(r, i) in results" :key="r.e.id">
          <li v-if="r.elsewhere && !results[i - 1]?.elsewhere" class="rule" role="separator"></li>
          <li
            :id="`${id}-${i}`"
            role="option"
            :aria-selected="i === highlight"
            :class="{ active: i === highlight }"
            @mouseenter="emit('hover', i)"
            @click="emit('choose', i)"
          >
            <SiteMark :url="r.e.url" />
            <span class="lines">
              <span class="title">{{ r.e.title || host(r.e.url) }}</span>
              <span class="detail">{{ host(r.e.url) }}</span>
            </span>
            <span v-if="r.folder" class="where">in {{ r.folder }}</span>
          </li>
        </template>
        <li v-if="!results.length" class="none" role="presentation">{{ typed ? 'No bookmarks hold that' : 'No bookmarks in this folder' }}</li>
      </ul>
    </div>
    <div class="keys" aria-hidden="true">
      <span><kbd>↑↓</kbd> choose</span>
      <span><kbd>←→</kbd> folders</span>
      <span><kbd>↩</kbd> open</span>
      <span><kbd>⇧↩</kbd> open folder as a group</span>
    </div>
  </div>
</template>

<style scoped>
/* The design system's floating surface, as the box's list has. */
.finder {
  display: flex;
  flex-direction: column;
  flex: none;
  width: min(520px, 86%);
  margin-top: 8px;
  overflow: hidden;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  box-shadow: var(--shadow-float);
  color: var(--text);
  font-size: calc(13rem / 16);
  box-sizing: border-box;
}

.columns {
  display: flex;
  max-height: 340px;
  min-height: 0;
}

ul {
  margin: 0;
  padding: 4px;
  list-style: none;
  overflow-y: auto;
  scrollbar-width: none;
}

.folders {
  flex: none;
  width: 168px;
  border-right: 1px solid var(--line);
  background: var(--bar);
  box-sizing: border-box;
}

.results {
  flex: 1;
  min-width: 0;
}

[role='option'] {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 10px;
  border-radius: 4px;
  white-space: nowrap;
  cursor: pointer;
}

.folders [role='option'] {
  height: 32px;
}

.folders [role='option']:hover {
  background: var(--accent-tint);
}

.folders .chosen {
  background: var(--accent-soft);
  font-weight: 500;
}

.results [role='option'] {
  height: 44px;
}

.results .active {
  background: var(--accent-tint);
}

.icon {
  flex: none;
  width: 16px;
  height: 16px;
  fill: none;
  stroke: var(--icon);
  stroke-width: 1.5;
  stroke-linejoin: round;
}

.lines {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.title,
.detail {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.detail,
.where {
  color: var(--muted);
  font-size: calc(12rem / 16);
}

.where {
  flex: none;
}

.rule {
  height: 1px;
  margin: 4px 6px;
  background: var(--line);
}

.none {
  padding: 12px 10px;
  color: var(--muted);
}

.keys {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
  padding: 8px 12px;
  border-top: 1px solid var(--line);
  color: var(--muted);
  font-size: calc(12rem / 16);
}

kbd {
  margin-right: 4px;
  padding: 0 4px;
  border: 1px solid var(--line);
  border-radius: 4px;
  font: inherit;
}
</style>
