<script setup lang="ts">
import { useToast } from '../../composables/useToast'
const { items, dismiss } = useToast()
</script>
<template><div class="toast-region" aria-live="polite" aria-atomic="true"><div v-for="toast in items" :key="toast.id" class="toast">{{ toast.message }}<button v-if="toast.action" @click="toast.onAction?.(); dismiss(toast.id)">{{ toast.action }}</button></div></div></template>
