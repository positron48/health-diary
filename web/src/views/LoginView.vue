<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { authApi, type Challenge } from '../api/auth'
import { ApiError } from '../api/client'
import { useSession } from '../composables/useSession'
import UiButton from '../components/ui/UiButton.vue'
const challenge=ref<Challenge|null>(null),code=ref(''),error=ref(''),busy=ref(false),router=useRouter(),route=useRoute(),session=useSession()
const expired=computed(()=>!!challenge.value?.expires_at&&new Date(challenge.value.expires_at)<new Date())
async function begin(){busy.value=true;error.value='';try{challenge.value=await authApi.challenge()}catch(e){error.value=(e as Error).message}finally{busy.value=false}}
async function verify(){if(!challenge.value||code.value.length!==6)return;busy.value=true;error.value='';try{await authApi.verify(challenge.value.challenge_id,code.value);session.authenticated(await authApi.session());router.replace(String(route.query.return||'/calendar'))}catch(e){const api=e as ApiError;error.value=api.code==='challenge_expired'?'Срок действия кода истёк. Запросите новый.':api.code==='challenge_locked'?'Слишком много попыток. Запросите новый код.':api.code==='invalid_code'?'Код не подошёл. Проверьте цифры.':api.message}finally{busy.value=false}}
</script>
<template><main class="login-page"><section class="card login-card"><p class="eyebrow">Личный дневник</p><h1>Вход через Telegram</h1><p>Одноразовый код не сохраняется в браузере.</p><p v-if="error" role="alert" class="field-error">{{error}}</p><UiButton v-if="!challenge||expired" :busy="busy" @click="begin">{{challenge?'Получить новый код':'Открыть бота'}}</UiButton><template v-else><a class="button telegram" :href="challenge.telegram_deep_link||challenge.telegram_url" target="_blank" rel="noreferrer">1. Открыть бота</a><label class="field">2. Код из Telegram<input v-model="code" inputmode="numeric" pattern="[0-9]*" maxlength="6" autocomplete="one-time-code" autofocus @keyup.enter="verify"/></label><p v-if="challenge.expires_at" class="muted">Код действует до {{new Date(challenge.expires_at).toLocaleTimeString('ru-RU',{hour:'2-digit',minute:'2-digit'})}}.</p><UiButton :busy="busy" :disabled="code.length!==6" @click="verify">Войти</UiButton></template><RouterLink to="/privacy">Как защищены данные</RouterLink></section></main></template>
<style scoped>.login-page{min-height:100vh;display:grid;place-items:center;padding:var(--s4)}.login-card{width:min(480px,100%);display:grid;gap:var(--s4)}.telegram{text-decoration:none;text-align:center}</style>
