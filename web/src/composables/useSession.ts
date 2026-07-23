import { readonly, ref } from 'vue'
import { authApi } from '../api/auth'
import { onUnauthorized } from '../api/client'
import type { Session } from '../api/types'

const user = ref<Session | null>(null)
const ready = ref(false)
const returnPath = ref('/calendar')
export function useSession() {
  async function bootstrap() {
    try { user.value = await authApi.session() } catch { user.value = null } finally { ready.value = true }
  }
  function expire(path = location.pathname + location.search) { user.value = null; returnPath.value = path }
  function authenticated(value: Session) { user.value = value }
  onUnauthorized(() => expire())
  return { user: readonly(user), ready: readonly(ready), returnPath: readonly(returnPath), bootstrap, expire, authenticated }
}
