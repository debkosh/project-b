<script setup>
import { computed, nextTick, ref, watch } from 'vue';
import { phone } from '@/phone.js';
import { openCustomColour } from './customColour.js';
import { inkOn } from './tint.js';
import { useTints } from './useTints.js';

// A colour from a row of presets, and with custom set, any colour behind the last swatch,
// which keeps a + on it so it reads as a picker: the system's picker, or on a phone our
// Custom colour sheet (customColour.js), its preview named item. It is one Tab stop; the
// arrow keys move the choice, as a radio group's do. colors holds '#rrggbb' strings, or
// { color, name, paint } where a preset has a name to be read out, and paint where its
// colour is drawn other than as itself, as a kept hue is (hues.js). A preset whose colour
// is '' is None, no colour: the surface with a stroke across it.
const props = defineProps({ modelValue: String, colors: Array, label: String, custom: Boolean, item: String });
const emit = defineEmits(['update:modelValue']);

const group = ref(null);
const list = computed(() => props.colors.map((c) => (typeof c === 'string' ? { color: c, name: c } : c)));
// The choice as the user last made it, ahead of the owner's answer, so quick presses
// count from it and focus follows it at once.
const current = ref(props.modelValue);
watch(() => props.modelValue, (v) => (current.value = v));
const isCustom = computed(() => !list.value.some((c) => c.color === current.value));

function choose(c) {
  current.value = c;
  emit('update:modelValue', c);
}

// A colour picked with + keeps the + on it, in the ink that reads on it, so it still reads
// as the picker.
const tints = useTints();
const customStyle = computed(() => (tints.value, isCustom.value && current.value ? { background: current.value, '--ui-swatch-ink': inkOn(current.value) } : null));

function openSheet() {
  openCustomColour({ color: current.value, name: props.item, onchange: choose });
}

async function onKey(e) {
  const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
  if (!step || e.target.type === 'color') return;
  e.preventDefault();
  const i = list.value.findIndex((c) => c.color === current.value);
  choose(list.value[(i + step + list.value.length) % list.value.length].color);
  await nextTick();
  group.value?.querySelector('[aria-checked="true"]')?.focus();
}
</script>

<template>
  <div ref="group" class="ui-swatches" role="radiogroup" :aria-label="label" @keydown="onKey">
    <button
      v-for="c in list"
      :key="c.color"
      type="button"
      role="radio"
      class="ui-swatch"
      :class="{ none: !c.color }"
      :aria-checked="c.color === current"
      :aria-label="c.name"
      :title="c.name"
      :tabindex="c.color === current || (isCustom && c === list[0]) ? 0 : -1"
      @click="choose(c.color)"
    >
      <span class="ui-swatch-dot" :style="{ background: c.paint || c.color }"></span>
    </button>
    <button v-if="custom && phone" type="button" class="ui-swatch custom" :class="{ on: isCustom }" aria-label="Custom colour" @click="openSheet">
      <span class="ui-swatch-dot" :style="customStyle"></span>
    </button>
    <label v-else-if="custom" class="ui-swatch custom" :class="{ on: isCustom }" title="Custom colour">
      <span class="ui-swatch-dot" :style="customStyle"></span>
      <input type="color" :value="current || '#808080'" aria-label="Custom colour" @input="choose($event.target.value)" />
    </label>
  </div>
</template>

<style>
.ui-swatches {
  flex: none;
  display: flex;
  align-items: center;
  gap: 8px;
}

/* A swatch is its target, holding its dot: the same 24px with a mouse, a 48px target round
   a 32px dot for a finger. */
.ui-swatch {
  position: relative;
  flex: none;
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: none;
  cursor: pointer;
}

.ui-swatch-dot {
  position: relative;
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  box-shadow: inset 0 0 0 1px var(--line);
}

/* The chosen one: a ring in the text colour, apart from the swatch by the surface. */
.ui-swatch[aria-checked='true'] .ui-swatch-dot,
.ui-swatch.custom.on .ui-swatch-dot {
  box-shadow: 0 0 0 2px var(--surface), 0 0 0 4px var(--text);
}

.ui-swatch:focus-visible,
.ui-swatch.custom:focus-within {
  outline: 2px solid var(--accent);
  outline-offset: 4px;
}

/* None: the surface, struck through. */
.ui-swatch.none .ui-swatch-dot {
  background: linear-gradient(135deg, transparent 47%, var(--muted) 47%, var(--muted) 53%, transparent 53%), var(--surface);
}

/* The custom swatch: a + on the surface, and once a colour is picked with it, that colour
   with the + on it in its ink. */
.ui-swatch.custom .ui-swatch-dot {
  background: var(--surface);
}

.ui-swatch.custom .ui-swatch-dot::before,
.ui-swatch.custom .ui-swatch-dot::after {
  content: '';
  position: absolute;
  width: 10px;
  height: 1.5px;
  border-radius: 1px;
  background: var(--ui-swatch-ink, var(--icon));
}

.ui-swatch.custom .ui-swatch-dot::after {
  transform: rotate(90deg);
}

.ui-swatch.custom input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0;
  border: 0;
  opacity: 0;
  cursor: pointer;
}

.touch .ui-swatches {
  flex-wrap: wrap;
  gap: 0;
}

.touch .ui-swatch {
  width: 48px;
  height: 48px;
}

.touch .ui-swatch-dot {
  width: 32px;
  height: 32px;
}

.touch .ui-swatch.custom .ui-swatch-dot::before,
.touch .ui-swatch.custom .ui-swatch-dot::after {
  width: 12px;
  height: 2px;
}
</style>
