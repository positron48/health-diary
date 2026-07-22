<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { journalApi } from '../api/journal'
import { useAsyncState } from '../composables/useAsyncState'
import EventCard from '../features/events/EventCard.vue'
import StatePanel from '../components/ui/StatePanel.vue'
import UiButton from '../components/ui/UiButton.vue'
const id=String(useRoute().params.id),{data,loading,error,load}=useAsyncState(()=>journalApi.episode(id))
async function toggle(){if(!data.value)return;if(data.value.status==='open')await journalApi.closeEpisode(id,data.value.revision,new Date().toISOString(),'exact');else await journalApi.reopenEpisode(id,data.value.revision);await load(true)}
onMounted(()=>load())
</script>
<template><div class="page stack"><StatePanel v-if="loading" kind="loading"/><StatePanel v-else-if="error" kind="error" :message="error" @retry="load()"/><template v-else-if="data"><header><p class="eyebrow">Эпизод головной боли</p><h1>{{data.status==='open'?'Открыт':'Завершён'}}</h1><p>{{new Date(data.started_at).toLocaleString('ru-RU')}} — {{data.ended_at?new Date(data.ended_at).toLocaleString('ru-RU'):'Эпизод ещё открыт'}}</p></header><section class="card cluster"><span><b>{{data.max_intensity??'?' }}/10</b><br><small>максимальная запись</small></span><span><b>{{data.observation_count??data.events?.length??0}}</b><br><small>наблюдений</small></span><span><b>{{data.duration_minutes==null?'Не указано':`${Math.round(data.duration_minutes/60)} ч`}}</b><br><small>продолжительность</small></span></section><section class="stack"><h2>Хронология</h2><EventCard v-for="event in data.events" :key="event.id" :event="event"/></section><UiButton variant="secondary" @click="toggle">{{data.status==='open'?'Закрыть эпизод сейчас':'Открыть снова'}}</UiButton><p class="muted">Дневник показывает только записанные наблюдения и не устанавливает причинную связь.</p></template></div></template>
