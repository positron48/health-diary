import type { ApiErrorBody } from './types'

export const API_BASE = '/api/v1'

export class ApiError extends Error {
  constructor(message: string, public status = 0, public code = 'network_error', public fields: Record<string, string> = {}) { super(message) }
  get isConflict() { return this.status === 409 || this.code === 'revision_conflict' }
}

let unauthorizedHandler: (() => void) | undefined
export const onUnauthorized = (handler: () => void) => { unauthorizedHandler = handler }

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, {
      ...init,
      credentials: 'same-origin',
      cache: 'no-store',
      headers: { Accept: 'application/json', 'Cache-Control': 'no-store', ...(init.body ? { 'Content-Type': 'application/json' } : {}), ...init.headers },
    })
  } catch {
    throw new ApiError('Не удалось связаться с сервером. Проверьте подключение и повторите попытку.')
  }
  if (response.status === 401) {
    unauthorizedHandler?.()
    throw new ApiError('Сессия завершена. Войдите снова.', 401, 'unauthorized')
  }
  if (!response.ok) {
    let body: ApiErrorBody = {}
    try { body = await response.json() as ApiErrorBody } catch { /* old endpoints can return plain text */ }
    throw new ApiError(body.error?.message || 'Операция не выполнена. Повторите попытку.', response.status, body.error?.code || `http_${response.status}`, body.error?.fields)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export const json = (method: string, body?: unknown): RequestInit => ({ method, body: body === undefined ? undefined : JSON.stringify(body) })
