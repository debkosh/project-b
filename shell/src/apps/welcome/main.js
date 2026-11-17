import { createApp } from 'vue';
import '@/design.css';
import WelcomeApp from './WelcomeApp.vue';
import { phone } from '@/phone.js';
import { connect, inputs, mark } from './state.js';

mark('welcome.script');
inputs();
document.documentElement.classList.toggle('touch', phone);
createApp(WelcomeApp).mount('#app');
connect();
