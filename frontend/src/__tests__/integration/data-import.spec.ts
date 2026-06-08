import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ImportDataModal from '@/components/admin/account/ImportDataModal.vue'
import { adminAPI } from '@/api/admin'

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      importData: vi.fn()
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('ImportDataModal', () => {
  beforeEach(() => {
    vi.mocked(adminAPI.accounts.importData).mockReset()
  })

  it('未选择文件时进入结果页并提示', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true, proxies: [], groups: [] },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          ProxySelector: true,
          GroupSelector: true
        }
      }
    })

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(adminAPI.accounts.importData).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.accounts.dataImportSelectFile')
  })

  it('无效 JSON 时进入结果页并提示解析失败', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true, proxies: [], groups: [] },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          ProxySelector: true,
          GroupSelector: true
        }
      }
    })

    const input = wrapper.find('input[type="file"]')
    const file = new File(['invalid json'], 'data.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () => Promise.resolve('invalid json')
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(adminAPI.accounts.importData).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.accounts.dataImportParseFailed')
  })

  it('支持合并多个 JSON 文件后一次导入', async () => {
    vi.mocked(adminAPI.accounts.importData).mockResolvedValue({
      proxies_received: 2,
      accounts_received: 2,
      proxy_created: 1,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 2,
      account_failed: 0,
      errors: []
    })

    const wrapper = mount(ImportDataModal, {
      props: { show: true, proxies: [], groups: [] },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          ProxySelector: true,
          GroupSelector: true
        }
      }
    })

    const input = wrapper.find('input[type="file"]')
    const fileA = new File(
      [JSON.stringify({ proxies: [{ proxy_key: 'p1' }], accounts: [{ name: 'a1' }] })],
      'a.json',
      { type: 'application/json' }
    )
    const fileB = new File(
      [JSON.stringify({ proxies: [{ proxy_key: 'p2' }], accounts: [{ name: 'a2' }] })],
      'b.json',
      { type: 'application/json' }
    )

    Object.defineProperty(fileA, 'text', {
      value: () =>
        Promise.resolve(
          JSON.stringify({
            proxies: [{ proxy_key: 'p1', protocol: 'http', host: '127.0.0.1', port: 7890, status: 'active' }],
            accounts: [
              {
                name: 'a1@example.com',
                platform: 'openai',
                type: 'oauth',
                credentials: { access_token: 'token-a' }
              }
            ]
          })
        )
    })
    Object.defineProperty(fileB, 'text', {
      value: () =>
        Promise.resolve(
          JSON.stringify({
            proxies: [{ proxy_key: 'p2', protocol: 'http', host: '127.0.0.1', port: 7891, status: 'active' }],
            accounts: [
              {
                name: 'a2@example.com',
                platform: 'openai',
                type: 'oauth',
                credentials: { access_token: 'token-b' }
              }
            ]
          })
        )
    })
    Object.defineProperty(input.element, 'files', {
      value: [fileA, fileB]
    })

    await input.trigger('change')
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(adminAPI.accounts.importData).toHaveBeenCalledWith({
      data: expect.objectContaining({
        exported_at: expect.any(String),
        proxies: expect.arrayContaining([
          expect.objectContaining({ proxy_key: 'p1' }),
          expect.objectContaining({ proxy_key: 'p2' })
        ]),
        accounts: expect.arrayContaining([
          expect.objectContaining({ name: 'a1@example.com' }),
          expect.objectContaining({ name: 'a2@example.com' })
        ])
      }),
      skip_default_group_bind: true,
      default_proxy_id: null,
      default_group_ids: []
    })
  })

  it('支持 Codex session 单文件格式并调用导入接口', async () => {
    vi.mocked(adminAPI.accounts.importData).mockResolvedValue({
      proxies_received: 0,
      accounts_received: 1,
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 1,
      account_failed: 0,
      errors: []
    })

    const wrapper = mount(ImportDataModal, {
      props: { show: true, proxies: [], groups: [] },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          ProxySelector: true,
          GroupSelector: true
        }
      }
    })

    const input = wrapper.find('input[type="file"]')
    const session = {
      email: 'user@example.com',
      access_token: 'session-access-token',
      refresh_token: 'session-refresh-token',
      type: 'codex',
      account_id: 'a4e523ff-bf2f-4212-ba8d-f1028c79e07a'
    }
    const file = new File([JSON.stringify(session)], 'user@example.com.json', {
      type: 'application/json'
    })
    Object.defineProperty(file, 'text', {
      value: () => Promise.resolve(JSON.stringify(session))
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(adminAPI.accounts.importData).toHaveBeenCalledWith(
      expect.objectContaining({
        data: expect.objectContaining({
          accounts: [
            expect.objectContaining({
              name: 'user@example.com',
              platform: 'openai',
              type: 'oauth',
              extra: expect.objectContaining({ import_source: 'codex_session' }),
              credentials: expect.objectContaining({
                access_token: 'session-access-token',
                refresh_token: 'session-refresh-token',
                email: 'user@example.com',
                chatgpt_account_id: 'a4e523ff-bf2f-4212-ba8d-f1028c79e07a'
              })
            })
          ]
        })
      })
    )
  })
})
