import { createApp } from 'vue';
import '@/design.css';
import HistoryApp from './HistoryApp.vue';
import { phone } from '@/phone.js';
import { connect, inputs, mark } from './state.js';

mark('history.script');
inputs();
document.documentElement.classList.toggle('touch', phone);
createApp(HistoryApp).mount('#app');
connect();
