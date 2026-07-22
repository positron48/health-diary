<script setup lang="ts">
defineProps<{ kind: 'loading'|'empty'|'error'|'offline'; title?: string; message?: string }>()
defineEmits<{ retry: [] }>()
</script>
<template>
  <div v-if="kind === 'loading'" class="skeleton-stack" role="status" aria-label="Загрузка"><span v-for="n in 3" :key="n" class="skeleton" /></div>
  <section v-else class="state" :role="kind === 'error' ? 'alert' : undefined">
    <h2>{{ title || (kind === 'empty' ? 'Пока здесь пусто' : kind === 'offline' ? 'Нет подключения' : 'Не удалось загрузить данные') }}</h2>
    <p>{{ message || (kind === 'offline' ? 'Доступна только оболочка приложения. Личные данные не сохраняются на устройстве.' : '') }}</p>
    <button v-if="kind === 'error'" class="button button--secondary" @click="$emit('retry')">Повторить</button>
    <slot />
  </section>
</template>
