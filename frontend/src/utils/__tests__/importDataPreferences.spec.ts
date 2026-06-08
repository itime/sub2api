import { describe, expect, it } from 'vitest'
import {
  clearImportDataPreferences,
  loadImportDataPreferences,
  resolveImportDataFormDefaults,
  saveImportDataPreferences,
} from '../importDataPreferences'

describe('importDataPreferences', () => {
  it('falls back to the first proxy when no saved preferences exist', () => {
    const defaults = resolveImportDataFormDefaults(
      [{ id: 11, name: 'p1' } as any, { id: 22, name: 'p2' } as any],
      [{ id: 3, name: 'g1' } as any],
      null
    )

    expect(defaults.presetProxyId).toBe(11)
    expect(defaults.presetGroupIds).toEqual([])
    expect(defaults.fastImport).toBe(true)
    expect(defaults.testAfterImport).toBe(false)
    expect(defaults.remembered).toBe(false)
  })

  it('restores saved preferences and drops missing proxy/group ids', () => {
    const defaults = resolveImportDataFormDefaults(
      [{ id: 22, name: 'p2' } as any],
      [{ id: 5, name: 'g5' } as any],
      {
        presetProxyId: 99,
        presetGroupIds: [5, 6],
        fastImport: false,
        testAfterImport: true,
      }
    )

    expect(defaults.presetProxyId).toBe(22)
    expect(defaults.presetGroupIds).toEqual([5])
    expect(defaults.fastImport).toBe(false)
    expect(defaults.testAfterImport).toBe(true)
    expect(defaults.remembered).toBe(true)
  })

  it('persists and clears preferences in localStorage', () => {
    localStorage.clear()
    saveImportDataPreferences({
      presetProxyId: 7,
      presetGroupIds: [1, 2],
      fastImport: true,
      testAfterImport: false,
    })

    expect(loadImportDataPreferences()).toEqual({
      presetProxyId: 7,
      presetGroupIds: [1, 2],
      fastImport: true,
      testAfterImport: false,
    })

    clearImportDataPreferences()
    expect(loadImportDataPreferences()).toBeNull()
  })
})
