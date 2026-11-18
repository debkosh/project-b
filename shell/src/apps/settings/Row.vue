<script setup>
import { inject } from 'vue';

// One row of a card: its label, a second line under it, and its control on the right, in
// the default slot. Every layout draws its rows with this. A row that opens one of the
// long lists has opens, the list's address: the whole label is then a button, with what
// the list holds on its right and a chevron. tag is 'label' for a row that is one switch,
// so a press anywhere on it flips it; for names the control a plain row's label is for.
const props = defineProps({
  label: String,
  hint: String,
  tag: { type: String, default: 'div' },
  for: String,
  opens: String,
  summary: String,
});

const go = inject('settingsGo');
</script>

<template>
  <component :is="tag" class="row">
    <button v-if="opens" type="button" class="label open" :aria-label="label" :data-section="opens" @click="go(opens)">
      <span class="text">
        {{ label }}
        <span v-if="hint" class="secondary">{{ hint }}</span>
      </span>
      <span v-if="summary" class="summary">{{ summary }}</span>
      <svg class="chevron" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 3.5 10.5 8 6 12.5" /></svg>
    </button>
    <template v-else>
      <!-- A control's name is its label alone; the line under it says more. -->
      <span class="label">
        <component :is="props.for ? 'label' : 'span'" class="name" :for="props.for">{{ label }}<slot name="tags" /></component>
        <span v-if="hint" class="secondary">{{ hint }}</span>
      </span>
      <slot />
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
  font-size: 13px;
  font-weight: 400;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chevron {
  flex: none;
  color: var(--icon);
}
</style>
