import { createApp } from 'vue';
import '@/design.css';
import App from './App.vue';
import { connect, vDrag, vPage, vShell } from '@/sdk/vue.js';
import { findBar } from '@/ui/find.js';
import { addressField } from './address.js';
import { started } from './started.js';

createApp(App).directive('shell', vShell).directive('drag', vDrag).directive('page', vPage).mount('#app');
connect({
  snapshot: () => (started.value = true),
  suggestions: (m) => addressField.suggestions(m.id, m.items || []),
  focusAddressBar: () => addressField.focus(),
  focusFind: () => findBar.focus(),
});
