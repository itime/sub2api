export interface TestModelOption {
  id: string
  display_name: string
}

const PRIORITY_TEST_MODEL_IDS = [
  'gpt-5.5',
  'gpt-5.4',
  'gpt-5.4-mini',
  'gpt-5.3-codex',
  'claude-sonnet-4-5-20250929',
  'claude-sonnet-4-5',
  'claude-3-5-sonnet-20241022',
  'gemini-2.5-flash',
  'gemini-2.5-pro',
  'gemini-2.0-flash',
]

function extractModelVersionScore(modelId: string): number {
  const matches = modelId.match(/(\d+)\.(\d+)/g)
  if (!matches || matches.length === 0) return 0
  const last = matches[matches.length - 1]
  const [major, minor] = last.split('.').map((part) => Number(part))
  if (!Number.isFinite(major) || !Number.isFinite(minor)) return 0
  return major * 100 + minor
}

export function sortTestModelOptions(models: TestModelOption[]): TestModelOption[] {
  const priorityMap = new Map(PRIORITY_TEST_MODEL_IDS.map((id, index) => [id, index]))

  return [...models].sort((a, b) => {
    const aPriority = priorityMap.get(a.id)
    const bPriority = priorityMap.get(b.id)
    if (aPriority !== undefined || bPriority !== undefined) {
      return (aPriority ?? Number.MAX_SAFE_INTEGER) - (bPriority ?? Number.MAX_SAFE_INTEGER)
    }

    const versionDiff = extractModelVersionScore(b.id) - extractModelVersionScore(a.id)
    if (versionDiff !== 0) return versionDiff
    return a.id.localeCompare(b.id)
  })
}

export function buildDefaultBulkTestModels(): TestModelOption[] {
  return sortTestModelOptions(
    PRIORITY_TEST_MODEL_IDS.map((id) => ({
      id,
      display_name: id,
    }))
  )
}

export function defaultBulkTestModelId(models: TestModelOption[]): string {
  return models[0]?.id ?? 'gpt-5.4'
}
