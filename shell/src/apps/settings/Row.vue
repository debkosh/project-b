<script setup>
import { computed, inject, onUnmounted, useId } from 'vue';
import Marked from './Marked.vue';
import { addRow, matches } from './search.js';

// One row of a card: its label, a second line under it, and its control on the right, in
// the default slot. Every layout draws its rows with this. A row that opens one of the
// long lists has opens, the list's address: the whole label is then a button, with what
// the list holds on its right and a chevron. tag is 'label' for a row that is one switch,
// so a press anywhere on it flips it; for names the control a plain row's label is for.
// The slot gets named, the attributes that name its control by the row's own words: its
// label as aria-label, and the line under it. Android reads no name from a label around a
// switch, nor from aria-labelledby: it reads aria-label.
// A plain row may lead with a mark (the lead slot), and draw its second line itself (the
// hint slot), hint still being its words for search; words are more that search finds it by.
const props = defineProps({
  label: String,
  hint: String,
  tag: { type: String, default: 'div' },
  for: String,
  opens: String,
  summary: String,
  words: String,
});

const go = inject('settingsGo');

const id = useId();
const named = computed(() => ({ 'aria-label': props.label, 'aria-describedby': props.hint ? id + '-hint' : undefined }));

// While search has words, the row shows only when its label, its second line or its
// section's name has them.
const section = inject('settingsSection', null);
const hit = computed(() => matches(props.label, props.hint, section?.label, props.words));
onUnmounted(addRow({ section: section?.id, hit: () => hit.value }));
</script>

<template>
  <component :is="tag" v-if="hit" class="row">
    <button v-if="opens" type="button" class="label open" :aria-label="label" :data-section="opens" @click="go(opens)">
      <span class="text">
        <Marked :text="label" />
        <span v-if="hint" class="secondary"><Marked :text="hint" /></span>
      </span>
      <span v-if="summary" class="summary">{{ summary }}</span>
      <svg class="chevron" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 3.5 10.5 8 6 12.5" /></svg>
    </button>
    <template v-else>
      <slot name="lead" />
      <!-- A control's name is its label alone; the line under it says more. -->
      <span class="label">
        <component :is="props.for ? 'label' : 'span'" class="name" :for="props.for"><span><Marked :text="label" /></span><slot name="tags" /></component>
        <span v-if="hint || $slots.hint" :id="id + '-hint'" class="secondary"><slot name="hint"><Marked :text="hint" /></slot></span>
      </span>
      <slot :named="named" />
    </template>
  </component>
</template>

<style scoped>
.name {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* The row opens a list: the label is the button, as wide as the row allows, with what the
   list holds and the chevron at its end. */
.open {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
}

.open:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
  border-radius: 4px;
}

.text {
  flex: 1;
  min-width: 0;
}

.summary {
  flex: none;
  max-width: 50%;
  overflow: hidden;
  color: var(--muted);
  font-size: calc(13rem / 16);
  font-weight: 400;
  /* Two lines at most, then an ellipsis: a larger font size shows the words. */
  display: -webkit-box;
  white-space: normal;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.chevron {
  flex: none;
  color: var(--icon);
}
</style>
