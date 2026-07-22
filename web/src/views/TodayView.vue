<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { journalApi } from '../api/journal'
import { useAsyncState } from '../composables/useAsyncState'
import EventCard from '../features/events/EventCard.vue'
import StatePanel from '../components/ui/StatePanel.vue'
import { useSession } from '../composables/useSession'
import { userDate } from '../utils/userDay'
const session = useSession()
const today = session.user.value?.current_local_date || userDate(new Date(), session.user.value?.timezone || 'Europe/Moscow', session.user.value?.settings?.day_start_time)
const { data, loading, error, load } = useAsyncState(async () => {
  try { return await journalApi.day(today) } catch { const list = await journalApi.events(`?from=${today}&to=${today}`); return { date: today, events: list.events, pending_count: 0 } }
})
const openEpisode = computed(() => data.value?.episodes?.find((episode) => episode.status === 'open'))
onMounted(() => load())
</script>
<template><div class="page stack"><header class="page-header"><div><p class="eyebrow">Личный дневник</p><h1>Сегодня</h1><p class="muted">{{ new Intl.DateTimeFormat('ru-RU',{dateStyle:'full',timeZone:'UTC'}).format(new Date(`${today}T12:00:00Z`)) }}</p></div></header><StatePanel v-if="loading" kind="loading" /><StatePanel v-else-if="error" kind="error" :message="error" @retry="load()" /><template v-else-if="data"><RouterLink v-if="data.pending_count" class="card pending-banner" to="/pending"><strong>{{ data.pending_count }} записей ждут проверки</strong><span>Проверить →</span></RouterLink><section v-if="openEpisode" class="card"><p class="eyebrow">Открытый эпизод</p><h2>Головная боль продолжается</h2><p>Начало: {{ new Date(openEpisode.started_at).toLocaleString('ru-RU') }}</p><RouterLink :to="`/episodes/${openEpisode.id}`">Открыть эпизод</RouterLink></section><section class="stack"><h2>Хронология</h2><EventCard v-for="event in data.events" :key="event.id" :event="event" /><StatePanel v-if="!data.events.length" kind="empty" title="За сегодня нет записей" message="Напишите боту, например: «Около трёх заболела голова, 6 из 10»." /></section><RouterLink :to="`/day/${today}`">Посмотреть весь день</RouterLink></template></div></template>
<style scoped>.pending-banner{display:flex;justify-content:space-between;text-decoration:none;color:var(--text)}</style>
