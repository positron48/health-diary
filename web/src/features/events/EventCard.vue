<script setup lang="ts">
import type { HealthEvent } from '../../api/types'
import { descriptorFor, eventFields, eventTime } from './eventRegistry'
const props = defineProps<{ event: HealthEvent; actions?: boolean }>()
defineEmits<{ delete: [event: HealthEvent] }>()
const descriptor = descriptorFor(props.event.kind)
</script>
<template><article class="event-card" :class="`tone-${descriptor.tone}`"><component :is="descriptor.icon" :size="20" aria-hidden="true" /><div class="event-card__body"><div class="event-card__title"><strong>{{ descriptor.label }}</strong><time :datetime="event.occurred_at">{{ eventTime(event) }}</time></div><dl v-if="eventFields(event).length"><template v-for="field in eventFields(event)" :key="field.label"><dt>{{ field.label }}</dt><dd>{{ field.value }}</dd></template></dl><div v-if="actions" class="inline-actions"><RouterLink :to="`/events/${event.id}/edit`">Изменить</RouterLink><RouterLink v-if="event.episode_id" :to="`/episodes/${event.episode_id}`">Эпизод</RouterLink><button class="link danger-text" @click="$emit('delete', event)">Удалить</button></div></div></article></template>
