<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { journalApi } from '../api/journal'
import UiButton from '../components/ui/UiButton.vue'

const route = useRoute()
const router = useRouter()
const text = ref('')
const busy = ref(false)
const error = ref('')
const date = computed(() => typeof route.query.date === 'string' ? route.query.date : '')

async function submit() {
  const value = text.value.trim()
  if (!value) {
    error.value = 'Напишите, что произошло.'
    return
  }
  busy.value = true
  error.value = ''
  try {
    await journalApi.createEntry(value, globalThis.crypto.randomUUID(), date.value || undefined)
    await router.push('/pending')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="page stack entry-create">
    <header>
      <p class="eyebrow">Новая запись</p>
      <h1>Что произошло?</h1>
      <p v-if="date" class="muted">Запись из хронологии за {{ new Date(`${date}T12:00:00`).toLocaleDateString('ru-RU', { dateStyle: 'long' }) }}</p>
    </header>
    <form class="card stack" @submit.prevent="submit">
      <label class="field">
        <span>Запись своими словами</span>
        <textarea v-model="text" rows="7" maxlength="4000" autofocus placeholder="Например: около 15:00 заболела голова справа, выпил ибупрофен 400 мг" />
      </label>
      <p class="muted">Текст будет распознан, а факты появятся во «Входящих». До подтверждения они не участвуют в календаре и аналитике.</p>
      <p v-if="error" class="field-error" role="alert">{{ error }}</p>
      <div class="cluster">
        <UiButton :busy="busy" type="submit">Отправить на распознавание</UiButton>
        <UiButton variant="secondary" type="button" @click="$router.back()">Отмена</UiButton>
      </div>
    </form>
  </div>
</template>

<style scoped>
.entry-create { max-width: 760px; }
textarea { resize: vertical; }
</style>
