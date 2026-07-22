<script setup lang="ts">
import { computed } from 'vue'
import type { HealthEvent } from '../../api/types'
import { descriptorFor, entryIdOf, eventFields, eventTime } from './eventRegistry'
const props = defineProps<{ event: HealthEvent; actions?: boolean }>()
const emit = defineEmits<{ delete: [event: HealthEvent]; source: [entryId: string] }>()
const data = computed(() => props.event.data || props.event.attributes || {})
const descriptor = computed(() => descriptorFor(props.event.kind, data.value))
const sourceId = computed(() => entryIdOf(props.event))
</script>
<template>
  <article class="event-card" :class="`tone-${descriptor.tone}`">
    <component :is="descriptor.icon" class="event-icon" :size="20" aria-hidden="true" />
    <div class="event-card__body">
      <div class="event-card__title">
        <strong>{{ descriptor.label }}</strong>
        <time :datetime="event.occurred_at">{{ eventTime(event) }}</time>
      </div>
      <dl v-if="eventFields(event).length">
        <template v-for="field in eventFields(event)" :key="field.label">
          <dt>{{ field.label }}</dt>
          <dd>{{ field.value }}</dd>
        </template>
      </dl>
      <div v-if="actions" class="inline-actions">
        <RouterLink :to="`/events/${event.id}/edit`">Изменить</RouterLink>
        <RouterLink v-if="event.episode_id" :to="`/episodes/${event.episode_id}`">Эпизод</RouterLink>
        <button v-if="sourceId" class="link" type="button" @click="emit('source', sourceId)">Исходная запись</button>
        <button class="link danger-text" type="button" @click="emit('delete', event)">Удалить</button>
      </div>
    </div>
  </article>
</template>
<style scoped>
.event-icon { color: inherit; }
.tone-pain .event-icon { color: var(--pain); }
.tone-medication .event-icon { color: var(--medication); }
.tone-sleep .event-icon { color: var(--sleep); }
.tone-activity .event-icon { color: var(--activity); }
.tone-wellbeing .event-icon { color: var(--wellbeing); }
.tone-food .event-icon,
.tone-measurement .event-icon,
.tone-note .event-icon { color: var(--muted); }
</style>
