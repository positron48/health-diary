import { createApp } from 'vue'
import App from './App.vue'
import { router } from './app/router'
import './styles/tokens.css'
import './styles/base.css'

createApp(App).use(router).mount('#app')

if ('serviceWorker' in navigator && import.meta.env.PROD) {
  window.addEventListener('load', () => navigator.serviceWorker.register('/sw.js'))
}
