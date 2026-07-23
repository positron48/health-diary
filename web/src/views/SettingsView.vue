<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { API_BASE, json, request } from '../api/client'
import { authApi } from '../api/auth'
import { settingsApi } from '../api/settings'
import { useSession } from '../composables/useSession'
import { useToast } from '../composables/useToast'
import UiButton from '../components/ui/UiButton.vue'
import UiDialog from '../components/ui/UiDialog.vue'

type PlaceHit = {
  provider_place_id: string
  label: string
  region?: string
  country_code: string
  timezone: string
  latitude: number
  longitude: number
}

const session = useSession()
const timezone = ref(session.user.value?.timezone || 'Europe/Moscow')
const dayStart = ref(session.user.value?.settings?.day_start_time || '00:00')
const homeQuery = ref('')
const homeLabel = ref('')
const homePlaceId = ref(session.user.value?.settings?.home_place_id || '')
const placeHits = ref<PlaceHit[]>([])
const danger = ref(false)
const busy = ref(false)
const toast = useToast()
const router = useRouter()

async function searchHome() {
  if (homeQuery.value.trim().length < 2) return
  busy.value = true
  try {
    const response = await request<{ places: PlaceHit[] }>(`${API_BASE}/places/search?q=${encodeURIComponent(homeQuery.value.trim())}`)
    placeHits.value = response.places || []
  } catch (e) {
    toast.show((e as Error).message)
  } finally {
    busy.value = false
  }
}

async function pickPlace(place: PlaceHit) {
  busy.value = true
  try {
    const saved = await request<{ id: string; label: string }>(`${API_BASE}/places`, json('POST', place))
    homePlaceId.value = saved.id
    homeLabel.value = saved.label
    placeHits.value = []
    toast.show(`Выбран город: ${saved.label}`)
  } catch (e) {
    toast.show((e as Error).message)
  } finally {
    busy.value = false
  }
}

async function save() {
  busy.value = true
  try {
    const settings: Record<string, string> = { day_start_time: dayStart.value }
    if (homePlaceId.value) settings.home_place_id = homePlaceId.value
    session.authenticated(await settingsApi.update({ timezone: timezone.value, locale: 'ru', settings }))
    toast.show('Настройки сохранены')
  } catch (e) {
    toast.show((e as Error).message)
  } finally {
    busy.value = false
  }
}

async function logout() {
  await authApi.logout()
  session.expire('/calendar')
  router.push('/login')
}

async function remove() {
  busy.value = true
  try {
    await settingsApi.deleteAccount()
    session.expire()
    router.push('/login')
  } catch (e) {
    toast.show((e as Error).message)
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <div class="page stack">
    <header>
      <p class="eyebrow">Профиль и данные</p>
      <h1>Ещё</h1>
    </header>
    <section class="card stack">
      <h2>Профиль</h2>
      <label class="field">Часовой пояс<input v-model="timezone" autocomplete="off"></label>
      <label class="field">
        Начало нового дня
        <input v-model="dayStart" type="time" step="60">
        <small class="muted">События до этого времени относятся к предыдущему дню. По умолчанию — 00:00.</small>
      </label>
      <label class="field">
        Домашний город
        <input v-model="homeQuery" placeholder="Липецк" autocomplete="off" @keyup.enter="searchHome">
        <small class="muted">
          Только город, без GPS. Погода подтягивается для дома и подтверждённых поездок.
          {{ homeLabel || homePlaceId ? `Сейчас: ${homeLabel || homePlaceId}` : 'Можно выбрать Липецк по умолчанию.' }}
        </small>
      </label>
      <div class="cluster">
        <UiButton variant="secondary" :busy="busy" @click="searchHome">Найти город</UiButton>
        <UiButton
          variant="ghost"
          @click="homePlaceId = '00000000-0000-4000-8000-000000000001'; homeLabel = 'Липецк'"
        >
          Липецк
        </UiButton>
      </div>
      <ul v-if="placeHits.length" class="place-hits">
        <li v-for="place in placeHits" :key="place.provider_place_id">
          <button type="button" class="button button--secondary" @click="pickPlace(place)">
            {{ place.label }}<span v-if="place.region">, {{ place.region }}</span>
          </button>
        </li>
      </ul>
      <UiButton :busy="busy" @click="save">Сохранить</UiButton>
    </section>
    <section class="card">
      <h2>Приватность</h2>
      <p>Исходные сообщения отделены от извлечённых фактов. Погода — Open-Meteo (CC BY 4.0), только центр выбранного города.</p>
      <RouterLink to="/privacy">Подробнее о данных и хранении</RouterLink>
    </section>
    <section class="card">
      <h2>Экспорт</h2>
      <p>Скачайте текущие подтверждённые данные. Ответ не кешируется приложением.</p>
      <div class="cluster">
        <a class="button button--secondary" :href="settingsApi.exportUrl('json')">JSON</a>
        <a class="button button--secondary" :href="settingsApi.exportUrl('csv')">CSV</a>
      </div>
    </section>
    <section class="card">
      <h2>Сессии</h2>
      <div class="cluster">
        <UiButton variant="secondary" @click="logout">Выйти</UiButton>
        <UiButton variant="ghost" @click="authApi.logoutAll().then(logout)">Выйти на всех устройствах</UiButton>
      </div>
    </section>
    <section class="card danger-zone">
      <h2>Опасная зона</h2>
      <p>Удаление затрагивает исходные записи, события, ревизии, экспорты, контекст, погоду и сессии сервиса. История чата Telegram удаляется отдельно.</p>
      <UiButton variant="danger" @click="danger = true">Удалить данные аккаунта</UiButton>
    </section>
    <UiDialog :open="danger" title="Удалить все данные?" @close="danger = false">
      <p>Операцию нельзя отменить. Сервер может запросить недавнюю повторную авторизацию.</p>
      <div class="cluster">
        <UiButton variant="danger" :busy="busy" @click="remove">Удалить всё</UiButton>
        <UiButton variant="secondary" @click="danger = false">Отмена</UiButton>
      </div>
    </UiDialog>
  </div>
</template>
<style scoped>
.danger-zone { margin-top: var(--s7); border-color: #e6b6b3; }
.place-hits { list-style: none; padding: 0; margin: 0; display: grid; gap: var(--s2); }
</style>
