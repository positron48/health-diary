import { createRouter, createWebHistory } from 'vue-router'
import { useSession } from '../composables/useSession'
import AppShell from './AppShell.vue'
const view = (name: string) => () => import(`../views/${name}.vue`)
export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/today' }, { path: '/login', component: view('LoginView'), meta: { public: true, title: 'Вход' } },
    { path: '/privacy', component: view('PrivacyView'), meta: { public: true, title: 'Приватность' } },
    { path: '/', component: AppShell, children: [
      { path: 'today', component: view('TodayView'), meta: { title: 'Сегодня' } },
      { path: 'pending', component: view('PendingView'), meta: { title: 'Входящие' } },
      { path: 'calendar/:month?', component: view('CalendarView'), meta: { title: 'Календарь' } },
      { path: 'day/:date', component: view('DayView'), meta: { title: 'День' } },
      { path: 'entries/new', component: view('EntryCreateView'), meta: { title: 'Новая запись' } },
      { path: 'events/:id/edit', component: view('EventEditView'), meta: { title: 'Изменение события' } },
      { path: 'episodes/:id', component: view('EpisodeView'), meta: { title: 'Эпизод' } },
      { path: 'analytics', component: view('AnalyticsView'), meta: { title: 'Аналитика' } },
      { path: 'settings', component: view('SettingsView'), meta: { title: 'Настройки' } },
    ] },
    { path: '/:pathMatch(.*)*', redirect: '/today' },
  ],
  scrollBehavior: () => ({ top: 0 }),
})
router.beforeEach(async (to) => {
  const session = useSession()
  if (!session.ready.value) await session.bootstrap()
  if (!to.meta.public && !session.user.value) return { path: '/login', query: { return: to.fullPath } }
  if (to.path === '/login' && session.user.value) return '/today'
  document.title = `${String(to.meta.title || 'Дневник')} — Дневник здоровья`
})
