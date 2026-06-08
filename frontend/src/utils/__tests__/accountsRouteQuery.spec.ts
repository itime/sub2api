import { describe, expect, it } from 'vitest'
import {
  buildAccountsRouteQuery,
  parseAccountsRouteQuery,
  routeQueriesEqual
} from '../accountsRouteQuery'

const sortableKeys = new Set([
  'name',
  'status',
  'rate_limit_reset_at',
  'created_at'
])

describe('accountsRouteQuery', () => {
  it('parses and rebuilds account filters from route query', () => {
    const query = {
      search: 'demo',
      platform: 'openai',
      type: 'oauth',
      status: 'rate_limited',
      privacy_mode: '__unset__',
      group: '12',
      sort_by: 'usage_7d_reset_at',
      sort_order: 'desc',
      page: '3',
      page_size: '50'
    }

    const parsed = parseAccountsRouteQuery(query, { sortableKeys, defaultPageSize: 20 })
    expect(parsed).toMatchObject({
      search: 'demo',
      platform: 'openai',
      type: 'oauth',
      status: 'rate_limited',
      privacy_mode: '__unset__',
      group: '12',
      sort: { sort_by: 'rate_limit_reset_at', sort_order: 'desc' },
      page: 3,
      page_size: 50
    })

    const built = buildAccountsRouteQuery(query, {
      search: parsed.search,
      platform: parsed.platform,
      type: parsed.type,
      status: parsed.status,
      privacy_mode: parsed.privacy_mode,
      group: parsed.group,
      sort_by: parsed.sort!.sort_by,
      sort_order: parsed.sort!.sort_order,
      page: parsed.page!,
      page_size: parsed.page_size!,
      defaultPageSize: 20,
      defaultSort: { sort_by: 'name', sort_order: 'asc' }
    })

    expect(built).toEqual({
      search: 'demo',
      platform: 'openai',
      type: 'oauth',
      status: 'rate_limited',
      privacy_mode: '__unset__',
      group: '12',
      sort_by: 'rate_limit_reset_at',
      sort_order: 'desc',
      page: '3',
      page_size: '50'
    })
    expect(routeQueriesEqual(built, built)).toBe(true)
  })

  it('omits default pagination and sort from rebuilt query', () => {
    const built = buildAccountsRouteQuery({}, {
      search: '',
      platform: '',
      type: '',
      status: '',
      privacy_mode: '',
      group: '',
      sort_by: 'name',
      sort_order: 'asc',
      page: 1,
      page_size: 20,
      defaultPageSize: 20,
      defaultSort: { sort_by: 'name', sort_order: 'asc' }
    })

    expect(built).toEqual({})
  })
})
