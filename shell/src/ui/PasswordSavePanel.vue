<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { send, state } from '@/sdk/vue.js';
import { pushEscape } from './escape.js';

// The offer to save a password (docs/password-autofill.md), after a sign-in went through:
// the username, the password as dots, which Show shows as the person typed it a moment
// ago, and Save or Update, Not now, and Never for this site. Enter saves. Escape or a
// press outside only closes it: the offer waits at the key in the address field, which
// brings it back, until it is answered. It hangs from that key, as the star's panel hangs
// from the star: the caller marks it the shell's and places it.
const props = defineProps({ offer: Object });
const emit = defineEmits(['close']);

const root = ref(null);
const primary = ref(null);
const shown = computed(() => (state.passwordSaveShown?.offer === props.offer.id ? state.passwordSaveShown.password : ''));
const showing = ref(false);

function answer(choice) {
  send({ t: 'passwordSaveAnswer', offer: props.offer.id, choice });
  emit('close');
}
function show() {
  showing.value = !showing.value;
  if (showing.value && !shown.value) send({ t: 'passwordSaveShow', offer: props.offer.id });
}
function outside(e) {
  if (!root.value?.contains(e.target) && !e.target.closest?.('[data-password-key]')) emit('close');
}

let pop = null;
onMounted(() => {
  pop = pushEscape(() => emit('close'));
  document.addEventListener('pointerdown', outside, true);
  primary.value?.focus();
});
onUnmounted(() => {
  pop();
  document.removeEventListener('pointerdown', outside, true);
});
</script>

<template>
  <form ref="root" class="ui-password-save" role="dialog" :aria-label="offer.update ? 'Update password?' : 'Save password?'" @submit.prevent.stop="answer('save')">
    <h2 class="ui-password-save-head">{{ offer.update ? 'Update password?' : 'Save password?' }}</h2>
    <p class="ui-password-save-site">For <b>{{ offer.site }}</b></p>
    <div class="ui-password-save-row">
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <circle cx="8" cy="5.25" r="2.75" />
        <path d="M2.75 14a5.25 5.25 0 0 1 10.5 0" />
      </svg>
      <span class="ui-password-save-value" :class="{ none: !offer.username }">{{ offer.username || 'No username' }}</span>
    </div>
    <div class="ui-password-save-row">
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M5 7V5a3 3 0 0 1 6 0v2M3.75 7h8.5v6.5h-8.5z" />
      </svg>
      <span class="ui-password-save-value" :class="{ secret: showing && shown }">{{ showing && shown ? shown : '••••••••' }}</span>
      <button type="button" class="text-button quiet" :aria-label="showing ? 'Hide the password' : 'Show the password'" @click="show">{{ showing ? 'Hide' : 'Show' }}</button>
    </div>
    <div class="ui-password-save-buttons">
      <button type="button" class="text-button quiet never" @click="answer('never')">Never for this site</button>
      <button type="button" class="text-button" @click="answer('dismiss')">Not now</button>
      <button ref="primary" type="submit" class="text-button primary">{{ offer.update ? 'Update' : 'Save' }}</button>
    </div>
  </form>
</template>

<style>
.ui-password-save {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 340px;
  max-width: calc(100vw - 16px);
  box-sizing: border-box;
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  box-shadow: var(--shadow-float);
  color: var(--text);
}

.ui-password-save-head {
  margin: 0;
  font-size: calc(14rem / 16);
  font-weight: 600;
}

.ui-password-save-site {
  margin: 0 0 4px;
  color: var(--muted);
  font-size: calc(12rem / 16);
}

.ui-password-save-site b {
  color: var(--text);
  font-weight: 600;
}

.ui-password-save-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 32px;
}

.ui-password-save-row > svg {
  flex: none;
  color: var(--icon);
}

.ui-password-save-value {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ui-password-save-value.none {
  color: var(--muted);
}

.ui-password-save-value.secret {
  font:
    calc(13rem / 16)/20px ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
  user-select: text;
}

.ui-password-save-buttons {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}

.ui-password-save-buttons .never {
  margin-right: auto;
  margin-left: -8px;
}
</style>
