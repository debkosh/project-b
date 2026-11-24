<script setup>
import { computed, inject, nextTick, provide, ref, watch, watchEffect } from 'vue';
import { upTo } from '@/ui/escape.js';
import { phone } from '@/phone.js';
import Cards from './Cards.vue';
import Icon from './Icon.vue';
import ListPage from './ListPage.vue';
import Results from './Results.vue';
import SearchBox from './SearchBox.vue';
import StatusLine from './StatusLine.vue';
import { lists, partsOf, resolve, sections } from './arrange.js';
import { hits, searching } from './search.js';
import { state } from './state.js';

// The settings in sections (docs/settings.md), as Chrome's: on a desktop the list at the
// left, with icons, and one section beside it; on a phone the list alone, each section with
// a line saying what it is set to, a press going in and the back button out again. Search,
// at the top of the list, shows the rows that match from every section, under each
// section's name, and the list counts them; emptying it goes back to the section.

const go = inject('settingsGo');
// A phone's section has a heading of its own, over the section, which names it.
provide('settingsTitled', phone);
const jumps = inject('settingsJumps');
const content = ref(null);

// The section showing. A desktop shows the first when the address names none; a phone
// shows its list of sections.
const shown = computed(() => {
  const r = resolve(state.section, sections.value);
  if (!r.section && !phone) return { list: '', section: sections.value[0] };
  return r;
});

watchEffect(() => {
  const { list, section } = shown.value;
  if (phone) document.title = 'Settings';
  else document.title = `Settings › ${list ? lists[list].label : section.label}`;
});

const above = computed(() => sections.value.filter((s) => !s.below));
const below = computed(() => sections.value.filter((s) => s.below));

// Each new section starts at its top, and a part named in the address is scrolled to.
watch(
  () => [state.section, jumps.value],
  async () => {
    await nextTick();
    if (content.value) content.value.scrollTop = 0;
  },
  { flush: 'post' },
);
</script>

<template>
  <div class="body">
    <nav v-if="!phone" class="nav sections-nav" aria-label="Settings sections">
      <SearchBox class="nav-search" />
      <template v-for="(group, i) in [above, below]" :key="i">
        <div v-if="i === 1" class="nav-line" role="separator"></div>
        <button
          v-for="s in group"
          :key="s.id"
          type="button"
          class="nav-item"
          :class="{ below: s.below, dim: searching && !hits.by[s.id] }"
          :data-section="s.id"
          :data-parts="partsOf(s).join(' ')"
          :aria-current="!searching && shown.section?.id === s.id ? 'page' : undefined"
          @click="go(s.id)"
        >
          <Icon :name="s.icon" />
          {{ s.label }}
          <span v-if="searching && hits.by[s.id]" class="count">{{ hits.by[s.id] }}</span>
        </button>
      </template>
    </nav>
    <main ref="content" class="content" :class="{ phone, searching }">
      <div class="column">
        <!-- Search's results: every section's rows that match, under the section's name. -->
        <!-- A phone searches from its list of sections; the field stays as the results come. -->
        <SearchBox v-if="phone && (searching || !shown.section)" class="phone-search" />
        <!-- The status line tops a desktop's every section, and a phone's list. -->
        <StatusLine v-if="!searching && (phone ? !shown.section : !shown.list)" />
        <template v-if="searching">
          <Results />
          <section v-for="s in sections" v-show="hits.by[s.id]" :key="s.id" class="found" :aria-label="s.label">
            <h2 class="crumb"><Icon :name="s.icon" />{{ s.label }}</h2>
            <Cards :section="s" titled />
          </section>
        </template>
        <ListPage v-else-if="shown.list" :list="shown.list" :back="shown.section" />
        <template v-else-if="shown.section">
          <div v-if="phone" class="list-head">
            <button type="button" class="icon-button back" aria-label="Back to Settings" @click="upTo('', go)">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10 3.5 5.5 8l4.5 4.5" /></svg>
            </button>
            <h2 class="list-title">{{ shown.section.label }}</h2>
          </div>
          <Cards :key="shown.section.id" :section="shown.section" />
        </template>
        <!-- A phone's list of sections, each saying what it is set to. -->
        <template v-else>
          <div v-for="(group, i) in [above, below]" :key="i" class="card section-list" :class="{ below: i === 1 }">
            <button v-for="s in group" :key="s.id" type="button" class="row section-row" :data-section="s.id" :data-parts="partsOf(s).join(' ')" @click="go(s.id)">
              <span v-if="i === 0" class="section-icon"><Icon :name="s.icon" /></span>
              <span class="label">
                {{ s.label }}
                <span v-if="s.summary()" class="secondary">{{ s.summary() }}</span>
              </span>
              <svg class="chevron" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 3.5 10.5 8 6 12.5" /></svg>
            </button>
          </div>
        </template>
      </div>
    </main>
  </div>
</template>

<style scoped>
.sections-nav {
  width: 220px;
}

.nav-search {
  margin-bottom: 12px;
}

.phone-search {
  margin-bottom: 16px;
}

.searching .phone-search {
  margin-bottom: 0;
}

.found {
  margin-top: 20px;
}

.crumb {
  display: flex;
  align-items: center;
  gap: 6px;
}

.crumb .icon {
  color: var(--icon);
}

.nav-item .icon {
  flex: none;
  color: var(--icon);
}

.nav-item.below {
  color: var(--muted);
  font-weight: 400;
}

.nav-line {
  height: 1px;
  margin: 8px 12px;
  background: var(--line);
}

.list-head {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  margin: 0 0 8px -8px;
}

.list-title {
  color: var(--text);
  font-size: 17px;
  font-weight: 600;
}

.section-list + .section-list {
  margin-top: 16px;
}

/* Each section is one button the width of the card. */
.section-row {
  width: 100%;
  min-height: 60px;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
}

/* A section's name, and the line under it, each on one line: what doesn't fit ends in an
   ellipsis, however narrow the phone or large its type. */
.section-row .label,
.section-row .secondary {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.section-list .section-row + .section-row {
  border-top: 1px solid var(--line);
}

.section-row:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.below .label {
  color: var(--muted);
  font-weight: 400;
}

.section-icon {
  flex: none;
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: var(--accent-tint);
  color: var(--accent);
}

.chevron {
  flex: none;
  color: var(--icon);
}
</style>
