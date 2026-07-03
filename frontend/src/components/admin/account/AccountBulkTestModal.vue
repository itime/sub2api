<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.bulkTest.title')"
    width="normal"
    @close="handleClose"
  >
    <div class="space-y-4">
      <div
        class="rounded-xl border border-gray-200 bg-gradient-to-r from-gray-50 to-gray-100 p-3 dark:border-dark-500 dark:from-dark-700 dark:to-dark-600"
      >
        <div class="flex items-center gap-3">
          <div
            class="flex h-10 w-10 items-center justify-center rounded-lg bg-gradient-to-br from-primary-500 to-primary-600"
          >
            <Icon name="play" size="md" class="text-white" :stroke-width="2" />
          </div>
          <div>
            <div class="font-semibold text-gray-900 dark:text-gray-100">
              {{ scopeTitle }}
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.bulkTest.scopeHint') }}
            </div>
          </div>
        </div>
      </div>

      <div v-if="phase === 'form'" class="space-y-4">
        <div v-if="mode === 'statuses'" class="space-y-2">
          <div class="flex items-center justify-between gap-2">
            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.bulkTest.statusFilterLabel') }}
            </label>
            <div class="flex gap-2 text-xs">
              <button
                type="button"
                class="font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
                @click="selectAllStatuses"
              >
                {{ t('admin.accounts.bulkTest.selectAllStatuses') }}
              </button>
              <span class="text-gray-300 dark:text-dark-600">|</span>
              <button
                type="button"
                class="font-medium text-gray-600 hover:text-gray-800 dark:text-dark-300"
                @click="clearStatuses"
              >
                {{ t('admin.accounts.bulkTest.clearStatuses') }}
              </button>
            </div>
          </div>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <label
              v-for="option in statusOptions"
              :key="option.value"
              class="flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors"
              :class="
                isStatusSelected(option.value)
                  ? 'border-primary-300 bg-primary-50 text-primary-900 dark:border-primary-700 dark:bg-primary-900/20 dark:text-primary-100'
                  : 'border-gray-200 bg-white text-gray-700 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200 dark:hover:bg-dark-700'
              "
            >
              <input
                type="checkbox"
                class="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :checked="isStatusSelected(option.value)"
                @change="toggleStatus(option.value)"
              />
              <span class="min-w-0 flex-1 truncate">{{ option.label }}</span>
              <span class="tabular-nums text-xs text-gray-500 dark:text-dark-400">{{ formatStatusCount(option.count) }}</span>
            </label>
          </div>
          <p class="text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.accounts.bulkTest.statusFilterHint') }}
          </p>
        </div>

        <div class="space-y-1.5">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.accounts.selectTestModel') }}
          </label>
          <Select
            v-model="selectedModelId"
            :options="availableModels"
            value-key="id"
            label-key="display_name"
            :placeholder="t('admin.accounts.selectTestModel')"
          />
        </div>

        <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-xs text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300">
          <div>{{ t('admin.accounts.bulkTest.concurrencyHint', { workers: workerConcurrency }) }}</div>
          <div class="mt-1">{{ t('admin.accounts.bulkTest.accountCount', { count: resolvedCount }) }}</div>
          <div v-if="resolvedCount === 0" class="mt-2 text-amber-700 dark:text-amber-300">
            {{ mode === 'selected' ? t('admin.accounts.bulkActions.noSelection') : t('admin.accounts.bulkTest.noStatusSelected') }}
          </div>
          <div
            v-if="resolvedCount > BATCH_TEST_MAX"
            class="mt-2 text-amber-700 dark:text-amber-300"
          >
            {{ t('admin.accounts.bulkTest.maxAccountsWarning', { max: BATCH_TEST_MAX, count: resolvedCount }) }}
          </div>
        </div>
      </div>

      <div v-else-if="phase === 'running'" class="space-y-4 py-1">
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ collectingIds ? t('admin.accounts.bulkTest.collectingAccounts') : t('admin.accounts.bulkTest.running') }}
        </div>
        <div class="h-2.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
          <div
            class="h-full rounded-full bg-primary-500 transition-all duration-300 ease-out"
            :style="{ width: `${progressPercent}%` }"
          />
        </div>
        <div class="flex items-center justify-between text-xs text-gray-500 dark:text-dark-400">
          <span>{{ progressDetail }}</span>
          <span class="font-mono tabular-nums">{{ progressPercent }}%</span>
        </div>

        <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
          <div class="rounded-lg border border-green-200 bg-green-50 px-3 py-2 dark:border-green-800 dark:bg-green-900/20">
            <div class="text-xs text-green-700 dark:text-green-300">{{ t('admin.accounts.bulkTest.stats.success') }}</div>
            <div class="text-lg font-semibold tabular-nums text-green-800 dark:text-green-200">{{ stats.success }}</div>
          </div>
          <div class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 dark:border-red-800 dark:bg-red-900/20">
            <div class="text-xs text-red-700 dark:text-red-300">{{ t('admin.accounts.bulkTest.stats.failed') }}</div>
            <div class="text-lg font-semibold tabular-nums text-red-800 dark:text-red-200">{{ stats.failed }}</div>
          </div>
          <div class="rounded-lg border border-orange-200 bg-orange-50 px-3 py-2 dark:border-orange-800 dark:bg-orange-900/20">
            <div class="text-xs text-orange-700 dark:text-orange-300">{{ t('admin.accounts.bulkTest.stats.deactivated') }}</div>
            <div class="text-lg font-semibold tabular-nums text-orange-800 dark:text-orange-200">{{ stats.deactivated }}</div>
          </div>
          <div class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 dark:border-amber-800 dark:bg-amber-900/20">
            <div class="text-xs text-amber-700 dark:text-amber-300">{{ t('admin.accounts.bulkTest.stats.rateLimited') }}</div>
            <div class="text-lg font-semibold tabular-nums text-amber-800 dark:text-amber-200">{{ stats.rate_limited }}</div>
          </div>
          <div class="rounded-lg border border-purple-200 bg-purple-50 px-3 py-2 dark:border-purple-800 dark:bg-purple-900/20">
            <div class="text-xs text-purple-700 dark:text-purple-300">{{ t('admin.accounts.bulkTest.stats.authError') }}</div>
            <div class="text-lg font-semibold tabular-nums text-purple-800 dark:text-purple-200">{{ stats.auth_error }}</div>
          </div>
          <div class="rounded-lg border border-blue-200 bg-blue-50 px-3 py-2 dark:border-blue-800 dark:bg-blue-900/20">
            <div class="text-xs text-blue-700 dark:text-blue-300">{{ t('admin.accounts.bulkTest.stats.updated') }}</div>
            <div class="text-lg font-semibold tabular-nums text-blue-800 dark:text-blue-200">{{ stats.updated }}</div>
          </div>
        </div>
      </div>

      <div v-else class="space-y-4">
        <div
          class="rounded-xl border p-4 text-sm"
          :class="resultToneClass"
        >
          <div class="font-medium">{{ resultHeadline }}</div>
          <div class="mt-2 text-gray-700 dark:text-dark-300">{{ resultSummary }}</div>
        </div>

        <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
          <div class="rounded-lg border border-green-200 bg-green-50 px-3 py-2 dark:border-green-800 dark:bg-green-900/20">
            <div class="text-xs text-green-700 dark:text-green-300">{{ t('admin.accounts.bulkTest.stats.success') }}</div>
            <div class="text-lg font-semibold tabular-nums text-green-800 dark:text-green-200">{{ stats.success }}</div>
          </div>
          <div class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 dark:border-red-800 dark:bg-red-900/20">
            <div class="text-xs text-red-700 dark:text-red-300">{{ t('admin.accounts.bulkTest.stats.failed') }}</div>
            <div class="text-lg font-semibold tabular-nums text-red-800 dark:text-red-200">{{ stats.failed }}</div>
          </div>
          <div class="rounded-lg border border-orange-200 bg-orange-50 px-3 py-2 dark:border-orange-800 dark:bg-orange-900/20">
            <div class="text-xs text-orange-700 dark:text-orange-300">{{ t('admin.accounts.bulkTest.stats.deactivated') }}</div>
            <div class="text-lg font-semibold tabular-nums text-orange-800 dark:text-orange-200">{{ stats.deactivated }}</div>
          </div>
          <div class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 dark:border-amber-800 dark:bg-amber-900/20">
            <div class="text-xs text-amber-700 dark:text-amber-300">{{ t('admin.accounts.bulkTest.stats.rateLimited') }}</div>
            <div class="text-lg font-semibold tabular-nums text-amber-800 dark:text-amber-200">{{ stats.rate_limited }}</div>
          </div>
          <div class="rounded-lg border border-purple-200 bg-purple-50 px-3 py-2 dark:border-purple-800 dark:bg-purple-900/20">
            <div class="text-xs text-purple-700 dark:text-purple-300">{{ t('admin.accounts.bulkTest.stats.authError') }}</div>
            <div class="text-lg font-semibold tabular-nums text-purple-800 dark:text-purple-200">{{ stats.auth_error }}</div>
          </div>
          <div class="rounded-lg border border-blue-200 bg-blue-50 px-3 py-2 dark:border-blue-800 dark:bg-blue-900/20">
            <div class="text-xs text-blue-700 dark:text-blue-300">{{ t('admin.accounts.bulkTest.stats.updated') }}</div>
            <div class="text-lg font-semibold tabular-nums text-blue-800 dark:text-blue-200">{{ stats.updated }}</div>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button
          v-if="phase !== 'running'"
          class="rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
          @click="handleClose"
        >
          {{ t('common.close') }}
        </button>
        <button
          v-if="phase === 'form'"
          class="flex items-center gap-2 rounded-lg bg-primary-500 px-4 py-2 text-sm font-medium text-white transition-all hover:bg-primary-600 disabled:cursor-not-allowed disabled:bg-primary-400"
          :disabled="!canStartTest"
          @click="startTest"
        >
          <Icon name="play" size="sm" :stroke-width="2" />
          <span>{{ t('admin.accounts.startTest') }}</span>
        </button>
        <button
          v-else-if="phase === 'running'"
          class="flex items-center gap-2 rounded-lg bg-red-500 px-4 py-2 text-sm font-medium text-white transition-all hover:bg-red-600 disabled:cursor-not-allowed disabled:bg-red-400"
          :disabled="stopping"
          @click="stopTestAndRefresh"
        >
          <Icon v-if="stopping" name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
          <Icon v-else name="x" size="sm" :stroke-width="2" />
          <span>{{ stopping ? t('admin.accounts.bulkTest.stopping') : t('admin.accounts.bulkTest.stopAndRefresh') }}</span>
        </button>
        <template v-else-if="phase === 'done'">
          <button
            class="rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
            @click="refreshList"
          >
            {{ t('admin.accounts.bulkTest.refreshList') }}
          </button>
          <button
            class="flex items-center gap-2 rounded-lg bg-primary-500 px-4 py-2 text-sm font-medium text-white transition-all hover:bg-primary-600"
            @click="resetToForm"
          >
            <Icon name="refresh" size="sm" :stroke-width="2" />
            <span>{{ t('admin.accounts.retry') }}</span>
          </button>
        </template>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import axios from 'axios'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { Icon } from '@/components/icons'
import { adminAPI } from '@/api/admin'
import type { AccountStatusCounts, BatchTestStats } from '@/api/admin/accounts'
import type { AccountStatusTabValue } from '@/components/admin/account/AccountStatusTabs.vue'
import {
  buildDefaultBulkTestModels,
  defaultBulkTestModelId,
  type TestModelOption,
} from '@/utils/testModelOptions'

const CHUNK_SIZE = 80
const PARALLEL_REQUESTS = 3
const WORKER_CONCURRENCY = 60
const BATCH_TEST_MAX = 5000

const { t } = useI18n()

type Phase = 'form' | 'running' | 'done'

const statusOptionDefs: Array<{
  value: AccountStatusTabValue
  countKey: keyof AccountStatusCounts
  labelKey: string
}> = [
  { value: 'active', countKey: 'active', labelKey: 'admin.accounts.status.active' },
  { value: 'rate_limited', countKey: 'rate_limited', labelKey: 'admin.accounts.status.rateLimited' },
  { value: 'inactive', countKey: 'inactive', labelKey: 'admin.accounts.status.inactive' },
  { value: 'error', countKey: 'error', labelKey: 'admin.accounts.status.error' },
  { value: 'temp_unschedulable', countKey: 'temp_unschedulable', labelKey: 'admin.accounts.status.tempUnschedulable' },
  { value: 'unschedulable', countKey: 'unschedulable', labelKey: 'admin.accounts.status.unschedulable' },
]

const props = defineProps<{
  show: boolean
  mode: 'selected' | 'statuses'
  selectedCount: number
  statusCounts: AccountStatusCounts | null
  initialStatuses: AccountStatusTabValue[]
  resolveSelectedIds: () => Promise<number[]>
  fetchIdsByStatuses: (statuses: AccountStatusTabValue[], signal?: AbortSignal) => Promise<number[]>
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'started'): void
  (e: 'completed'): void
  (e: 'refresh'): void
}>()

const phase = ref<Phase>('form')
const selectedModelId = ref('')
const availableModels = ref<TestModelOption[]>(buildDefaultBulkTestModels())
const selectedStatuses = ref<AccountStatusTabValue[]>([])
const resolvedCount = ref(0)
const collectingIds = ref(false)
const completedCount = ref(0)
const totalCount = ref(0)
const stats = ref<BatchTestStats>(emptyStats())
const wasStopped = ref(false)
const stopping = ref(false)
let abortRequested = false
let abortController: AbortController | null = null

const workerConcurrency = WORKER_CONCURRENCY

const statusOptions = computed(() =>
  statusOptionDefs.map((option) => ({
    value: option.value,
    label: t(option.labelKey),
    count: props.statusCounts?.[option.countKey] ?? 0,
  }))
)

const scopeTitle = computed(() => {
  if (props.mode === 'selected') {
    return t('admin.accounts.bulkTest.scopeSelected', { count: props.selectedCount })
  }
  return t('admin.accounts.bulkTest.scopeStatuses')
})

const canStartTest = computed(() => {
  if (!selectedModelId.value || resolvedCount.value === 0) {
    return false
  }
  if (props.mode === 'statuses' && selectedStatuses.value.length === 0) {
    return false
  }
  if (resolvedCount.value > BATCH_TEST_MAX) {
    return false
  }
  return true
})

const formatStatusCount = (count: number) => {
  if (count >= 1000) {
    return count.toLocaleString()
  }
  return String(count)
}

const isStatusSelected = (status: AccountStatusTabValue) => selectedStatuses.value.includes(status)

const toggleStatus = (status: AccountStatusTabValue) => {
  if (isStatusSelected(status)) {
    selectedStatuses.value = selectedStatuses.value.filter((item: AccountStatusTabValue) => item !== status)
  } else {
    selectedStatuses.value = [...selectedStatuses.value, status]
  }
}

const selectAllStatuses = () => {
  selectedStatuses.value = statusOptionDefs.map((option) => option.value)
}

const clearStatuses = () => {
  selectedStatuses.value = []
}

const estimateCountForStatuses = (statuses: AccountStatusTabValue[]) => {
  if (!props.statusCounts || statuses.length === 0) return 0
  return statuses.reduce((sum, status) => {
    const def = statusOptionDefs.find((item) => item.value === status)
    if (!def) return sum
    return sum + (props.statusCounts?.[def.countKey] ?? 0)
  }, 0)
}

const refreshResolvedCount = () => {
  if (props.mode === 'selected') {
    resolvedCount.value = props.selectedCount
    return
  }

  if (selectedStatuses.value.length === 0) {
    resolvedCount.value = 0
    return
  }

  resolvedCount.value = estimateCountForStatuses(selectedStatuses.value)
}

const progressPercent = computed(() => {
  if (totalCount.value <= 0) return 0
  return Math.min(100, Math.round((completedCount.value / totalCount.value) * 100))
})

const progressDetail = computed(() =>
  t('admin.accounts.bulkTest.progressDetail', {
    done: completedCount.value,
    total: totalCount.value,
  })
)

const resultToneClass = computed(() =>
  stats.value.failed > 0
    ? 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-200'
    : 'border-green-200 bg-green-50 text-green-800 dark:border-green-800 dark:bg-green-900/20 dark:text-green-200'
)

const resultHeadline = computed(() => {
  if (wasStopped.value) {
    return t('admin.accounts.bulkTest.resultStopped')
  }
  if (stats.value.failed > 0) {
    return t('admin.accounts.bulkTest.resultPartial')
  }
  return t('admin.accounts.bulkTest.resultSuccess')
})

const resultSummary = computed(() => {
  if (wasStopped.value) {
    return t('admin.accounts.bulkTest.resultStoppedSummary', {
      done: completedCount.value,
      total: totalCount.value,
      updated: stats.value.updated,
    })
  }
  return t('admin.accounts.bulkTest.resultSummary', {
    success: stats.value.success,
    failed: stats.value.failed,
    updated: stats.value.updated,
  })
})

watch(
  () => props.show,
  (visible) => {
    if (!visible) {
      abortRequested = true
      return
    }
    abortRequested = false
    resetToForm()
    selectedModelId.value = defaultBulkTestModelId(availableModels.value)
    selectedStatuses.value = [...props.initialStatuses]
    refreshResolvedCount()
  }
)

watch(selectedStatuses, () => {
  if (!props.show || props.mode !== 'statuses' || phase.value !== 'form') return
  refreshResolvedCount()
})

watch(
  () => props.selectedCount,
  () => {
    if (!props.show || props.mode !== 'selected' || phase.value !== 'form') return
    refreshResolvedCount()
  }
)

function emptyStats(): BatchTestStats {
  return {
    success: 0,
    failed: 0,
    deactivated: 0,
    rate_limited: 0,
    auth_error: 0,
    updated: 0,
  }
}

function resetToForm() {
  phase.value = 'form'
  collectingIds.value = false
  stopping.value = false
  wasStopped.value = false
  completedCount.value = 0
  totalCount.value = 0
  stats.value = emptyStats()
  abortRequested = false
  abortController = null
}

function isAbortError(error: unknown): boolean {
  if (axios.isCancel(error)) return true
  if (error instanceof DOMException && error.name === 'AbortError') return true
  if (error && typeof error === 'object' && (error as { code?: string }).code === 'ERR_CANCELED') {
    return true
  }
  return false
}

function requestStop() {
  if (phase.value !== 'running' || stopping.value) return
  stopping.value = true
  wasStopped.value = true
  abortRequested = true
  abortController?.abort()
}

function mergeStats(result: { stats?: BatchTestStats; success: number; failed: number; updated?: number }) {
  if (result.stats) {
    stats.value.success += result.stats.success
    stats.value.failed += result.stats.failed
    stats.value.deactivated += result.stats.deactivated
    stats.value.rate_limited += result.stats.rate_limited
    stats.value.auth_error += result.stats.auth_error
    stats.value.updated += result.stats.updated
    return
  }

  stats.value.success += result.success
  stats.value.failed += result.failed
  stats.value.updated += result.updated ?? 0
}

function chunkArray<T>(items: T[], size: number): T[][] {
  const chunks: T[][] = []
  for (let i = 0; i < items.length; i += size) {
    chunks.push(items.slice(i, i + size))
  }
  return chunks
}

async function runChunkQueue(chunks: number[][]) {
  const queue = [...chunks]
  const signal = abortController?.signal

  const workers = Array.from({ length: PARALLEL_REQUESTS }, async () => {
    while (queue.length > 0) {
      if (abortRequested) return
      const chunk = queue.shift()
      if (!chunk || chunk.length === 0) continue

      try {
        const result = await adminAPI.accounts.batchTestConnection(chunk, {
          modelId: selectedModelId.value,
          concurrency: WORKER_CONCURRENCY,
          signal,
        })
        if (abortRequested) return
        mergeStats(result)
        completedCount.value += chunk.length
      } catch (error) {
        if (abortRequested || isAbortError(error)) return
        throw error
      }
    }
  })

  await Promise.all(workers)
}

function finishRun(stopped: boolean) {
  collectingIds.value = false
  stopping.value = false
  wasStopped.value = stopped
  phase.value = 'done'
  emit('completed')
}

const startTest = async () => {
  if (!selectedModelId.value || phase.value === 'running') return

  phase.value = 'running'
  collectingIds.value = props.mode === 'statuses'
  completedCount.value = 0
  stats.value = emptyStats()
  wasStopped.value = false
  stopping.value = false
  abortRequested = false
  abortController = new AbortController()
  emit('started')

  try {
    const accountIds =
      props.mode === 'selected'
        ? await props.resolveSelectedIds()
        : await props.fetchIdsByStatuses(selectedStatuses.value, abortController.signal)

    if (abortRequested) {
      finishRun(true)
      return
    }

    collectingIds.value = false
    if (accountIds.length === 0) {
      phase.value = 'form'
      return
    }
    if (accountIds.length > BATCH_TEST_MAX) {
      phase.value = 'form'
      return
    }

    totalCount.value = accountIds.length
    resolvedCount.value = accountIds.length

    const chunks = chunkArray(accountIds, CHUNK_SIZE)
    await runChunkQueue(chunks)
    finishRun(abortRequested)
  } catch (error) {
    if (abortRequested || isAbortError(error)) {
      finishRun(true)
      return
    }
    console.error('Bulk test connection failed:', error)
    finishRun(false)
  }
}

const stopTestAndRefresh = () => {
  requestStop()
}

const refreshList = () => {
  emit('refresh')
}

const handleClose = () => {
  if (phase.value === 'running') {
    stopTestAndRefresh()
    return
  }
  emit('close')
}
</script>
