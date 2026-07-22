import { ref } from 'vue'
import { ApiError } from '../api/client'
export function useAsyncState<T>(loader: () => Promise<T>) {
  const data = ref<T | null>(null), loading = ref(false), refreshing = ref(false), error = ref('')
  async function load(refresh = false) {
    if (refresh) refreshing.value = true; else loading.value = true
    error.value = ''
    try { data.value = await loader() } catch (e) { error.value = e instanceof ApiError ? e.message : 'Не удалось загрузить данные.' }
    finally { loading.value = refreshing.value = false }
  }
  return { data, loading, refreshing, error, load }
}
