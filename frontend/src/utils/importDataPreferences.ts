import type { AdminGroup, Proxy as AccountProxy } from '@/types'

const STORAGE_KEY = 'admin-account-import-preferences'

export interface ImportDataFormPreferences {
  presetProxyId: number | null
  presetGroupIds: number[]
  fastImport: boolean
  testAfterImport: boolean
}

export function loadImportDataPreferences(): ImportDataFormPreferences | null {
  if (typeof window === 'undefined') return null
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<ImportDataFormPreferences>
    if (!parsed || typeof parsed !== 'object') return null

    return {
      presetProxyId:
        parsed.presetProxyId === null || parsed.presetProxyId === undefined
          ? null
          : Number(parsed.presetProxyId),
      presetGroupIds: Array.isArray(parsed.presetGroupIds)
        ? parsed.presetGroupIds.map((id) => Number(id)).filter((id) => Number.isFinite(id))
        : [],
      fastImport: parsed.fastImport !== false,
      testAfterImport: parsed.testAfterImport === true,
    }
  } catch (error) {
    console.warn('Failed to read import data preferences:', error)
    return null
  }
}

export function saveImportDataPreferences(preferences: ImportDataFormPreferences): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(preferences))
  } catch (error) {
    console.warn('Failed to save import data preferences:', error)
  }
}

export function clearImportDataPreferences(): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.removeItem(STORAGE_KEY)
  } catch (error) {
    console.warn('Failed to clear import data preferences:', error)
  }
}

export function resolveImportDataFormDefaults(
  proxies: AccountProxy[],
  groups: AdminGroup[],
  saved: ImportDataFormPreferences | null
): ImportDataFormPreferences & { remembered: boolean } {
  const proxyIds = new Set(proxies.map((proxy) => proxy.id))
  const groupIds = new Set(groups.map((group) => group.id))
  const firstProxyId = proxies[0]?.id ?? null

  if (!saved) {
    return {
      presetProxyId: firstProxyId,
      presetGroupIds: [],
      fastImport: true,
      testAfterImport: false,
      remembered: false,
    }
  }

  const presetProxyId =
    saved.presetProxyId !== null && proxyIds.has(saved.presetProxyId)
      ? saved.presetProxyId
      : firstProxyId

  return {
    presetProxyId,
    presetGroupIds: saved.presetGroupIds.filter((id) => groupIds.has(id)),
    fastImport: saved.fastImport,
    testAfterImport: saved.testAfterImport,
    remembered: true,
  }
}
