<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.a2gImportTitle')"
    width="normal"
    :z-index="120"
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

      <!-- Browser direct bridge to G2A -->
      <div class="rounded-xl border border-sky-200 bg-sky-50/60 p-4 dark:border-sky-800 dark:bg-sky-900/20">
        <div class="mb-3 text-sm font-medium text-sky-900 dark:text-sky-200">
          {{ t('admin.accounts.a2gDirectTitle') }}
        </div>
        <div class="grid grid-cols-1 gap-3">
          <div>
            <label class="input-label" for="a2g-base-url">{{ t('admin.accounts.a2gBaseUrl') }}</label>
            <input
              id="a2g-base-url"
              v-model="g2aBaseUrl"
              type="text"
              class="input font-mono text-sm"
              :placeholder="t('admin.accounts.a2gBaseUrlPlaceholder')"
              autocomplete="off"
            />
          </div>
          <div>
            <label class="input-label" for="a2g-admin-key">{{ t('admin.accounts.a2gAdminKey') }}</label>
            <input
              id="a2g-admin-key"
              v-model="g2aAdminKey"
              type="password"
              class="input font-mono text-sm"
              :placeholder="t('admin.accounts.a2gAdminKeyPlaceholder')"
              autocomplete="off"
            />
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="fetching || importing"
              @click="handleFetchFromG2A"
            >
              {{ fetching ? t('admin.accounts.a2gFetching') : t('admin.accounts.a2gFetchButton') }}
            </button>
            <span v-if="fetchedCount > 0" class="text-xs text-sky-700 dark:text-sky-300">
              {{ t('admin.accounts.a2gFetchedCount', { count: fetchedCount }) }}
            </span>
          </div>
          <div class="text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.accounts.a2gDirectNote') }}
          </div>
          <div
            v-if="statusMessage"
            class="rounded-lg border px-3 py-2 text-xs whitespace-pre-wrap"
            :class="statusToneClass"
          >
            {{ statusMessage }}
          </div>
        </div>
      </div>

      <!-- Advanced: file / paste -->
      <details class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
        <summary class="cursor-pointer text-sm font-medium text-gray-700 dark:text-dark-200">
          {{ t('admin.accounts.a2gAdvanced') }}
        </summary>
        <div class="mt-3 space-y-3">
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
              rows="6"
              class="input font-mono text-xs"
              :placeholder="t('admin.accounts.a2gImportPastePlaceholder')"
            />
          </div>
        </div>
      </details>

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
              #{{ item.index }} {{ item.name || item.email || item.sso_masked || '-' }} — {{ item.message }}
            </div>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="importing || fetching" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn btn-primary"
          type="submit"
          form="a2g-import-form"
          :disabled="importing || fetching"
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

const STORAGE_URL_KEY = 'sub2api_a2g_g2a_base_url'

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
const fetching = ref(false)
const result = ref<GrokA2GImportResult | null>(null)
const g2aBaseUrl = ref('http://127.0.0.1:8010')
const g2aAdminKey = ref('')
const fetchedTokens = ref<string[]>([])
const statusMessage = ref('')
const statusTone = ref<'info' | 'success' | 'error' | ''>('')

const fetchedCount = computed(() => fetchedTokens.value.length)
const errorItems = computed(() =>
  (result.value?.items || []).filter((item) => item.action === 'failed')
)
const statusToneClass = computed(() => {
  switch (statusTone.value) {
    case 'success':
      return 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300'
    case 'error':
      return 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300'
    case 'info':
      return 'border-sky-200 bg-sky-50 text-sky-800 dark:border-sky-800 dark:bg-sky-900/20 dark:text-sky-300'
    default:
      return 'border-gray-200 bg-gray-50 text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200'
  }
})

const setStatus = (message: string, tone: 'info' | 'success' | 'error' | '' = 'info') => {
  statusMessage.value = message
  statusTone.value = tone
}

watch(
  () => props.show,
  (show) => {
    if (!show) return
    fileName.value = ''
    fileContent.value = ''
    pasteContent.value = ''
    result.value = null
    importing.value = false
    fetching.value = false
    dragActive.value = false
    fetchedTokens.value = []
    statusMessage.value = ''
    statusTone.value = ''
    try {
      const saved = localStorage.getItem(STORAGE_URL_KEY)
      if (saved) g2aBaseUrl.value = saved
    } catch {
      /* ignore */
    }
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
  if (importing.value || fetching.value) return
  if (result.value && (result.value.created || 0) > 0) {
    emit('imported')
  }
  emit('close')
}

const normalizeBaseUrl = (raw: string) => raw.trim().replace(/\/+$/, '')

const candidateBaseUrls = (raw: string): string[] => {
  const base = normalizeBaseUrl(raw)
  if (!base) return []
  const out: string[] = [base]
  try {
    const u = new URL(base)
    const port = u.port || (u.protocol === 'https:' ? '443' : '80')
    const alts = port === '8010' ? ['8012'] : port === '8012' ? ['8010'] : []
    for (const p of alts) {
      const alt = new URL(base)
      alt.port = p
      out.push(alt.toString().replace(/\/+$/, ''))
    }
  } catch {
    /* ignore */
  }
  return [...new Set(out)]
}

const extractTokensFromG2APayload = (data: any): string[] => {
  const out: string[] = []
  const seen = new Set<string>()
  const push = (v: unknown) => {
    if (typeof v !== 'string') return
    const token = v.trim()
    if (!token || seen.has(token)) return
    seen.add(token)
    out.push(token)
  }
  if (!data) return out
  if (Array.isArray(data.tokens)) {
    for (const item of data.tokens) {
      if (typeof item === 'string') push(item)
      else if (item && typeof item === 'object') push((item as any).token || (item as any).sso)
    }
  }
  for (const key of ['basic', 'super', 'heavy', 'console']) {
    const arr = (data as any)[key]
    if (Array.isArray(arr)) {
      for (const item of arr) {
        if (typeof item === 'string') push(item)
        else if (item && typeof item === 'object') push(item.token || item.sso)
      }
    }
  }
  if (data.data && typeof data.data === 'object') {
    for (const token of extractTokensFromG2APayload(data.data)) push(token)
  }
  return out
}

const fetchJsonWithTimeout = async (url: string, key: string, ms = 20000): Promise<any> => {
  const controller = new AbortController()
  const timer = window.setTimeout(() => controller.abort(), ms)
  try {
    const resp = await fetch(url, {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${key}`,
        Accept: 'application/json'
      },
      signal: controller.signal
    })
    if (!resp.ok) {
      const text = await resp.text().catch(() => '')
      throw new Error(
        t('admin.accounts.a2gFetchFailedDetail', {
          status: resp.status,
          detail: text.slice(0, 200) || resp.statusText
        })
      )
    }
    return await resp.json()
  } finally {
    window.clearTimeout(timer)
  }
}

const handleFetchFromG2A = async () => {
  const key = g2aAdminKey.value.trim()
  const bases = candidateBaseUrls(g2aBaseUrl.value)
  if (!bases.length || !key) {
    const msg = t('admin.accounts.a2gMissingFields')
    setStatus(msg, 'error')
    appStore.showError(msg)
    return
  }
  fetching.value = true
  result.value = null
  setStatus(t('admin.accounts.a2gFetching'), 'info')
  const errors: string[] = []
  try {
    for (const base of bases) {
      try {
        const data = await fetchJsonWithTimeout(`${base}/admin/api/tokens`, key, 20000)
        const tokens = extractTokensFromG2APayload(data)
        if (!tokens.length) {
          errors.push(`${base}: ${t('admin.accounts.a2gFetchEmpty')}`)
          continue
        }
        fetchedTokens.value = tokens
        g2aBaseUrl.value = base
        try {
          localStorage.setItem(STORAGE_URL_KEY, base)
        } catch {
          /* ignore */
        }
        const ok = t('admin.accounts.a2gFetchedCount', { count: tokens.length })
        setStatus(`${ok}\n${base}`, 'success')
        appStore.showSuccess(ok)
        return
      } catch (error: any) {
        const msg =
          error?.name === 'AbortError'
            ? t('admin.accounts.a2gFetchTimeout')
            : error?.message || t('admin.accounts.a2gFetchFailed')
        errors.push(`${base}: ${msg}`)
      }
    }
    fetchedTokens.value = []
    const joined = errors.join('\n') || t('admin.accounts.a2gFetchFailed')
    const hint = /Failed to fetch|NetworkError|CORS|Load failed|AbortError|timeout|超时/i.test(joined)
      ? `\n${t('admin.accounts.a2gFetchCorsHint')}`
      : ''
    const finalMsg = `${joined}${hint}\n${t('admin.accounts.a2gKeyHint')}`
    setStatus(finalMsg, 'error')
    appStore.showError(finalMsg.slice(0, 300))
  } finally {
    fetching.value = false
  }
}

const handleImport = async () => {
  const tokens = [...fetchedTokens.value]
  const content = (fileContent.value || pasteContent.value || '').trim()
  if (!tokens.length && !content) {
    if (g2aBaseUrl.value.trim() && g2aAdminKey.value.trim()) {
      await handleFetchFromG2A()
      if (!fetchedTokens.value.length) return
      return handleImport()
    }
    const msg = t('admin.accounts.a2gImportSelectFile')
    setStatus(msg, 'error')
    appStore.showError(msg)
    return
  }

  importing.value = true
  result.value = null
  setStatus(t('admin.accounts.a2gImporting'), 'info')
  try {
    const roughCount = Math.max(
      1,
      tokens.length || content.split(/\r?\n/).filter((l) => l.trim()).length
    )
    const payload: { tokens?: string[]; content?: string } = {}
    if (tokens.length) payload.tokens = tokens
    if (content) payload.content = content
    const res = await adminAPI.accounts.importA2G(payload, {
      timeout: getGrokSSOImportTimeout(roughCount)
    })
    result.value = res
    const msgParams = {
      created: res.created,
      skipped: res.skipped,
      failed: res.failed
    }
    if (res.failed > 0) {
      const msg = t('admin.accounts.a2gImportCompletedWithErrors', msgParams)
      setStatus(msg, 'error')
      appStore.showError(msg)
    } else {
      const msg = t('admin.accounts.a2gImportSuccess', msgParams)
      setStatus(msg, 'success')
      appStore.showSuccess(msg)
      if (res.created > 0) emit('imported')
    }
  } catch (error: any) {
    const msg = error?.message || t('admin.accounts.a2gImportFailed')
    setStatus(msg, 'error')
    appStore.showError(msg)
  } finally {
    importing.value = false
  }
}
</script>
