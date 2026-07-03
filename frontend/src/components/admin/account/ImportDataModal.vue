<template>
  <BaseDialog
    :show="show"
    :title="dialogTitle"
    width="normal"
    :close-on-click-outside="phase === 'form'"
    @close="handleClose"
  >
    <form
      v-if="phase === 'form'"
      id="import-data-form"
      class="space-y-4"
      @submit.prevent="handleImport"
    >
      <div class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.accounts.dataImportHint') }}
      </div>
      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-600 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {{ t('admin.accounts.dataImportWarning') }}
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.dataImportFile') }}</label>
        <div
          class="flex items-center justify-between gap-3 rounded-lg border border-dashed border-gray-300 bg-gray-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="min-w-0">
            <div class="truncate text-sm text-gray-700 dark:text-dark-200">
              {{ fileLabel || t('admin.accounts.dataImportSelectFile') }}
            </div>
            <div class="text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.accounts.dataImportFileHint') }}
            </div>
          </div>
          <button type="button" class="btn btn-secondary shrink-0" @click="openFilePicker">
            {{ t('common.chooseFile') }}
          </button>
        </div>
        <input
          ref="fileInput"
          type="file"
          class="hidden"
          accept="application/json,.json,application/zip,.zip,application/gzip,.gz,application/x-gzip"
          multiple
          @change="handleFileChange"
        />
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.dataImportPresetProxy') }}</label>
        <ProxySelector v-model="presetProxyId" :proxies="proxies" />
        <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
          {{ t('admin.accounts.dataImportPresetProxyHint') }}
        </p>
      </div>

      <GroupSelector v-model="presetGroupIds" :groups="groups" />
      <p class="-mt-2 text-xs text-gray-500 dark:text-dark-400">
        {{ t('admin.accounts.dataImportPresetGroupsHint') }}
      </p>

      <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-dark-200">
        <input
          v-model="fastImport"
          type="checkbox"
          class="mt-0.5 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
        />
        <span>
          <span class="font-medium">{{ t('admin.accounts.dataImportFastImport') }}</span>
          <span class="mt-0.5 block text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.accounts.dataImportFastImportHint') }}
          </span>
        </span>
      </label>

      <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-dark-200">
        <input
          v-model="testAfterImport"
          type="checkbox"
          class="mt-0.5 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
        />
        <span>
          <span class="font-medium">{{ t('admin.accounts.dataImportTestAfter') }}</span>
          <span class="mt-0.5 block text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.accounts.dataImportTestAfterHint') }}
          </span>
        </span>
      </label>

      <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-dark-200">
        <input
          v-model="rememberImportConfig"
          type="checkbox"
          class="mt-0.5 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
        />
        <span>
          <span class="font-medium">{{ t('admin.accounts.dataImportRememberConfig') }}</span>
          <span class="mt-0.5 block text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.accounts.dataImportRememberConfigHint') }}
          </span>
        </span>
      </label>
    </form>

    <div v-else-if="phase === 'importing'" class="space-y-4 py-2">
      <div class="text-sm font-medium text-gray-900 dark:text-white">
        {{ importProgressLabel }}
      </div>
      <div class="h-2.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
        <div
          class="h-full rounded-full bg-primary-500 transition-all duration-300 ease-out"
          :style="{ width: `${importProgressPercent}%` }"
        />
      </div>
      <div class="flex items-center justify-between text-xs text-gray-500 dark:text-dark-400">
        <span>{{ importProgressDetail }}</span>
        <span class="font-mono tabular-nums">{{ importProgressPercent }}%</span>
      </div>
      <div
        v-if="parseSummary"
        class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-xs text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300"
      >
        {{ parseSummary }}
      </div>
    </div>

    <div v-else class="space-y-4">
      <div
        class="rounded-xl border p-4 text-sm"
        :class="resultToneClass"
      >
        <div class="font-medium">{{ resultHeadline }}</div>
        <div class="mt-2 text-gray-700 dark:text-dark-300">
          {{ resultSummary }}
        </div>
        <div
          v-if="parseSummary"
          class="mt-2 text-xs text-gray-500 dark:text-dark-400"
        >
          {{ parseSummary }}
        </div>
      </div>

      <div v-if="groupedItems.length" class="space-y-3">
        <div
          v-for="group in groupedItems"
          :key="group.action"
          class="rounded-xl border border-gray-200 dark:border-dark-700"
        >
          <div class="border-b border-gray-200 px-3 py-2 text-sm font-medium text-gray-900 dark:border-dark-700 dark:text-white">
            {{ actionSectionTitle(group.action) }}
            <span class="ml-1 text-gray-500 dark:text-dark-400">({{ group.items.length }})</span>
          </div>
          <div class="max-h-40 overflow-auto p-3 font-mono text-xs text-gray-700 dark:text-dark-300">
            <div
              v-for="(item, idx) in group.items"
              :key="`${group.action}-${idx}`"
              class="whitespace-pre-wrap py-0.5"
            >
              {{ formatImportItemLine(item) }}
            </div>
          </div>
        </div>
      </div>

      <div
        v-else-if="result && !hasAnyOutcome"
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300"
      >
        {{ t('admin.accounts.dataImportNoOutcomeHint') }}
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <template v-if="phase === 'form'">
          <button class="btn btn-secondary" type="button" :disabled="importing" @click="handleClose">
            {{ t('common.cancel') }}
          </button>
          <button
            class="btn btn-primary"
            type="submit"
            form="import-data-form"
            :disabled="importing"
          >
            {{ importing ? t('admin.accounts.dataImporting') : t('admin.accounts.dataImportButton') }}
          </button>
        </template>
        <span v-else-if="phase === 'importing'" class="text-sm text-gray-500 dark:text-dark-400">
          {{ t('admin.accounts.dataImporting') }}
        </span>
        <button v-else class="btn btn-primary" type="button" @click="handleResultClose">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import { adminAPI } from '@/api/admin'
import { describeHttpClientError } from '@/utils/apiError'
import { partitionImportFiles } from '@/utils/importArchive'
import {
  clearImportDataPreferences,
  loadImportDataPreferences,
  resolveImportDataFormDefaults,
  saveImportDataPreferences,
} from '@/utils/importDataPreferences'
import type {
  AdminDataAccount,
  AdminDataImportAction,
  AdminDataImportItem,
  AdminDataImportResult,
  AdminDataPayload,
  AdminDataProxy,
  AdminGroup,
  Proxy as AccountProxy
} from '@/types'

interface Props {
  show: boolean
  proxies: AccountProxy[]
  groups: AdminGroup[]
}

interface Emits {
  (e: 'close'): void
  (e: 'imported', payload?: { createdAccountIds: number[]; testAfterImport: boolean }): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()

type ImportPhase = 'form' | 'importing' | 'result'
type ImportProgressStage = 'parse' | 'archive' | 'upload'

const phase = ref<ImportPhase>('form')
const importing = ref(false)
const files = ref<File[]>([])
const result = ref<AdminDataImportResult | null>(null)
const parseSummary = ref('')
const presetProxyId = ref<number | null>(null)
const presetGroupIds = ref<number[]>([])
const fastImport = ref(true)
const testAfterImport = ref(false)
const rememberImportConfig = ref(false)
const createdAccountIds = ref<number[]>([])
const importProgressStage = ref<ImportProgressStage>('parse')
const importProgressCurrent = ref(0)
const importProgressTotal = ref(0)
const importProgressPercent = ref(0)

const importProgressLabel = computed(() => {
  switch (importProgressStage.value) {
    case 'parse':
      return t('admin.accounts.dataImportProgressParsing')
    case 'archive':
      return t('admin.accounts.dataImportProgressArchive')
    default:
      return t('admin.accounts.dataImportProgressUploading')
  }
})

const importProgressDetail = computed(() => {
  switch (importProgressStage.value) {
    case 'parse':
      return t('admin.accounts.dataImportProgressParseDetail', {
        current: importProgressCurrent.value,
        total: importProgressTotal.value
      })
    case 'archive':
      return t('admin.accounts.dataImportProgressArchiveDetail', {
        current: importProgressCurrent.value,
        total: importProgressTotal.value,
        percent: importProgressPercent.value
      })
    default:
      return t('admin.accounts.dataImportProgressUploadDetail', {
        current: importProgressCurrent.value,
        total: importProgressTotal.value
      })
  }
})

const setImportProgress = (stage: ImportProgressStage, current: number, total: number, absolutePercent?: number) => {
  importProgressStage.value = stage
  importProgressCurrent.value = current
  importProgressTotal.value = Math.max(total, 1)
  if (absolutePercent != null) {
    importProgressPercent.value = Math.min(99, Math.round(absolutePercent))
    return
  }
  const parseWeight = 0.15
  const archiveWeight = 0.35
  const uploadWeight = 0.5
  if (stage === 'parse') {
    importProgressPercent.value = Math.min(99, Math.round((current / importProgressTotal.value) * parseWeight * 100))
    return
  }
  if (stage === 'archive') {
    const archiveBase = parseWeight * 100
    importProgressPercent.value = Math.min(
      99,
      Math.round(archiveBase + (current / importProgressTotal.value) * archiveWeight * 100)
    )
    return
  }
  const uploadBase = (parseWeight + archiveWeight) * 100
  importProgressPercent.value = Math.min(
    99,
    Math.round(uploadBase + (current / importProgressTotal.value) * uploadWeight * 100)
  )
}

const fileInput = ref<HTMLInputElement | null>(null)
const fileLabel = computed(() => {
  if (files.value.length === 0) return ''
  if (files.value.length === 1) return files.value[0]?.name || ''
  return t('admin.accounts.dataImportSelectedFiles', { count: files.value.length })
})

const dialogTitle = computed(() => {
  if (phase.value === 'result') return t('admin.accounts.dataImportResultTitle')
  if (phase.value === 'importing') return t('admin.accounts.dataImportProgressTitle')
  return t('admin.accounts.dataImportTitle')
})

const parseOnlySkipped = ref(false)

const hasAnyOutcome = computed(() => {
  if (!result.value) return false
  return (
    (result.value.account_created ?? 0) > 0 ||
    (result.value.account_updated ?? 0) > 0 ||
    (result.value.account_skipped ?? 0) > 0 ||
    (result.value.account_failed ?? 0) > 0 ||
    (result.value.proxy_created ?? 0) > 0 ||
    (result.value.proxy_reused ?? 0) > 0 ||
    (result.value.proxy_skipped ?? 0) > 0 ||
    (result.value.proxy_failed ?? 0) > 0
  )
})

const hasFailures = computed(() => {
  if (!result.value) return false
  return (result.value.account_failed ?? 0) > 0 || (result.value.proxy_failed ?? 0) > 0
})

const hasSuccess = computed(() => {
  if (!result.value) return false
  return (result.value.account_created ?? 0) > 0 || (result.value.account_updated ?? 0) > 0
})

const resultToneClass = computed(() => {
  if (hasFailures.value && hasSuccess.value) {
    return 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-200'
  }
  if (hasFailures.value) {
    return 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300'
  }
  if (hasSuccess.value) {
    return 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-200'
  }
  return 'border-gray-200 bg-gray-50 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300'
})

const resultHeadline = computed(() => {
  if (!result.value) return ''
  if (parseOnlySkipped.value) {
    return t('admin.accounts.dataImportResultParseFailed')
  }
  if (hasFailures.value && hasSuccess.value) {
    return t('admin.accounts.dataImportResultPartial')
  }
  if (hasFailures.value) {
    return t('admin.accounts.dataImportResultFailed')
  }
  if (hasSuccess.value) {
    return t('admin.accounts.dataImportResultSuccessTitle')
  }
  return t('admin.accounts.dataImportResultEmpty')
})

const resultSummary = computed(() => {
  if (!result.value) return ''
  return t('admin.accounts.dataImportResultSummary', {
    proxies_received: result.value.proxies_received ?? 0,
    accounts_received: result.value.accounts_received ?? 0,
    proxy_created: result.value.proxy_created,
    proxy_reused: result.value.proxy_reused,
    proxy_skipped: result.value.proxy_skipped ?? 0,
    proxy_failed: result.value.proxy_failed,
    account_created: result.value.account_created,
    account_updated: result.value.account_updated ?? 0,
    account_skipped: result.value.account_skipped ?? 0,
    account_failed: result.value.account_failed
  })
})

const groupedItems = computed(() => {
  const items = result.value?.items ?? []
  const order: AdminDataImportAction[] = ['created', 'updated', 'reused', 'skipped', 'failed']
  const groups = new Map<AdminDataImportAction, AdminDataImportItem[]>()
  for (const item of items) {
    const bucket = groups.get(item.action) ?? []
    bucket.push(item)
    groups.set(item.action, bucket)
  }
  return order
    .filter((action) => (groups.get(action)?.length ?? 0) > 0)
    .map((action) => ({ action, items: groups.get(action) ?? [] }))
})

const resetImportForm = () => {
  const defaults = resolveImportDataFormDefaults(
    props.proxies,
    props.groups,
    loadImportDataPreferences()
  )
  presetProxyId.value = defaults.presetProxyId
  presetGroupIds.value = defaults.presetGroupIds
  fastImport.value = defaults.fastImport
  testAfterImport.value = defaults.testAfterImport
  rememberImportConfig.value = defaults.remembered
}

const persistImportFormPreferences = () => {
  if (rememberImportConfig.value) {
    saveImportDataPreferences({
      presetProxyId: presetProxyId.value,
      presetGroupIds: presetGroupIds.value,
      fastImport: fastImport.value,
      testAfterImport: testAfterImport.value,
    })
    return
  }
  clearImportDataPreferences()
}

watch(
  () => props.show,
  (open) => {
    if (open) {
      phase.value = 'form'
      files.value = []
      result.value = null
      parseSummary.value = ''
      parseOnlySkipped.value = false
      resetImportForm()
      createdAccountIds.value = []
      importProgressStage.value = 'parse'
      importProgressCurrent.value = 0
      importProgressTotal.value = 0
      importProgressPercent.value = 0
      if (fileInput.value) {
        fileInput.value.value = ''
      }
    }
  }
)

const openFilePicker = () => {
  fileInput.value?.click()
}

const handleFileChange = (event: Event) => {
  const target = event.target as HTMLInputElement
  files.value = Array.from(target.files || [])
}

const handleClose = () => {
  if (importing.value) return
  emit('close')
}

const handleResultClose = () => {
  if (hasSuccess.value) {
    emit('imported', {
      createdAccountIds: createdAccountIds.value,
      testAfterImport: testAfterImport.value
    })
  }
  emit('close')
}

const readFileAsText = async (sourceFile: File): Promise<string> => {
  if (typeof sourceFile.text === 'function') {
    return sourceFile.text()
  }

  if (typeof sourceFile.arrayBuffer === 'function') {
    const buffer = await sourceFile.arrayBuffer()
    return new TextDecoder().decode(buffer)
  }

  return await new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () => reject(reader.error || new Error('Failed to read file'))
    reader.readAsText(sourceFile)
  })
}

const asRecord = (value: unknown): Record<string, unknown> | null => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

const pickString = (row: Record<string, unknown>, paths: string[]): string => {
  for (const path of paths) {
    const parts = path.split('.')
    let current: unknown = row
    for (const part of parts) {
      const node = asRecord(current)
      if (!node) {
        current = null
        break
      }
      current = node[part]
    }
    if (typeof current === 'string' && current.trim()) {
      return current.trim()
    }
  }
  return ''
}

type ImportParseChunk = {
  proxies: AdminDataProxy[]
  accounts: AdminDataAccount[]
  skipped: AdminDataImportItem[]
}

const CPA_EXTRA_FIELDS = new Set([
  'password',
  'source',
  'type',
  'disabled',
  'mailbox',
  'mail_provider',
  'health_status',
  'backup_written',
  'cpa_sync_status',
  'created_at',
  'last_cpa_sync_at',
  'last_cpa_sync_error',
  'last_probe_at',
  'last_probe_detail',
  'last_probe_result',
  'last_probe_status_code',
  'last_refresh'
])

const parseISOToUnix = (iso: string): number | null => {
  const timestamp = Date.parse(iso)
  if (!Number.isFinite(timestamp)) return null
  return Math.floor(timestamp / 1000)
}

const isCPARecord = (row: Record<string, unknown>): boolean => {
  if (Array.isArray(row.accounts)) return false
  return Boolean(pickString(row, ['access_token', 'refresh_token']))
}

const applyCPARecordShape = (row: Record<string, unknown>, account: AdminDataAccount) => {
  const extra = { ...(account.extra ?? {}) }
  for (const key of CPA_EXTRA_FIELDS) {
    const value = row[key]
    if (value != null && value !== '' && extra[key] == null) {
      extra[key] = value
    }
  }
  const expired = pickString(row, ['expired'])
  if (expired && account.expires_at == null) {
    const unix = parseISOToUnix(expired)
    if (unix) account.expires_at = unix
  }
  if (!extra.import_source) extra.import_source = 'cpa'
  account.extra = extra
  if (account.rate_multiplier == null) account.rate_multiplier = 1
  if (account.auto_pause_on_expired == null) account.auto_pause_on_expired = true
}

const buildImportCredentials = (row: Record<string, unknown>): Record<string, unknown> | null => {
  const credentials = { ...(asRecord(row.credentials) ?? {}) }

  const accessToken = pickString(row, [
    'access_token',
    'accessToken',
    'token',
    'tokens.access_token',
    'tokens.accessToken'
  ])
  const refreshToken = pickString(row, [
    'refresh_token',
    'refreshToken',
    'rt',
    'tokens.refresh_token',
    'tokens.refreshToken'
  ])
  const idToken = pickString(row, ['id_token', 'idToken', 'tokens.id_token', 'tokens.idToken'])
  const email = pickString(row, ['email', 'user.email'])

  if (accessToken) credentials.access_token = accessToken
  if (refreshToken) credentials.refresh_token = refreshToken
  if (idToken) credentials.id_token = idToken
  if (email && !credentials.email) credentials.email = email

  return Object.keys(credentials).length > 0 ? credentials : null
}

const normalizeImportAccountType = (raw: string): AdminDataAccount['type'] => {
  const value = raw.trim().toLowerCase()
  if (!value || value === 'codex') {
    return 'oauth'
  }
  return value as AdminDataAccount['type']
}

const buildParseSkipItem = (
  kind: 'account' | 'proxy',
  source: string,
  missingFields: string[],
  name?: string
): AdminDataImportItem => ({
  kind,
  action: 'skipped',
  name: name || source,
  message:
    missingFields.length > 0
      ? t('admin.accounts.dataImportParseSkipMissing', { fields: missingFields.join('、') })
      : t('admin.accounts.dataImportParseSkipUnrecognized')
})

const resolveAccountName = (
  row: Record<string, unknown>,
  credentials: Record<string, unknown>,
  fileHint = ''
): string => {
  let name =
    pickString(row, ['name', 'user.name']) ||
    pickString(credentials, ['email']) ||
    pickString(asRecord(row.extra) ?? {}, ['email']) ||
    pickString(row, ['email', 'user.email'])
  if (!name && fileHint) {
    name = fileHint.replace(/\.json$/i, '')
  }
  return name
}

const diagnoseAccountRecord = (
  value: unknown,
  sourceLabel: string,
  fileHint = ''
): { account: AdminDataAccount | null; skip: AdminDataImportItem | null } => {
  const row = asRecord(value)
  if (!row) {
    return {
      account: null,
      skip: buildParseSkipItem('account', sourceLabel, [t('admin.accounts.dataImportFieldObject')])
    }
  }

  const missingFields: string[] = []
  const credentials = buildImportCredentials(row)
  if (!credentials) {
    missingFields.push(t('admin.accounts.dataImportFieldCredentials'))
  }

  const name = credentials ? resolveAccountName(row, credentials, fileHint) : ''
  if (credentials && !name) {
    missingFields.push(t('admin.accounts.dataImportFieldName'))
  }

  if (missingFields.length > 0) {
    const hintName =
      pickString(row, ['name', 'email', 'user.email']) ||
      pickString(asRecord(row.extra) ?? {}, ['email']) ||
      undefined
    return {
      account: null,
      skip: buildParseSkipItem('account', sourceLabel, missingFields, hintName)
    }
  }

  const rawType = pickString(row, ['type'])
  const platform = pickString(row, ['platform']) || 'openai'
  const type = normalizeImportAccountType(rawType)
  const extra = { ...(asRecord(row.extra) ?? {}) }
  const rowEmail = pickString(row, ['email', 'user.email'])
  if (rowEmail && !extra.email) {
    extra.email = rowEmail
  }
  if (rawType.toLowerCase() === 'codex' && !extra.import_source) {
    extra.import_source = 'codex_session'
  }
  const accountID = pickString(row, ['account_id', 'chatgpt_account_id'])
  if (accountID && !credentials!.chatgpt_account_id) {
    credentials!.chatgpt_account_id = accountID
  }

  const account: AdminDataAccount = {
    name,
    platform: platform as AdminDataAccount['platform'],
    type: type as AdminDataAccount['type'],
    credentials: credentials!,
    concurrency: typeof row.concurrency === 'number' ? row.concurrency : 3,
    priority: typeof row.priority === 'number' ? row.priority : 50
  }

  if (typeof row.notes === 'string') account.notes = row.notes
  if (Object.keys(extra).length > 0) account.extra = extra
  if (typeof row.proxy_key === 'string') account.proxy_key = row.proxy_key
  if (Array.isArray(row.group_ids)) {
    account.group_ids = row.group_ids.filter((id): id is number => typeof id === 'number')
  }
  if (typeof row.rate_multiplier === 'number') account.rate_multiplier = row.rate_multiplier
  if (typeof row.expires_at === 'number') account.expires_at = row.expires_at
  if (typeof row.auto_pause_on_expired === 'boolean') {
    account.auto_pause_on_expired = row.auto_pause_on_expired
  }
  if (isCPARecord(row)) {
    applyCPARecordShape(row, account)
  }

  return { account, skip: null }
}

const diagnoseProxyRecord = (
  value: unknown,
  sourceLabel: string
): { proxy: AdminDataProxy | null; skip: AdminDataImportItem | null } => {
  const row = asRecord(value)
  if (!row) {
    return {
      proxy: null,
      skip: buildParseSkipItem('proxy', sourceLabel, [t('admin.accounts.dataImportFieldObject')])
    }
  }

  const missingFields: string[] = []
  if (typeof row.protocol !== 'string' || !row.protocol.trim()) {
    missingFields.push('protocol')
  }
  if (typeof row.host !== 'string' || !row.host.trim()) {
    missingFields.push('host')
  }
  if (typeof row.port !== 'number' || row.port <= 0) {
    missingFields.push('port')
  }

  if (missingFields.length > 0) {
    const hintName = typeof row.name === 'string' ? row.name : undefined
    return {
      proxy: null,
      skip: buildParseSkipItem('proxy', sourceLabel, missingFields, hintName)
    }
  }

  return { proxy: row as unknown as AdminDataProxy, skip: null }
}

const diagnoseUnrecognizedFile = (parsed: unknown, fileHint: string): AdminDataImportItem => {
  const row = asRecord(parsed)
  if (!row) {
    return buildParseSkipItem('account', fileHint, [t('admin.accounts.dataImportFieldObject')])
  }

  const dataNode = asRecord(row.data) ?? row
  const topLevelKeys = Object.keys(dataNode)
  const fieldsPreview =
    topLevelKeys.length > 0
      ? topLevelKeys.slice(0, 8).join('、')
      : t('admin.accounts.dataImportParseEmptyObject')

  return {
    kind: 'account',
    action: 'skipped',
    name: fileHint,
    message: t('admin.accounts.dataImportParseSkipFileUnrecognized', {
      fields: t('admin.accounts.dataImportFieldCredentials'),
      nameField: t('admin.accounts.dataImportFieldName'),
      keys: fieldsPreview
    })
  }
}

const normalizeImportPayload = (parsed: unknown, fileHint = ''): ImportParseChunk => {
  const chunk: ImportParseChunk = {
    proxies: [],
    accounts: [],
    skipped: []
  }

  if (Array.isArray(parsed)) {
    parsed.forEach((item, idx) => {
      const sourceLabel = `${fileHint}#${idx + 1}`
      const { account, skip } = diagnoseAccountRecord(item, sourceLabel, fileHint)
      if (account) {
        chunk.accounts.push(account)
      } else if (skip) {
        chunk.skipped.push(skip)
      }
    })
    return chunk
  }

  const row = asRecord(parsed)
  if (!row) {
    chunk.skipped.push(buildParseSkipItem('account', fileHint, [t('admin.accounts.dataImportFieldObject')]))
    return chunk
  }

  const dataNode = asRecord(row.data) ?? row

  if (Array.isArray(dataNode.proxies)) {
    dataNode.proxies.forEach((item, idx) => {
      const sourceLabel = `${fileHint}:proxy#${idx + 1}`
      const { proxy, skip } = diagnoseProxyRecord(item, sourceLabel)
      if (proxy) {
        chunk.proxies.push(proxy)
      } else if (skip) {
        chunk.skipped.push(skip)
      }
    })
  }

  if (Array.isArray(dataNode.accounts)) {
    dataNode.accounts.forEach((item, idx) => {
      const sourceLabel = `${fileHint}:account#${idx + 1}`
      const { account, skip } = diagnoseAccountRecord(item, sourceLabel, fileHint)
      if (account) {
        chunk.accounts.push(account)
      } else if (skip) {
        chunk.skipped.push(skip)
      }
    })
    return chunk
  }

  const single = diagnoseAccountRecord(dataNode, fileHint, fileHint)
  if (single.account) {
    chunk.accounts.push(single.account)
  } else {
    chunk.skipped.push(diagnoseUnrecognizedFile(parsed, fileHint))
  }

  return chunk
}

const buildParseOnlyResult = (skippedItems: AdminDataImportItem[]): AdminDataImportResult => {
  return {
    proxies_received: 0,
    accounts_received: 0,
    proxy_created: 0,
    proxy_reused: 0,
    proxy_skipped: skippedItems.filter((item) => item.kind === 'proxy').length,
    proxy_failed: 0,
    account_created: 0,
    account_updated: 0,
    account_skipped: skippedItems.filter((item) => item.kind === 'account').length,
    account_failed: 0,
    items: skippedItems
  }
}

const applyImportPresets = (accounts: AdminDataAccount[]): AdminDataAccount[] => {
  return accounts.map((account) => {
    const next: AdminDataAccount = { ...account }
    if ((!next.group_ids || next.group_ids.length === 0) && presetGroupIds.value.length > 0) {
      next.group_ids = [...presetGroupIds.value]
    }
    return next
  })
}

const actionSectionTitle = (action: AdminDataImportAction) => {
  switch (action) {
    case 'created':
      return t('admin.accounts.dataImportActionCreated')
    case 'updated':
      return t('admin.accounts.dataImportActionUpdated')
    case 'reused':
      return t('admin.accounts.dataImportActionReused')
    case 'skipped':
      return t('admin.accounts.dataImportActionSkipped')
    default:
      return t('admin.accounts.dataImportActionFailed')
  }
}

const formatImportItemLine = (item: AdminDataImportItem) => {
  const kindLabel = item.kind === 'proxy' ? t('admin.accounts.dataImportKindProxy') : t('admin.accounts.dataImportKindAccount')
  const name = item.name || item.proxy_key || '-'
  const detail = item.message ? ` — ${item.message}` : ''
  return `[${kindLabel}] ${name}${detail}`
}

const IMPORT_BATCH_SIZE = 200
const IMPORT_BATCH_CONCURRENCY = 8
const IMPORT_PARSE_CONCURRENCY = 24

const resolveImportErrorMessage = (error: unknown, context?: string): string => {
  if (error instanceof SyntaxError) {
    return context
      ? `[${context}] ${t('admin.accounts.dataImportParseFailed')}`
      : t('admin.accounts.dataImportParseFailed')
  }
  return describeHttpClientError(error, {
    context,
    fallback: t('admin.accounts.dataImportFailed'),
    t
  })
}

const appendArchiveFailure = (
  aggregated: AdminDataImportResult | null,
  archiveName: string,
  message: string
): AdminDataImportResult => {
  const failureItem: AdminDataImportItem = {
    kind: 'account',
    name: archiveName,
    action: 'failed',
    message
  }
  if (!aggregated) {
    return {
      proxies_received: 0,
      accounts_received: 0,
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 0,
      account_updated: 0,
      account_failed: 1,
      items: [failureItem],
      errors: [{ kind: 'account', message }]
    }
  }
  return {
    ...aggregated,
    account_failed: aggregated.account_failed + 1,
    items: [...(aggregated.items ?? []), failureItem],
    errors: [...(aggregated.errors ?? []), { kind: 'account', message }]
  }
}

const mergeImportResults = (
  base: AdminDataImportResult,
  next: AdminDataImportResult
): AdminDataImportResult => ({
  proxies_received: (base.proxies_received ?? 0) + (next.proxies_received ?? 0),
  accounts_received: (base.accounts_received ?? 0) + (next.accounts_received ?? 0),
  proxy_created: base.proxy_created + next.proxy_created,
  proxy_reused: base.proxy_reused + next.proxy_reused,
  proxy_skipped: (base.proxy_skipped ?? 0) + (next.proxy_skipped ?? 0),
  proxy_failed: base.proxy_failed + next.proxy_failed,
  account_created: base.account_created + next.account_created,
  account_updated: (base.account_updated ?? 0) + (next.account_updated ?? 0),
  account_skipped: (base.account_skipped ?? 0) + (next.account_skipped ?? 0),
  account_failed: base.account_failed + next.account_failed,
  created_account_ids: [...(base.created_account_ids ?? []), ...(next.created_account_ids ?? [])],
  items: [...(base.items ?? []), ...(next.items ?? [])],
  errors: [...(base.errors ?? []), ...(next.errors ?? [])]
})

const importPayloadInBatches = async (
  payload: AdminDataPayload,
  options: {
    skip_default_group_bind?: boolean
    fast_import?: boolean
    default_proxy_id?: number | null
    default_group_ids?: number[]
  }
): Promise<AdminDataImportResult> => {
  const accounts = payload.accounts
  const proxies = payload.proxies
  if (accounts.length <= IMPORT_BATCH_SIZE && proxies.length === 0) {
    return adminAPI.accounts.importData({
      data: payload,
      ...options
    })
  }

  const batchCount = Math.max(1, Math.ceil(accounts.length / IMPORT_BATCH_SIZE))
  const chunks: AdminDataPayload[] = []
  for (let batchIndex = 0; batchIndex < batchCount; batchIndex += 1) {
    const offset = batchIndex * IMPORT_BATCH_SIZE
    chunks.push({
      ...payload,
      proxies: batchIndex === 0 ? proxies : [],
      accounts: accounts.slice(offset, offset + IMPORT_BATCH_SIZE)
    })
  }

  let completedBatches = 0
  let aggregated: AdminDataImportResult | null = null
  let nextChunkIndex = 0
  setImportProgress('upload', 0, batchCount)

  const worker = async () => {
    while (nextChunkIndex < chunks.length) {
      const batchIndex = nextChunkIndex
      nextChunkIndex += 1
      const chunk = chunks[batchIndex]

      const res = await adminAPI.accounts.importData({
        data: chunk,
        ...options
      })
      aggregated = aggregated ? mergeImportResults(aggregated, res) : res
      completedBatches += 1
      setImportProgress('upload', completedBatches, batchCount)
      parseSummary.value = [
        parseSummary.value.split('；').filter(Boolean).slice(0, 1).join('；'),
        t('admin.accounts.dataImportBatchDone', {
          done: completedBatches,
          total: batchCount
        }),
        t('admin.accounts.dataImportBatchCreated', {
          created: aggregated.account_created,
          failed: aggregated.account_failed
        })
      ]
        .filter(Boolean)
        .join('；')
    }
  }

  const workers = Array.from(
    { length: Math.min(IMPORT_BATCH_CONCURRENCY, chunks.length) },
    () => worker()
  )
  await Promise.all(workers)

  return aggregated ?? {
    proxy_created: 0,
    proxy_reused: 0,
    proxy_failed: 0,
    account_created: 0,
    account_failed: 0
  }
}

const importArchivesSequentially = async (archiveFiles: File[]) => {
  if (archiveFiles.length === 0) {
    return null
  }

  let aggregated: AdminDataImportResult | null = null
  setImportProgress('archive', 0, archiveFiles.length)

  for (let index = 0; index < archiveFiles.length; index += 1) {
    const archive = archiveFiles[index]
    setImportProgress('archive', index, archiveFiles.length)

    try {
      const res = await adminAPI.accounts.importArchive(archive, {
        fast_import: fastImport.value,
        skip_default_group_bind: true,
        default_proxy_id: presetProxyId.value,
        default_group_ids: presetGroupIds.value,
        onUploadProgress: (percent) => {
          const overall = ((index + percent / 100) / archiveFiles.length) * 100
          setImportProgress('archive', index + 1, archiveFiles.length, 15 + overall * 0.35)
        }
      })
      aggregated = aggregated ? mergeImportResults(aggregated, res) : res
      parseSummary.value = [
        parseSummary.value.split('；').filter(Boolean).slice(0, 1).join('；'),
        t('admin.accounts.dataImportArchiveDone', {
          name: archive.name,
          created: res.account_created,
          skipped: res.account_skipped ?? 0,
          failed: res.account_failed
        })
      ]
        .filter(Boolean)
        .join('；')
    } catch (error: unknown) {
      const message = resolveImportErrorMessage(error, archive.name)
      aggregated = appendArchiveFailure(aggregated, archive.name, message)
      parseSummary.value = [
        parseSummary.value.split('；').filter(Boolean).slice(0, 1).join('；'),
        message
      ]
        .filter(Boolean)
        .join('；')
    }

    setImportProgress('archive', index + 1, archiveFiles.length)
  }

  return aggregated
}

const parseFilesInParallel = async (sourceFiles: File[]) => {
  const mergedPayload: AdminDataPayload = {
    exported_at: new Date().toISOString(),
    proxies: [],
    accounts: []
  }
  const parseSkippedItems: AdminDataImportItem[] = []
  let parsedFiles = 0
  let nextFileIndex = 0
  setImportProgress('parse', 0, sourceFiles.length)

  const worker = async () => {
    while (nextFileIndex < sourceFiles.length) {
      const fileIndex = nextFileIndex
      nextFileIndex += 1
      const sourceFile = sourceFiles[fileIndex]
      const text = await readFileAsText(sourceFile)
      const parsed = JSON.parse(text) as unknown
      const chunk = normalizeImportPayload(parsed, sourceFile.name)
      mergedPayload.proxies.push(...chunk.proxies)
      mergedPayload.accounts.push(...chunk.accounts)
      parseSkippedItems.push(...chunk.skipped)
      parsedFiles += 1
      setImportProgress('parse', parsedFiles, sourceFiles.length)
    }
  }

  const workers = Array.from(
    { length: Math.min(IMPORT_PARSE_CONCURRENCY, sourceFiles.length) },
    () => worker()
  )
  await Promise.all(workers)

  mergedPayload.accounts = applyImportPresets(mergedPayload.accounts)
  return { mergedPayload, parseSkippedItems, fileCount: sourceFiles.length }
}

const handleImport = async () => {
  if (files.value.length === 0) {
    phase.value = 'result'
    result.value = null
    parseSummary.value = t('admin.accounts.dataImportSelectFile')
    return
  }

  persistImportFormPreferences()

  importing.value = true
  phase.value = 'importing'
  parseOnlySkipped.value = false
  parseSummary.value = ''
  try {
    const { archives, jsonFiles } = partitionImportFiles(files.value)
    let aggregatedResult: AdminDataImportResult | null = null
    let parseSkippedItems: AdminDataImportItem[] = []

    if (jsonFiles.length > 0) {
      const { mergedPayload, parseSkippedItems: skipped, fileCount } = await parseFilesInParallel(jsonFiles)
      parseSkippedItems = skipped

      const parseSummaryParts = [
        t('admin.accounts.dataImportParseSummary', {
          files: fileCount,
          proxies: mergedPayload.proxies.length,
          accounts: mergedPayload.accounts.length
        })
      ]
      if (archives.length > 0) {
        parseSummaryParts.push(
          t('admin.accounts.dataImportArchiveQueued', { count: archives.length })
        )
      }
      if (parseSkippedItems.length > 0) {
        parseSummaryParts.push(
          t('admin.accounts.dataImportParseSkippedSummary', { skipped: parseSkippedItems.length })
        )
      }
      parseSummary.value = parseSummaryParts.join('；')

      if (mergedPayload.accounts.length > 0 || mergedPayload.proxies.length > 0) {
        const res = await importPayloadInBatches(mergedPayload, {
          skip_default_group_bind: true,
          fast_import: fastImport.value,
          default_proxy_id: presetProxyId.value,
          default_group_ids: presetGroupIds.value
        })
        aggregatedResult = res
      }
    } else if (archives.length > 0) {
      parseSummary.value = t('admin.accounts.dataImportArchiveOnly', { count: archives.length })
    }

    if (archives.length > 0) {
      const archiveResult = await importArchivesSequentially(archives)
      aggregatedResult = aggregatedResult && archiveResult
        ? mergeImportResults(aggregatedResult, archiveResult)
        : archiveResult ?? aggregatedResult
    }

    if (!aggregatedResult) {
      phase.value = 'result'
      parseOnlySkipped.value = parseSkippedItems.length > 0
      result.value = parseSkippedItems.length > 0 ? buildParseOnlyResult(parseSkippedItems) : {
        proxies_received: 0,
        accounts_received: 0,
        proxy_created: 0,
        proxy_reused: 0,
        proxy_failed: 0,
        account_created: 0,
        account_updated: 0,
        account_failed: 0,
        items: []
      }
      return
    }

    importProgressPercent.value = 100
    createdAccountIds.value = aggregatedResult.created_account_ids ?? []
    result.value = aggregatedResult
    if (parseSkippedItems.length > 0) {
      result.value = {
        ...aggregatedResult,
        items: [...parseSkippedItems, ...(aggregatedResult.items ?? [])],
        account_skipped:
          (aggregatedResult.account_skipped ?? 0) + parseSkippedItems.filter((item) => item.kind === 'account').length,
        proxy_skipped:
          (aggregatedResult.proxy_skipped ?? 0) + parseSkippedItems.filter((item) => item.kind === 'proxy').length
      }
    }
    phase.value = 'result'
  } catch (error: unknown) {
    phase.value = 'result'
    const message = resolveImportErrorMessage(error)
    result.value = {
      proxies_received: 0,
      accounts_received: 0,
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 0,
      account_updated: 0,
      account_failed: 1,
      items: [
        {
          kind: 'account',
          action: 'failed',
          message
        }
      ],
      errors: [{ kind: 'account', message }]
    }
  } finally {
    importing.value = false
  }
}
</script>
