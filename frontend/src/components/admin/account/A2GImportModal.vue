<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.a2gImportTitle')"
    width="normal"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="a2g-import-form" class="space-y-4" @submit.prevent="handleImport">
      <div class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.accounts.a2gImportHint') }}
      </div>
      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {{ t('admin.accounts.a2gImportWarning') }}
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.a2gImportFile') }}</label>
        <div
          class="flex items-center justify-between gap-3 rounded-lg border border-dashed px-4 py-3 transition-colors"
          :class="dragActive
            ? 'border-primary-400 bg-primary-50/70 dark:border-primary-500 dark:bg-primary-900/20'
            : 'border-gray-300 bg-gray-50 dark:border-dark-600 dark:bg-dark-800'"
          @dragenter.prevent="dragActive = true"
          @dragover.prevent
          @dragleave.prevent="dragActive = false"
          @drop.prevent="handleDrop"
        >
          <div class="min-w-0">
            <div class="truncate text-sm text-gray-700 dark:text-dark-200" :title="fileName">
              {{ fileName || t('admin.accounts.a2gImportSelectFile') }}
            </div>
            <div class="text-xs text-gray-500 dark:text-dark-400">
              .json / .txt
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
          accept=".json,.txt,application/json,text/plain"
          @change="handleFileChange"
        />
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.a2gImportPaste') }}</label>
        <textarea
          v-model="pasteContent"
          rows="8"
          class="input font-mono text-xs"
          :placeholder="t('admin.accounts.a2gImportPastePlaceholder')"
        />
      </div>

      <div
        v-if="result"
        class="space-y-2 rounded-xl border border-gray-200 p-4 dark:border-dark-700"
      >
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.a2gImportResult') }}
        </div>
        <div class="text-sm text-gray-700 dark:text-dark-300">
          {{ t('admin.accounts.a2gImportResultSummary', result) }}
        </div>
        <div v-if="errorItems.length" class="mt-2">
          <div class="text-sm font-medium text-red-600 dark:text-red-400">
            {{ t('admin.accounts.a2gImportErrors') }}
          </div>
          <div class="mt-2 max-h-48 overflow-auto rounded-lg bg-gray-50 p-3 font-mono text-xs dark:bg-dark-800">
            <div v-for="(item, idx) in errorItems" :key="idx" class="whitespace-pre-wrap">
              #{{ item.index }} {{ item.name || item.sso_masked || '-' }} — {{ item.message }}
            </div>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="importing" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn btn-primary"
          type="submit"
          form="a2g-import-form"
          :disabled="importing"
        >
          {{ importing ? t('admin.accounts.a2gImporting') : t('admin.accounts.a2gImportButton') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { GrokA2GImportResult } from '@/api/admin/accounts'
import { getGrokSSOImportTimeout } from '@/api/admin/grok'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'imported'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const fileInput = ref<HTMLInputElement | null>(null)
const fileName = ref('')
const fileContent = ref('')
const pasteContent = ref('')
const dragActive = ref(false)
const importing = ref(false)
const result = ref<GrokA2GImportResult | null>(null)

const errorItems = computed(() =>
  (result.value?.items || []).filter((item) => item.action === 'failed')
)

watch(
  () => props.show,
  (show) => {
    if (!show) return
    fileName.value = ''
    fileContent.value = ''
    pasteContent.value = ''
    result.value = null
    importing.value = false
    dragActive.value = false
  }
)

const openFilePicker = () => fileInput.value?.click()

const readFile = async (file: File) => {
  fileName.value = file.name
  fileContent.value = await file.text()
}

const handleFileChange = async (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  await readFile(file)
  input.value = ''
}

const handleDrop = async (event: DragEvent) => {
  dragActive.value = false
  const file = event.dataTransfer?.files?.[0]
  if (!file) return
  await readFile(file)
}

const handleClose = () => {
  if (importing.value) return
  if (result.value && (result.value.created || 0) > 0) {
    emit('imported')
  }
  emit('close')
}

const handleImport = async () => {
  const content = (fileContent.value || pasteContent.value || '').trim()
  if (!content) {
    appStore.showError(t('admin.accounts.a2gImportSelectFile'))
    return
  }
  importing.value = true
  result.value = null
  try {
    // Rough token count for timeout: lines or json length heuristic
    const roughCount = Math.max(1, content.split(/\r?\n/).filter((l) => l.trim()).length)
    const res = await adminAPI.accounts.importA2G(
      { content },
      { timeout: getGrokSSOImportTimeout(roughCount) }
    )
    result.value = res
    const msgParams = {
      created: res.created,
      skipped: res.skipped,
      failed: res.failed
    }
    if (res.failed > 0) {
      appStore.showError(t('admin.accounts.a2gImportCompletedWithErrors', msgParams))
    } else {
      appStore.showSuccess(t('admin.accounts.a2gImportSuccess', msgParams))
      if (res.created > 0) emit('imported')
    }
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.a2gImportFailed'))
  } finally {
    importing.value = false
  }
}
</script>
