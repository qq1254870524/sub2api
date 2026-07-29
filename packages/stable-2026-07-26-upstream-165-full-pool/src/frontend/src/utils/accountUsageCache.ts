import type { AccountUsageInfo } from '@/types'

/** Module-level usage cache shared across all AccountUsageCell instances */
const usageCache = new Map<number, { data: AccountUsageInfo; ts: number }>()

export const USAGE_CACHE_TTL = 5 * 60 * 1000 // 5 minutes

export function getCachedAccountUsage(
  accountId: number
): { data: AccountUsageInfo; ts: number } | undefined {
  return usageCache.get(accountId)
}

export function setCachedAccountUsage(accountId: number, data: AccountUsageInfo, ts = Date.now()): void {
  usageCache.set(accountId, { data, ts })
}

export function deleteCachedAccountUsage(accountId: number): void {
  usageCache.delete(accountId)
}

/**
 * Drop sticky frontend usage badges (e.g. is_forbidden) after a successful
 * connection test / reset status / recover. Call before forcing a refresh.
 */
export function invalidateAccountUsageCache(accountId?: number | number[] | null): void {
  if (accountId == null) {
    usageCache.clear()
    return
  }
  const ids = Array.isArray(accountId) ? accountId : [accountId]
  for (const id of ids) {
    usageCache.delete(id)
  }
}