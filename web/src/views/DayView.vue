<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { journalApi } from '../api/journal'
import type { HealthEvent } from '../api/types'
import { useAsyncState } from '../composables/useAsyncState'
import { useToast } from '../composables/useToast'
import EventCard from '../features/events/EventCard.vue'
import StatePanel from '../components/ui/StatePanel.vue'
import UiDialog from '../components/ui/UiDialog.vue'
import UiButton from '../components/ui/UiButton.vue'
const date = String(useRoute().params.date), target = ref<HealthEvent|null>(null), busy=ref(false), toast=useToast()
const {data,loading,error,load}=useAsyncState(async()=>{try{return await journalApi.day(date)}catch{const list=await journalApi.events(`?from=${date}T00:00:00&to=${date}T23:59:59`);return{date,events:list.events}}})
async function remove(){if(!target.value||!data.value)return;const event=target.value,index=data.value.events.indexOf(event);data.value.events.splice(index,1);target.value=null;busy.value=true;try{await journalApi.remove(event);toast.show('Событие удалено','Отменить',async()=>{try{await journalApi.restore(event);data.value?.events.splice(index,0,{...event,revision:event.revision+2})}catch{toast.show('Не удалось восстановить событие')}})}catch{data.value.events.splice(index,0,event);toast.show('Не удалось удалить событие')}finally{busy.value=false}}
onMounted(()=>load())
</script>
<template><div class="page stack"><header><p class="eyebrow">Хронология</p><h1>{{ new Date(`${date}T12:00:00`).toLocaleDateString('ru-RU',{dateStyle:'long'}) }}</h1><p class="muted">{{ data?.events.length || 0 }} событий<span v-if="data?.pending_count"> · {{data.pending_count}} ждут проверки</span></p></header><StatePanel v-if="loading" kind="loading"/><StatePanel v-else-if="error" kind="error" :message="error" @retry="load()"/><StatePanel v-else-if="!data?.events.length" kind="empty" title="За этот день нет записей"/><EventCard v-for="event in data?.events" v-else :key="event.id" :event="event" actions @delete="target=$event"/><UiDialog :open="!!target" title="Удалить событие?" @close="target=null"><p>Событие исчезнет из дневника и аналитики. Исходная запись хранится отдельно.</p><div class="cluster"><UiButton variant="danger" :busy="busy" @click="remove">Удалить</UiButton><UiButton variant="secondary" @click="target=null">Отмена</UiButton></div></UiDialog></div></template>
