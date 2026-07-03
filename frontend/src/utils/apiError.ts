/**
 * Centralized API error message extraction
 *
 * The API client interceptor rejects with a plain object: { status, code, message, error }
 * This utility extracts the user-facing message from any error shape.
 */

import axios from 'axios'

interface ApiErrorLike {
  status?: number
  code?: number | string
  message?: string
  error?: string
  reason?: string
  metadata?: Record<string, unknown>
  response?: {
    data?: {
      detail?: string
      message?: string
      code?: number | string
    }
  }
}

/**
 * Extract the error code from an API error object.
 *
 * Prefers the string `reason` (e.g. "PAYMENT_PROVIDER_MISCONFIGURED") over the
 * numeric HTTP `code`, because reason is granular enough to drive i18n lookup
 * while HTTP code is not.
 */
export function extractApiErrorCode(err: unknown): string | undefined {
  if (!err || typeof err !== 'object') return undefined
  const e = err as ApiErrorLike
  const code = e.reason ?? e.code ?? e.response?.data?.code
  return code != null ? String(code) : undefined
}

/**
 * Extract metadata (interpolation params) from an API error object.
 * Backend errors carry `metadata` with template variables that fill i18n placeholders.
 */
export function extractApiErrorMetadata(err: unknown): Record<string, unknown> | undefined {
  if (!err || typeof err !== 'object') return undefined
  const e = err as ApiErrorLike
  return e.metadata
}

type TranslateFn = (key: string, params?: Record<string, unknown>) => string
type TranslateWithExistsFn = TranslateFn & { te?: (key: string) => boolean }

/**
 * Translate a value via i18n if a matching key exists, otherwise return the original.
 * Example: "certSerial" → t('admin.settings.payment.field_certSerial') → "证书序列号".
 */
function tryTranslate(t: TranslateFn, key: string, fallback: string): string {
  const translated = t(key)
  if (translated === key) return fallback
  const te = (t as TranslateWithExistsFn).te
  if (te && !te(key)) return fallback
  return translated
}

/**
 * Replace raw config field names in metadata (e.g. "certSerial") with their
 * localized UI labels (e.g. "证书序列号"), using the provider-config field i18n namespace.
 * Handles both single `key` and `/`-joined `keys` patterns used by wxpay errors.
 */
function localizeMetadata(metadata: Record<string, unknown>, t: TranslateFn): Record<string, unknown> {
  const out: Record<string, unknown> = { ...metadata }
  if (typeof out.key === 'string') {
    out.key = tryTranslate(t, `admin.settings.payment.field_${out.key}`, out.key)
  }
  if (typeof out.keys === 'string') {
    out.keys = out.keys
      .split('/')
      .map(k => tryTranslate(t, `admin.settings.payment.field_${k}`, k))
      .join(' / ')
  }
  return out
}

/**
 * Extract a localized error message from an API error by looking up
 * `<namespace>.<REASON>` in i18n and substituting metadata as placeholders.
 *
 * Config-field names in metadata (`key` / `keys`) are automatically translated
 * to their UI labels before substitution, so error messages read like
 * "缺少必填项：证书序列号" instead of "缺少必填项：certSerial".
 *
 * @param err      - The caught error
 * @param t        - Vue i18n translate function
 * @param namespace- i18n key prefix, e.g. "payment.errors"
 * @param fallback - Fallback key or plain string if no localized mapping exists
 */
export function extractI18nErrorMessage(
  err: unknown,
  t: TranslateFn,
  namespace: string,
  fallback: string,
): string {
  const code = extractApiErrorCode(err)
  if (code) {
    const key = `${namespace}.${code}`
    const rawMetadata = extractApiErrorMetadata(err) ?? {}
    const metadata = localizeMetadata(rawMetadata, t)
    const translated = t(key, metadata)
    // Vue i18n returns the key itself when missing; detect that and fall back.
    if (translated !== key) return translated
    // If the framework exposes `te`, use it to double-check.
    const te = (t as TranslateWithExistsFn).te
    if (te && te(key)) return translated
  }
  return extractApiErrorMessage(err, fallback)
}

/**
 * Extract a displayable error message from an API error.
 *
 * @param err - The caught error (unknown type)
 * @param fallback - Fallback message if none can be extracted (use t('common.error') or similar)
 * @param i18nMap - Optional map of error codes to i18n translated strings
 */
export function extractApiErrorMessage(
  err: unknown,
  fallback = 'Unknown error',
  i18nMap?: Record<string, string>,
): string {
  if (!err) return fallback

  // Try i18n mapping by error code first
  if (i18nMap) {
    const code = extractApiErrorCode(err)
    if (code && i18nMap[code]) return i18nMap[code]
  }

  // Plain object from API client interceptor (most common case)
  if (typeof err === 'object' && err !== null) {
    const e = err as ApiErrorLike
    // Interceptor shape: { message, error }
    if (e.message) return e.message
    if (e.error) return e.error
    // Legacy axios shape: { response.data.detail }
    if (e.response?.data?.detail) return e.response.data.detail
    if (e.response?.data?.message) return e.response.data.message
  }

  // Standard Error
  if (err instanceof Error) return err.message

  // Last resort
  const str = String(err)
  return str === '[object Object]' ? fallback : str
}

type DescribeHttpClientErrorOptions = {
  fallback?: string
  /** File name or other context shown before the message. */
  context?: string
  t?: TranslateFn
}

/**
 * Human-readable message for import/upload failures (timeouts, network, HTTP errors).
 */
export function describeHttpClientError(
  err: unknown,
  options?: DescribeHttpClientErrorOptions
): string {
  const prefix = options?.context ? `[${options.context}] ` : ''
  const fallback = options?.fallback ?? 'Request failed'
  const t = options?.t

  if (axios.isAxiosError(err)) {
    if (err.code === 'ECONNABORTED' || err.code === 'ERR_CANCELED') {
      const timeoutMs = err.config?.timeout
      const minutes = timeoutMs ? Math.max(1, Math.round(timeoutMs / 60000)) : undefined
      if (t && minutes) {
        return prefix + t('admin.accounts.dataImportTimeout', { minutes })
      }
      return prefix + (minutes ? `Request timed out after ${minutes} minutes` : 'Request timed out')
    }

    if (!err.response) {
      const detail = err.message?.trim() || err.code || 'connection failed'
      if (t) {
        return prefix + t('admin.accounts.dataImportNetworkFailed', { detail })
      }
      return prefix + `Network error (${detail})`
    }

    const data = err.response.data
    const apiRecord =
      typeof data === 'object' && data !== null ? (data as Record<string, unknown>) : undefined
    const apiMsg =
      (typeof apiRecord?.message === 'string' && apiRecord.message) ||
      (typeof apiRecord?.detail === 'string' && apiRecord.detail) ||
      ''
    if (apiMsg.trim()) {
      return prefix + `${apiMsg} (HTTP ${err.response.status})`
    }
    if (t) {
      return prefix + t('admin.accounts.dataImportHttpFailed', { status: err.response.status })
    }
    return prefix + `HTTP ${err.response.status}`
  }

  return prefix + extractApiErrorMessage(err, fallback)
}
