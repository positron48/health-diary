import { describe, expect, it, vi } from 'vitest'
import { ApiError, request } from './client'
describe('api client', () => {
  it('всегда запрещает кеш и передаёт cookie', async () => {
    const fetcher=vi.spyOn(globalThis,'fetch').mockResolvedValue(new Response(JSON.stringify({ok:true}),{headers:{'Content-Type':'application/json'}}))
    await request('/events')
    expect(fetcher).toHaveBeenCalledWith('/events',expect.objectContaining({cache:'no-store',credentials:'same-origin'}))
  })
  it('разбирает поля error envelope', async () => {
    vi.spyOn(globalThis,'fetch').mockResolvedValue(new Response(JSON.stringify({error:{code:'validation_failed',message:'Ошибка',fields:{intensity:'0..10'}}}),{status:422}))
    await expect(request('/events')).rejects.toMatchObject({code:'validation_failed',fields:{intensity:'0..10'}} satisfies Partial<ApiError>)
  })
})
