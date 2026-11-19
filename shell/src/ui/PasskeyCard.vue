<script setup>
import { computed, nextTick, onMounted, ref } from 'vue';
// The design system's dialog styles: .ui-dialog, its title and its buttons.
import '@/ui/Dialog.vue';
import { checkPin, codePath, codeSize, firstControl, trapTab } from '@/ui/passkey.js';
import { answerPasskey, focusPage } from '@/sdk/vue.js';

// One step of a page's passkey request (docs/passkeys.md), as Go words it: the card of a
// page dialog of kind passkey. Its parent makes a new one for each step, so each starts
// with empty fields and the keyboard on its first control. The card stays after an
// answer: Go sends the next step, or takes the dialog away.
const props = defineProps({ dialog: Object });

const step = computed(() => props.dialog.passkey);
const hostAt = computed(() => (props.dialog.host ? props.dialog.message.indexOf(props.dialog.host) : -1));
const card = ref(null);
const values = ref(['', '']);
// The words under each PIN field: the key's, of the PIN it refused, then our own.
const errors = ref([step.value.pin?.error || '', '']);

onMounted(() => nextTick(() => firstControl(card.value)?.focus()));

function act(action, text = '') {
  answerPasskey(props.dialog.id, action, text);
  if (action === 'cancel') focusPage();
}

function next() {
  const wrong = checkPin(step.value.pin, values.value);
  errors.value = ['', ''];
  if (wrong) {
    errors.value[wrong.field] = wrong.words;
    card.value.querySelectorAll('input')[wrong.field].focus();
    return;
  }
  act('pin', values.value[0]);
}

function primary() {
  if (step.value.primary === 'pin') next();
  else act(step.value.primary);
}
</script>

<template>
  <div ref="card" class="ui-dialog card" :class="{ wide: step.wide }" role="alertdialog" aria-modal="true" :aria-label="dialog.title" @keydown="trapTab(card, $event)">
    <h2 class="ui-dialog-title">{{ dialog.title }}</h2>
    <p v-if="dialog.message" class="words">
      <template v-if="hostAt >= 0">{{ dialog.message.slice(0, hostAt) }}<b>{{ dialog.host }}</b>{{ dialog.message.slice(hostAt + dialog.host.length) }}</template>
      <template v-else>{{ dialog.message }}</template>
    </p>
    <template v-if="step.pin">
      <div v-for="(label, i) in step.pin.labels" :key="label" class="group">
        <label :for="`pb-passkey-pin-${i}`">{{ label }}</label>
        <input
          :id="`pb-passkey-pin-${i}`"
          v-model="values[i]"
          class="text-field"
          type="password"
          autocomplete="off"
          :aria-invalid="!!errors[i]"
          :aria-describedby="errors[i] ? `pb-passkey-pin-error-${i}` : null"
          @input="errors[i] = ''"
          @keydown.enter.prevent="next"
        />
        <p v-if="errors[i]" :id="`pb-passkey-pin-error-${i}`" class="text-field-error" role="alert">{{ errors[i] }}</p>
      </div>
    </template>
    <div v-if="step.accounts" class="accounts">
      <button v-for="(account, i) in step.accounts" :key="i" type="button" class="account" :title="account.secondary ? `${account.title}\n${account.secondary}` : account.title" @click="act('account', String(i))">
        <span class="name" :class="{ unnamed: account.unnamed }">{{ account.title }}</span>
        <span v-if="account.secondary" class="secondary name">{{ account.secondary }}</span>
      </button>
    </div>
    <template v-if="step.code">
      <div class="code">
        <svg role="img" aria-label="QR code for your phone" :viewBox="`0 0 ${codeSize(step.code)} ${codeSize(step.code)}`" shape-rendering="crispEdges">
          <path :d="codePath(step.code)" fill="#000" />
        </svg>
      </div>
      <div v-if="step.bluetooth" class="bluetooth">
        <span>{{ step.bluetooth.words }}</span>
        <button v-if="step.bluetooth.button" type="button" class="text-button" @click="act(step.bluetooth.button.action)">{{ step.bluetooth.button.label }}</button>
      </div>
    </template>
    <div class="ui-dialog-buttons buttons">
      <button v-if="step.other" type="button" class="text-button quiet other" @click="act(step.other.action)">{{ step.other.label }}</button>
      <button v-if="dialog.cancel" type="button" class="text-button" @click="act('cancel')">{{ dialog.cancel }}</button>
      <button v-if="dialog.ok" type="button" class="text-button primary" @click="primary">{{ dialog.ok }}</button>
    </div>
  </div>
</template>

<style scoped>
.card {
  display: block;
}

/* A card of ours with the way to a security key on its left: wide enough for that, Cancel
   and the primary button on one row. */
.card.wide {
  width: 440px;
}

.words {
  margin: 0;
  overflow-wrap: anywhere;
}

/* The card's 16px rhythm: the buttons sit 16px under what is above them, the first field
   16px under the words and 12px between two, each label 4px over its field. */
.card .ui-dialog-buttons {
  margin-top: 16px;
}

.group {
  margin-top: 16px;
}

.group + .group {
  margin-top: 12px;
}

.group label {
  display: block;
  margin: 0 0 4px;
}

.group .text-field {
  width: 100%;
  box-sizing: border-box;
}

.card .text-field-error {
  margin: 4px 0 0;
  color: var(--danger);
}

/* A key's accounts, in a card of their own 16px under the words: a hairline round it and
   between its rows, each row a button of the card's full width. About five show before
   the list scrolls inside the card, so Cancel stays in view. */
.accounts {
  max-height: 244px;
  margin-top: 16px;
  border: 1px solid var(--line);
  border-radius: 6px;
  overflow-y: auto;
}

.account {
  display: flex;
  flex-direction: column;
  justify-content: center;
  width: 100%;
  height: 48px;
  padding: 8px 12px;
  box-sizing: border-box;
  border: 0;
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: default;
}

.account + .account {
  border-top: 1px solid var(--line);
}

/* An account the key has no name for. */
.unnamed {
  color: var(--muted);
}

.account:hover {
  background: var(--accent-tint);
}

.account:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring-inset);
}

.name {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* The phone's code: dark on a white tile whatever the theme, never dimmed, its quiet zone
   inside the tile. */
.code {
  width: 200px;
  height: 200px;
  margin: 16px auto 0;
  border-radius: 4px;
  background: #fff;
  overflow: hidden;
}

.code svg {
  display: block;
  width: 100%;
  height: 100%;
}

.bluetooth {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
}

.bluetooth span {
  flex: 1;
  color: var(--muted);
  font-size: 12px;
}

.bluetooth .text-button {
  flex: none;
  font-size: 13px;
}

/* The way to another kind of key sits apart, on the left. */
.other {
  margin-right: auto;
}
</style>
