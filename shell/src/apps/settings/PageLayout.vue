<script setup>
import { computed, inject, nextTick, ref, watch, watchEffect } from 'vue';
import { phone } from '@/phone.js';
import Cards from './Cards.vue';
import ListPage from './ListPage.vue';
import Results from './Results.vue';
import SearchBox from './SearchBox.vue';
import StatusLine from './StatusLine.vue';
import { lists, pageGroups, resolve } from './arrange.js';
import { hits, searching } from './search.js';
import { state } from './state.js';

// Layout B, one page by task (docs/settings.md): every group on one scrolling page, a rail
// at its left on a desktop and chips over it on a phone, each jumping down to its group.
// The long lists open as pages of their own, in place of the page, with a link back.
// Search, over it all, filters the page to the rows that match, in their groups, and the
// rail counts them; emptying it puts the page back as it was.

const go = inject('settingsGo');
const jumps = inject('settingsJumps');
const content = ref(null);

const shown = computed(() => {
  const r = resolve(state.section, pageGroups.value);
  return searching.value ? { ...r, list: '' } : r;
});

watchEffect(() => {
  const list = shown.value.list;
  document.title = list && !phone ? `Settings › ${lists[list].label}` : 'Settings';
});

// The group at the top of the page, which the rail and the chips mark. A short group at
// the foot never reaches the top: with the page scrolled to its end, the group last
// jumped to is the one marked.
const current = ref('');
let jumpedTo = '';
function onScroll() {
  const c = content.value;
  // A scroll can land as the other layout takes the page's place.
  if (!c) return;
  const top = c.getBoundingClientRect().top;
  const groups = [...c.querySelectorAll('.group')];
  const at = groups.filter((g) => g.getBoundingClientRect().top - top <= 48).pop() || groups[0];
  const end = c.scrollTop > 0 && c.scrollTop + c.clientHeight >= c.scrollHeight - 2;
  current.value = end && jumpedTo ? jumpedTo : at?.dataset.group || '';
}

// An address, from the rail, a back link or Go, shows its page, or scrolls the page to the
// block it names or else its group; '' is the page's top.
watch(
  () => [state.section, jumps.value],
  async () => {
    await nextTick();
    if (!content.value) return;
    const g = shown.value.section;
    const el = !shown.value.list && g && state.section ? content.value.querySelector(`[data-part="${state.section}"], [data-group="${g.id}"]`) : null;
    jumpedTo = el ? g.id : '';
    if (el) el.scrollIntoView({ block: 'start' });
    else content.value.scrollTop = 0;
    onScroll();
  },
  { flush: 'post', immediate: true },
);
</script>

<template>
  <div class="body">
    <nav v-if="!phone" class="nav" aria-label="Settings groups">
      <button
        v-for="g in pageGroups"
        :key="g.id"
        type="button"
        class="nav-item"
        :data-section="g.id"
        :class="{ dim: searching && !hits.by[g.id] }"
        :aria-current="!searching && (shown.list ? shown.section.id : current) === g.id ? 'true' : undefined"
        @click="go(g.id)"
      >
        {{ g.label }}
        <span v-if="searching && hits.by[g.id]" class="count">{{ hits.by[g.id] }}</span>
      </button>
    </nav>
    <main ref="content" class="content" :class="{ phone, searching }" @scroll.passive="onScroll">
      <div class="column">
        <SearchBox class="top-search" />
        <ListPage v-if="shown.list" :list="shown.list" :back="shown.section" />
        <template v-else>
          <Results v-if="searching" />
          <nav v-if="phone && !searching" class="chips" aria-label="Settings groups">
            <button
              v-for="g in pageGroups"
              :key="g.id"
              type="button"
              class="chip"
              :data-section="g.id"
              :aria-current="current === g.id ? 'true' : undefined"
              @click="go(g.id)"
            >{{ g.label }}</button>
          </nav>
          <StatusLine v-if="!searching" />
          <section v-for="g in pageGroups" v-show="!searching || hits.by[g.id]" :key="g.id" class="group" :data-group="g.id" :aria-labelledby="'group-' + g.id">
            <div class="group-head">
              <h2 :id="'group-' + g.id" class="group-title">{{ g.label }}</h2>
              <span v-if="!phone" class="group-hint">{{ g.hint }}</span>
            </div>
            <Cards :section="g" />
          </section>
        </template>
      </div>
    </main>
  </div>
</template>

<style scoped>
.top-search {
  margin-bottom: 16px;
}

.chips {
  display: flex;
  gap: 8px;
  margin: -4px -16px 8px;
  padding: 0 16px 8px;
  overflow-x: auto;
  scrollbar-width: none;
}

.chip {
  flex: none;
  height: 36px;
  padding: 0 16px;
  border: 1px solid var(--line);
  border-radius: 18px;
  background: var(--surface);
  color: inherit;
  font: inherit;
  font-weight: 500;
}

.chip[aria-current='true'] {
  border-color: transparent;
  background: var(--accent-soft);
}

.chip:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.searching .group {
  margin-top: 20px;
}

.group + .group {
  margin-top: 32px;
}

.group-head {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 4px;
}

.group-title {
  color: var(--text);
  font-size: 17px;
  font-weight: 600;
}

.group-hint {
  color: var(--muted);
  font-size: 13px;
}
</style>
