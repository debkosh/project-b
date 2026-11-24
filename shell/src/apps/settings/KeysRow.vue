<script setup>
import { computed, ref } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import Segmented from '@/ui/Segmented.vue';
import Row from './Row.vue';
import { send, state } from './state.js';

// Keys: the keymap's template, as Go lists them, and Reset all, which is the same template
// again. With shortcuts changed it asks first, the choice showing the one asked about
// until the answer. Each shortcut is edited in Keyboard shortcuts.
const kb = computed(() => state.keyboard);

// A template named for a shell, such as Curve's, is called by that shell's name, as the
// registry gives it.
const templateNames = { 'project-b': 'Project B', chrome: 'Chrome', firefox: 'Firefox' };
const labelOf = (name) => templateNames[name] || state.profile?.shells?.find((s) => s.id === name)?.name || name;
const options = computed(() => kb.value.templates.map((t) => ({ value: t, label: labelOf(t) })));

// How many actions differ from the template: what a reset or another template loses.
const changed = computed(() => kb.value.groups.reduce((n, g) => n + g.actions.filter((a) => a.changed).length, 0));

const asking = ref('');
function use(name) {
  if (changed.value === 0) send({ t: 'useTemplate', text: name });
  else asking.value = name;
}
function confirm() {
  send({ t: 'useTemplate', text: asking.value });
  asking.value = '';
}
const title = computed(() => {
  const n = changed.value;
  return `Discard ${n} changed shortcut${n === 1 ? '' : 's'} and use ${labelOf(asking.value)}'s?`;
});
</script>

<template>
  <Row label="Keys" :hint="changed ? `${changed} changed from the template` : 'Every shortcut as the template has it'">
    <Segmented label="Keys" :options="options" :model-value="asking || kb.template" @update:model-value="use" />
    <button type="button" class="text-button" :disabled="!changed" @click="use(kb.template)">Reset all…</button>
    <Dialog v-if="asking" :title="title" @close="asking = ''">
      <template #buttons>
        <button type="button" class="text-button" @click="asking = ''">Cancel</button>
        <button type="button" class="text-button danger" @click="confirm">Discard</button>
      </template>
    </Dialog>
  </Row>
</template>
