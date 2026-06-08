import type { AccountStatusTabValue } from '@/components/admin/account/AccountStatusTabs.vue'

export const ACCOUNT_ROUTE_QUERY_KEYS = [
  'search',
  'platform',
  'type',
  'status',
  'privacy_mode',
  'group',
  'sort_by',
  'sort_order',
  'page',
  'page_size'
] as const

export type AccountRouteQueryKey = (typeof ACCOUNT_ROUTE_QUERY_KEYS)[number]

export type AccountRouteSortState = {
  sort_by: string
  sort_order: 'asc' | 'desc'
}

export type AccountRouteQueryState = {
  search: string
  platform: string
  type: string
  status: AccountStatusTabValue
  privacy_mode: string
  group: string
  sort: AccountRouteSortState | null
  page: number | null
  page_size: number | null
}

const VALID_PLATFORMS = new Set(['anthropic', 'openai', 'gemini', 'antigravity'])
const VALID_TYPES = new Set(['oauth', 'setup-token', 'apikey', 'bedrock'])
const VALID_STATUS = new Set<AccountStatusTabValue>([
  '',
  'active',
  'inactive',
  'error',
  'rate_limited',
  'temp_unschedulable',
  'unschedulable'
])
const VALID_PRIVACY_MODES = new Set([
  '__unset__',
  'training_off',
  'training_set_cf_blocked',
  'training_set_failed'
])

export type RouteQueryInput = Record<string, string | string[] | null | undefined>

export const readRouteQueryString = (query: RouteQueryInput, key: string): string => {
  const value = query[key]
  if (typeof value === 'string') return value
  if (Array.isArray(value) && typeof value[0] === 'string') return value[0]
  return ''
}

export const readRouteQueryNumber = (query: RouteQueryInput, key: string): number | null => {
  const raw = readRouteQueryString(query, key).trim()
  if (!raw) return null
  const parsed = Number.parseInt(raw, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

const normalizeStatus = (raw: string): AccountStatusTabValue => {
  return VALID_STATUS.has(raw as AccountStatusTabValue) ? (raw as AccountStatusTabValue) : ''
}

const normalizeGroup = (raw: string): string => {
  const trimmed = raw.trim()
  if (!trimmed) return ''
  if (trimmed === 'ungrouped') return 'ungrouped'
  return /^\d+$/.test(trimmed) ? trimmed : ''
}

const uiSortKeyFromQuery = (raw: string): string => {
  if (raw === 'usage_7d_reset_at') return 'rate_limit_reset_at'
  return raw
}

export const parseAccountsRouteQuery = (
  query: RouteQueryInput,
  options: {
    sortableKeys: ReadonlySet<string>
    defaultPageSize: number
  }
): AccountRouteQueryState => {
  const platformRaw = readRouteQueryString(query, 'platform').trim()
  const typeRaw = readRouteQueryString(query, 'type').trim()
  const privacyRaw = readRouteQueryString(query, 'privacy_mode').trim()
  const sortByRaw = uiSortKeyFromQuery(readRouteQueryString(query, 'sort_by').trim())
  const sortOrderRaw = readRouteQueryString(query, 'sort_order').trim()

  let sort: AccountRouteSortState | null = null
  if (sortByRaw && options.sortableKeys.has(sortByRaw)) {
    sort = {
      sort_by: sortByRaw,
      sort_order: sortOrderRaw === 'desc' ? 'desc' : 'asc'
    }
  }

  const page = readRouteQueryNumber(query, 'page')
  const pageSize = readRouteQueryNumber(query, 'page_size')

  return {
    search: readRouteQueryString(query, 'search'),
    platform: VALID_PLATFORMS.has(platformRaw) ? platformRaw : '',
    type: VALID_TYPES.has(typeRaw) ? typeRaw : '',
    status: normalizeStatus(readRouteQueryString(query, 'status').trim()),
    privacy_mode: VALID_PRIVACY_MODES.has(privacyRaw) ? privacyRaw : '',
    group: normalizeGroup(readRouteQueryString(query, 'group')),
    sort,
    page,
    page_size: pageSize
  }
}

export type BuildAccountsRouteQueryInput = {
  search: string
  platform: string
  type: string
  status: string
  privacy_mode: string
  group: string
  sort_by: string
  sort_order: 'asc' | 'desc'
  page: number
  page_size: number
  defaultPageSize: number
  defaultSort: AccountRouteSortState
}

export const buildAccountsRouteQuery = (
  currentQuery: RouteQueryInput,
  input: BuildAccountsRouteQueryInput
): Record<string, string> => {
  const next: Record<string, string> = {}

  for (const [key, value] of Object.entries(currentQuery)) {
    if (!ACCOUNT_ROUTE_QUERY_KEYS.includes(key as AccountRouteQueryKey) && typeof value === 'string') {
      next[key] = value
    }
  }

  const search = input.search.trim()
  if (search) next.search = search
  if (input.platform) next.platform = input.platform
  if (input.type) next.type = input.type
  if (input.status) next.status = input.status
  if (input.privacy_mode) next.privacy_mode = input.privacy_mode
  if (input.group) next.group = input.group

  const sortChanged =
    input.sort_by !== input.defaultSort.sort_by || input.sort_order !== input.defaultSort.sort_order
  if (sortChanged) {
    next.sort_by = input.sort_by
    next.sort_order = input.sort_order
  }

  if (input.page > 1) next.page = String(input.page)
  if (input.page_size !== input.defaultPageSize) next.page_size = String(input.page_size)

  return next
}

export const routeQueriesEqual = (left: RouteQueryInput, right: Record<string, string>): boolean => {
  const normalizedLeft: Record<string, string> = {}
  for (const key of ACCOUNT_ROUTE_QUERY_KEYS) {
    const value = readRouteQueryString(left, key)
    if (value) normalizedLeft[key] = value
  }

  const leftKeys = Object.keys(normalizedLeft).sort()
  const rightKeys = Object.keys(right).sort()
  if (leftKeys.length !== rightKeys.length) return false
  return leftKeys.every((key, index) => key === rightKeys[index] && normalizedLeft[key] === right[key])
}
