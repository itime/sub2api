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
          <div v-if="!resolvingCount && resolvedCount === 0" class="mt-2 text-amber-700 dark:text-amber-300">
            {{ t('admin.accounts.bulkActions.noSelection') }}
          </div>
        </div>
      </div>

      <div v-else-if="phase === 'running'" class="space-y-4 py-1">
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.bulkTest.running') }}
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
          :disabled="!selectedModelId || resolvingCount || resolvedCount === 0"
          @click="startTest"
        >
          <Icon v-if="resolvingCount" name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
          <Icon v-else name="play" size="sm" :stroke-width="2" />
          <span>{{ resolvingCount ? t('common.loading') : t('admin.accounts.startTest') }}</span>
        </button>
        <span v-else-if="phase === 'running'" class="text-sm text-gray-500 dark:text-dark-400">
          {{ t('admin.accounts.testing') }}
        </span>
        <button
          v-else-if="phase === 'done'"
          class="flex items-center gap-2 rounded-lg bg-primary-500 px-4 py-2 text-sm font-medium text-white transition-all hover:bg-primary-600"
          @click="resetToForm"
        >
          <Icon name="refresh" size="sm" :stroke-width="2" />
          <span>{{ t('admin.accounts.retry') }}</span>
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { Icon } from '@/components/icons'
import { adminAPI } from '@/api/admin'
import type { BatchTestStats } from '@/api/admin/accounts'
import {
  buildDefaultBulkTestModels,
  defaultBulkTestModelId,
  type TestModelOption,
} from '@/utils/testModelOptions'

const CHUNK_SIZE = 120
const PARALLEL_REQUESTS = 4
const WORKER_CONCURRENCY = 100

const { t } = useI18n()

type Phase = 'form' | 'running' | 'done'

const props = defineProps<{
  show: boolean
  scope: 'selected' | 'filtered'
  selectedCount: number
  resolveAccountIds: () => Promise<number[]>
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'started'): void
  (e: 'completed'): void
}>()

const phase = ref<Phase>('form')
const selectedModelId = ref('')
const availableModels = ref<TestModelOption[]>(buildDefaultBulkTestModels())
const resolvedCount = ref(0)
const resolvingCount = ref(false)
const completedCount = ref(0)
const totalCount = ref(0)
const stats = ref<BatchTestStats>(emptyStats())
let abortRequested = false

const workerConcurrency = WORKER_CONCURRENCY

const scopeTitle = computed(() => {
  if (props.scope === 'selected') {
    return t('admin.accounts.bulkTest.scopeSelected', { count: props.selectedCount })
  }
  return t('admin.accounts.bulkTest.scopeFiltered')
})

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

const resultHeadline = computed(() =>
  stats.value.failed > 0
    ? t('admin.accounts.bulkTest.resultPartial')
    : t('admin.accounts.bulkTest.resultSuccess')
)

const resultSummary = computed(() =>
  t('admin.accounts.bulkTest.resultSummary', {
    success: stats.value.success,
    failed: stats.value.failed,
    updated: stats.value.updated,
  })
)

watch(
  () => props.show,
  async (visible) => {
    if (!visible) {
      abortRequested = true
      return
    }
    abortRequested = false
    resetToForm()
    selectedModelId.value = defaultBulkTestModelId(availableModels.value)
    resolvingCount.value = true
    try {
      const ids = await props.resolveAccountIds()
      resolvedCount.value = ids.length
    } catch (error) {
      console.error('Failed to resolve bulk test account ids:', error)
      resolvedCount.value = props.scope === 'selected' ? props.selectedCount : 0
    } finally {
      resolvingCount.value = false
    }
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
  completedCount.value = 0
  totalCount.value = 0
  stats.value = emptyStats()
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

  const workers = Array.from({ length: PARALLEL_REQUESTS }, async () => {
    while (queue.length > 0) {
      if (abortRequested) return
      const chunk = queue.shift()
      if (!chunk || chunk.length === 0) continue

      const result = await adminAPI.accounts.batchTestConnection(chunk, {
        modelId: selectedModelId.value,
        concurrency: WORKER_CONCURRENCY,
      })
      mergeStats(result)
      completedCount.value += chunk.length
    }
  })

  await Promise.all(workers)
}

const startTest = async () => {
  if (!selectedModelId.value || phase.value === 'running') return

  phase.value = 'running'
  completedCount.value = 0
  stats.value = emptyStats()
  abortRequested = false
  emit('started')

  try {
    const accountIds = await props.resolveAccountIds()
    if (accountIds.length === 0) {
      phase.value = 'form'
      return
    }

    totalCount.value = accountIds.length
    resolvedCount.value = accountIds.length

    const chunks = chunkArray(accountIds, CHUNK_SIZE)
    await runChunkQueue(chunks)

    phase.value = 'done'
    emit('completed')
  } catch (error) {
    console.error('Bulk test connection failed:', error)
    phase.value = 'done'
  }
}

const handleClose = () => {
  if (phase.value === 'running') return
  emit('close')
}
</script>
