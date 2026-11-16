import { createApp } from 'vue';
import '@/design.css';
import App from './App.vue';
import { connect, vDrag, vPage, vShell } from '@/sdk/vue.js';
import { findBar } from '@/ui/find.js';
import { downloadsPanel } from '@/ui/downloads.js';
import { addressBar, newTabBox, openBox } from './state.js';

createApp(App).directive('shell', vShell).directive('drag', vDrag).directive('page', vPage).mount('#app');
connect({
  suggestions: (m) => {
    addressBar.suggestions(m.id, m.items || []);
    newTabBox.suggestions(m.id, m.items || []);
  },
  focusAddressBar: () => addressBar.focus(),
  focusFind: () => findBar.focus(),
  downloadStarted: () => downloadsPanel.started(),
  // Standard takes New Tab (core/shells/registry.json): Go asks it to open its box.
  action: (m) => m.id === 'tab.new' && openBox(),
});
