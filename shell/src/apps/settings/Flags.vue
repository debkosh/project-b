<script setup>
import { computed, inject } from 'vue';
import Field from '@/ui/Field.vue';
import Row from './Row.vue';
import { send, state } from './state.js';

// Flags, as core/flags/registry.json defines them, each drawn by its type: a switch for a
// boolean, a select for an enum, a field for the rest. A change waits for the next launch:
// each changed flag says so, and one Relaunch at the top applies them all.

// Where the page's own heading names the section, its own gives way.
const titled = inject('settingsTitled', false);

function set(flag, text) {
  send({ t: 'setFlag', id: flag.id, text });
}

const pending = computed(() => state.flags.filter((f) => f.pending).length);

// pattern is a flag's pattern as a JavaScript one. Go's syntax and JavaScript's agree on
// the simple patterns the registry has; one they don't is left to Go to check.
function pattern(flag) {
  try {
    return flag.pattern ? new RegExp(flag.pattern) : null;
  } catch {
    return null;
  }
}

// error is what is wrong with a value typed for a flag, as Go's flags.normal checks it,
// or ''. Empty is Off, and always good.
function error(flag, text) {
  if (!text) return '';
  const list = flag.type === 'numbers' || flag.type === 'strings';
  const items = list ? text.split(',').map((s) => s.trim()) : [text];
  if (flag.type === 'number' || flag.type === 'numbers') {
    const bad = items.find((s) => !/^[+-]?\d+$/.test(s) || (flag.min != null && +s < flag.min) || (flag.max != null && +s > flag.max));
    if (bad === undefined) return '';
    const n = list ? 'whole numbers' : 'a whole number';
    if (flag.min != null && flag.max != null) return `Use ${n} from ${flag.min} to ${flag.max}`;
    if (flag.min != null) return `Use ${n} of ${flag.min} or more`;
    if (flag.max != null) return `Use ${n} up to ${flag.max}`;
    return `Use ${n}`;
  }
  const p = pattern(flag);
  const bad = items.find((s) => !s || (p && !p.test(s)));
  if (bad === undefined) return '';
  return bad ? `"${bad}" isn't a value this flag takes` : 'Leave no empty value between commas';
}
</script>

<template>
  <section aria-label="Flags">
    <h2 v-if="!titled">Flags</h2>
    <div class="card">
      <Row v-if="pending" :label="`Relaunch to apply ${pending} ${pending === 1 ? 'change' : 'changes'}`">
        <button type="button" class="text-button" @click="send({ t: 'relaunch' })">Relaunch</button>
      </Row>
      <Row
        v-for="f in state.flags"
        :key="f.id"
        :label="f.name"
        :hint="f.description"
        :for="f.type === 'boolean' || f.type === 'enum' ? 'flag-' + f.id : undefined"
      >
        <template #tags>
          <span v-if="!f.built" class="tag">Not in this build</span>
          <span v-else-if="f.pending" class="tag">Relaunch to apply</span>
        </template>
        <input
          v-if="f.type === 'boolean'"
          :id="'flag-' + f.id"
          type="checkbox"
          role="switch"
          class="switch"
          :disabled="!f.built"
          :checked="f.value === 'on'"
          :aria-checked="String(f.value === 'on')"
          @change="set(f, $event.target.checked ? 'on' : '')"
        />
        <select v-else-if="f.type === 'enum'" :id="'flag-' + f.id" class="value" :disabled="!f.built" :value="f.value" @change="set(f, $event.target.value)">
          <option value="">Off</option>
          <option v-for="c in f.choices" :key="c" :value="c">{{ c }}</option>
        </select>
        <Field
          v-else
          class="value"
          :disabled="!f.built"
          :label="f.name"
          placeholder="Off"
          :value="f.value"
          :validate="(text) => error(f, text)"
          @commit="set(f, $event)"
        />
      </Row>
    </div>
  </section>
</template>

<style scoped>
.value {
  flex: none;
  width: 200px;
}

.touch .value {
  width: 100%;
}
</style>
