<script setup lang="ts">
withDefaults(defineProps<{ variant?: 'primary'|'secondary'|'ghost'|'danger'; busy?: boolean }>(), { variant: 'primary', busy: false })
</script>
<template><button class="button" :class="`button--${variant}`" :disabled="busy"><span v-if="busy" aria-hidden="true">…</span><slot /></button></template>
