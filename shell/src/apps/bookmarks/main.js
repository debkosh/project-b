import { createApp } from 'vue';
import '@/design.css';
import BookmarksApp from './BookmarksApp.vue';
import { phone } from '@/phone.js';
import { connect, inputs, mark } from './state.js';

mark('bookmarks.script');
inputs();
document.documentElement.classList.toggle('touch', phone);
createApp(BookmarksApp).mount('#app');
connect();
