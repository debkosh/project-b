<script setup>
import { choicesOf } from './imports.js';

// What a profile can bring over, a box each, with how many: the welcome's open row and
// Settings' Import dialog, which offers extensions too. The model is the ids ticked.
const props = defineProps({ profile: Object, extensions: Boolean });
const model = defineModel({ type: Array, default: () => [] });

function toggle(id, on) {
  model.value = on ? [...model.value, id] : model.value.filter((x) => x !== id);
}
</script>

<template>
  <div class="ui-import-choices" role="group" :aria-label="`What to bring over from ${props.profile.title}`">
    <label v-for="c in choicesOf(props.profile, { extensions: props.extensions })" :key="c.id" class="ui-import-choice">
      <input type="checkbox" :checked="model.includes(c.id)" @change="toggle(c.id, $event.target.checked)" />
      <span>{{ c.label }}</span>
      <span class="note">{{ c.note }}</span>
    </label>
  </div>
</template>

<style>
.ui-import-choices {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.ui-import-choice {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  padding: 0 12px 0 10px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
  font-size: 13px;
  color: var(--text);
}

.ui-import-choice input {
  margin: 0;
  accent-color: var(--accent);
}

.ui-import-choice .note {
  color: var(--muted);
}
</style>
