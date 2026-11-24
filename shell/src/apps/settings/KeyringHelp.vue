<script setup>
import { ref } from 'vue';
import Dialog from '@/ui/Dialog.vue';
import { send, state } from './state.js';

// Learn more, beside the keyring's line (keyring.js): why the desktop's keyring gave no key,
// and what to do, in a dialog. GNOME's keyring can stop answering every app until the next
// login, when an app closes while its unlock question is up (docs/research/linux.md, 4).
// Read more opens the help page, docs/help/keyring.md (core/links, "keyring"), which has the
// way round that without logging out.
const open = ref(false);
const readMore = () => {
  open.value = false;
  send({ t: 'openHelp', page: 'keyring' });
};
</script>

<template>
  <button type="button" class="link" @click="open = true">Learn more</button>
  <Dialog v-if="open" title="Why is my keyring locked?" @close="open = false">
    <div class="ui-dialog-part">
      <p>Project B keeps its key in your desktop's keyring. While the keyring is locked, your saved passwords, passkeys and sync stay closed.</p>
    </div>
    <div class="ui-dialog-part">
      <p>If the keyring asked and you pressed Cancel, try again and unlock it.</p>
    </div>
    <div class="ui-dialog-part">
      <p v-if="state.desktop === 'gnome'">
        If it doesn't ask at all, GNOME's keyring has stopped answering. That happens when an app closes while the keyring's unlock question is up. Log out and back in, and it
        works again.
      </p>
      <p v-else>If it doesn't ask at all, the keyring has stopped answering. Log out and back in, and it works again.</p>
    </div>
    <template #buttons>
      <button type="button" class="text-button" @click="readMore">Read more</button>
      <!-- Close is the default, focused as the Dialog opens: Enter only dismisses it, as Read
           more leaves for a web page. -->
      <button type="button" class="text-button" autofocus @click="open = false">Close</button>
    </template>
  </Dialog>
</template>
