import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import UiDialog from './UiDialog.vue'
describe('UiDialog', () => {
  it('имеет modal semantics и закрывается по Escape', async () => {
    const wrapper=mount(UiDialog,{attachTo:document.body,props:{open:true,title:'Подтверждение'},slots:{default:'<button>Действие</button>'}})
    expect(document.querySelector('[role="dialog"]')?.getAttribute('aria-modal')).toBe('true')
    document.querySelector('[role="dialog"]')?.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}))
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })
})
