<script setup>
import { ref } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import Field from '@/ui/Field.vue';
import Row from './Row.vue';
import { send, state } from './state.js';

// This profile: its name in the list, and the template it was made from or last reset to.
// Reset to template asks which template in a dialog that says what goes, then writes that
// template's values over the profile's settings; Go owns all of it and answers with the
// profile as the list now has it. The shell it loads is a row of its own, with the look.

const nameError = (text) => (text ? '' : 'A profile needs a name');

const resetting = ref(false);
const chosen = ref('');

function askReset() {
  chosen.value = state.profile.template;
  resetting.value = true;
}

function reset() {
  send({ t: 'resetProfile', text: chosen.value });
  resetting.value = false;
}
</script>

<template>
  <Row label="Profile name">
    <Field
      class="name"
      label="Profile name"
      :value="state.profile.name"
      :validate="nameError"
      @commit="send({ t: 'setProfileName', text: $event })"
    />
  </Row>
  <Row label="Template" :hint="state.profile.template" class="template-row">
    <button type="button" class="text-button" @click="askReset">Reset to template…</button>
    <Dialog v-if="resetting" title="Reset this profile's settings?" @close="resetting = false">
      <div class="ui-dialog-part">
        <label for="profile-template">Template</label>
        <select id="profile-template" v-model="chosen" class="template">
          <option v-for="t in state.profile.templates" :key="t" :value="t">{{ t }}</option>
        </select>
      </div>
      <div class="ui-dialog-part">
        <p>Its settings become {{ chosen }}'s. History, cookies, logins and extensions stay.</p>
      </div>
      <template #buttons>
        <button type="button" class="text-button" @click="resetting = false">Cancel</button>
        <button type="button" class="text-button danger" @click="reset">Reset</button>
      </template>
    </Dialog>
  </Row>
</template>

<style scoped>
/* The name's field takes up to 320px of the row and gives way on a narrow page. */
.name {
  flex: 0 1 320px;
}

/* On a phone the field goes under its label, the row's width, as a flag's value does: beside
   it, the label was squeezed to a few letters. */
.touch .name {
  flex-basis: 100%;
}

.template {
  width: 100%;
}
</style>
