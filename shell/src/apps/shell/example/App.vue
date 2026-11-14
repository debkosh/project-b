<script setup>
import { ref } from 'vue';
import { connect, controlSpace, send, setFrame, state, tabLabel } from '@/sdk/vue.js';
import { ExtensionsToolbar } from '@extensions';

setFrame(4);
const address = ref(null);
connect({ focusAddressBar: () => address.value.focus() });
</script>

<template>
  <div class="splash" :aria-busy="!state.loaded">Project B</div>
  <div v-if="state.loaded" class="window">
    <header v-shell v-drag :style="{ paddingLeft: controlSpace.left }">
      <button
        v-for="tab in state.tabs" :key="tab.id"
        role="tab" :data-tab="tab.id" :aria-selected="tab.id === state.active"
        @click="send({ t: 'activateTab', id: tab.id })"
      >{{ tabLabel(tab) }}</button>
      <button aria-label="New Tab" @click="send({ t: 'newTab' })">+</button>
      <button @click="send({ t: 'closeTab', id: state.active })">Close Tab</button>
      <button @click="send({ t: 'back' })">Back</button>
      <button @click="send({ t: 'forward' })">Forward</button>
      <button @click="send({ t: 'reload' })">Reload</button>
      <input ref="address" aria-label="Address"
        @keydown.enter="send({ t: 'navigate', text: $event.target.value })">
      <component :is="ExtensionsToolbar" v-if="ExtensionsToolbar" :extensions="state.extensions" />
      <button @click="send({ t: 'run', text: 'app.settings' })">Settings</button>
    </header>
    <main v-page="state.active"></main>
  </div>
</template>

<style>
.splash { position: fixed; inset: 0; display: grid; place-items: center; font: 500 15px/1 system-ui, sans-serif; letter-spacing: 0.02em; color: rgb(255 255 255 / 0.85); user-select: none; pointer-events: none; transition: opacity var(--motion) var(--ease), display var(--motion) allow-discrete; }
.splash[aria-busy='false'] { display: none; opacity: 0; }
.window { display: flex; flex-direction: column; height: 100%; transition: opacity var(--motion) var(--ease); }
@starting-style { .window { opacity: 0; } }
header { display: flex; align-items: center; gap: 4px; height: 44px; padding-right: 8px; background: var(--bar); color: var(--text); }
input { flex: 1; }
main { flex: 1; margin: 0 var(--frame-width) var(--frame-width); }
</style>
