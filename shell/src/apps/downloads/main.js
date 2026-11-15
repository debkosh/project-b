import { createApp } from 'vue';
import '@/design.css';
import DownloadsApp from './DownloadsApp.vue';
import { phone } from '@/phone.js';
import { connect, inputs, mark } from './state.js';

mark('downloads.script');
inputs();
document.documentElement.classList.toggle('touch', phone);
createApp(DownloadsApp).mount('#app');
connect();
