<script setup>
import { computed, inject, nextTick, provide, ref, watch, watchEffect } from 'vue';
import { phone } from '@/phone.js';
import Cards from './Cards.vue';
import Icon from './Icon.vue';
import ListPage from './ListPage.vue';
import { lists, resolve, sections } from './arrange.js';
import { state } from './state.js';

// Layout A, sections in a list (docs/settings.md), as Chrome's: on a desktop the list at the
// left, with icons, and one section beside it; on a phone the list alone, each section with
// a line saying what it is set to, a press going in and the back button out again.

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
      <template v-for="(group, i) in [above, below]" :key="i">
        <div v-if="i === 1" class="nav-line" role="separator"></div>
        <button
          v-for="s in group"
          :key="s.id"
          type="button"
          class="nav-item"
          :class="{ below: s.below }"
          :data-section="s.id"
          :aria-current="shown.section?.id === s.id ? 'page' : undefined"
          @click="go(s.id)"
        >
          <Icon :name="s.icon" />
          {{ s.label }}
        </button>
      </template>
    </nav>
    <main ref="content" class="content" :class="{ phone }">
      <div class="column">
        <ListPage v-if="shown.list" :list="shown.list" :back="shown.section" />
        <template v-else-if="shown.section">
          <div v-if="phone" class="list-head">
            <button type="button" class="icon-button back" aria-label="Back to Settings" @click="go('')">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10 3.5 5.5 8l4.5 4.5" /></svg>
            </button>
            <h2 class="list-title">{{ shown.section.label }}</h2>
          </div>
          <Cards :key="shown.section.id" :section="shown.section" />
        </template>
        <!-- A phone's list of sections, each saying what it is set to. -->
        <template v-else>
          <div v-for="(group, i) in [above, below]" :key="i" class="card section-list" :class="{ below: i === 1 }">
            <button v-for="s in group" :key="s.id" type="button" class="row section-row" :data-section="s.id" @click="go(s.id)">
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

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
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
