<template>
  <div
    class="rounded-2xl border border-gray-200 bg-white px-3 py-2.5 shadow-sm dark:border-dark-700 dark:bg-dark-800"
    role="tablist"
    :aria-label="t('admin.accounts.statusTabs.label')"
  >
    <div class="flex gap-1 overflow-x-auto pb-0.5 [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
      <button
        v-for="tab in tabs"
        :key="tab.value"
        type="button"
        role="tab"
        :aria-selected="modelValue === tab.value"
        class="inline-flex shrink-0 items-center gap-1.5 rounded-lg px-3 py-2 text-sm font-medium transition-colors"
        :class="
          modelValue === tab.value
            ? 'bg-primary-600 text-white shadow-sm dark:bg-primary-500'
            : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'
        "
        @click="selectTab(tab.value)"
      >
        <span>{{ tab.label }}</span>
        <span
          class="tabular-nums text-xs font-semibold"
          :class="
            modelValue === tab.value
              ? 'text-primary-100 dark:text-primary-50'
              : 'text-gray-400 dark:text-gray-500'
          "
        >
          {{ formatCount(tab.count) }}
        </span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

export type AccountStatusTabValue =
  | ''
  | 'active'
  | 'inactive'
  | 'error'
  | 'rate_limited'
  | 'temp_unschedulable'
  | 'unschedulable'

export type AccountStatusCounts = Partial<Record<'all' | AccountStatusTabValue, number>>

const props = defineProps<{
  modelValue: AccountStatusTabValue
  counts: AccountStatusCounts | null
  loading?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: AccountStatusTabValue]
  change: []
}>()

const { t } = useI18n()

const tabDefs: Array<{ value: AccountStatusTabValue; countKey: keyof AccountStatusCounts; labelKey: string }> = [
  { value: '', countKey: 'all', labelKey: 'admin.accounts.statusTabs.all' },
  { value: 'active', countKey: 'active', labelKey: 'admin.accounts.status.active' },
  { value: 'inactive', countKey: 'inactive', labelKey: 'admin.accounts.status.inactive' },
  { value: 'error', countKey: 'error', labelKey: 'admin.accounts.status.error' },
  { value: 'rate_limited', countKey: 'rate_limited', labelKey: 'admin.accounts.status.rateLimited' },
  { value: 'temp_unschedulable', countKey: 'temp_unschedulable', labelKey: 'admin.accounts.status.tempUnschedulable' },
  { value: 'unschedulable', countKey: 'unschedulable', labelKey: 'admin.accounts.status.unschedulable' }
]

const tabs = computed(() =>
  tabDefs.map((tab) => ({
    value: tab.value,
    label: t(tab.labelKey),
    count: props.counts?.[tab.countKey] ?? (props.loading ? undefined : 0)
  }))
)

const formatCount = (count: number | undefined) => {
  if (count === undefined) return '…'
  return count.toLocaleString()
}

const selectTab = (value: AccountStatusTabValue) => {
  if (value === props.modelValue) return
  emit('update:modelValue', value)
  emit('change')
}
</script>
