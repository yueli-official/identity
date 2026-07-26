import {
  failureFromProblemResponse,
  getApiFailure,
  type ApiFailure,
  type QueryValue,
} from '@yueli/http-runtime'

type AccountApiMethod = 'GET' | 'HEAD' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

export interface AccountApiCallOptions {
  method?: AccountApiMethod
  params?: Readonly<Record<string, QueryValue>>
  query?: Readonly<Record<string, QueryValue>>
  body?: unknown
  headers?: HeadersInit
  signal?: AbortSignal
  timeout?: number
}

const ERROR_BODY_LIMIT = 64 * 1024

function statusForFailure(failure: ApiFailure): number {
  if (failure.kind === 'remote') return failure.status
  if (failure.kind === 'timeout') return 504
  if (failure.kind === 'network') return 503
  if (failure.kind === 'aborted') return 499
  return 502
}

function throwFailure(failure: ApiFailure): never {
  throw createError({
    statusCode: statusForFailure(failure),
    statusMessage: failure.code,
    message: failure.code,
    data: { failure },
  })
}

function localFailure(error: unknown, signal?: AbortSignal): ApiFailure {
  const candidate = error as { name?: string, message?: string }
  if (signal?.aborted || candidate?.name === 'AbortError') {
    return {
      kind: 'aborted',
      code: 'foundation.request.aborted',
      reauth: 'not-attempted',
    }
  }
  if (
    candidate?.name === 'TimeoutError'
    || candidate?.name === 'FetchError' && /timed?\s*out|timeout/i.test(candidate.message ?? '')
  ) {
    return {
      kind: 'timeout',
      code: 'foundation.request.timeout',
      reauth: 'not-attempted',
    }
  }
  return {
    kind: 'network',
    code: 'foundation.request.network',
    reauth: 'not-attempted',
  }
}

function responseBody(data: unknown, status: number, method: AccountApiMethod): BodyInit | null {
  if (status === 204 || status === 205 || status === 304 || method === 'HEAD') return null
  if (typeof data === 'string') return data
  return JSON.stringify(data)
}

export function useApi() {
  async function call<T>(
    url: string,
    options: AccountApiCallOptions = {},
  ): Promise<T> {
    const cookieHeaders = import.meta.server
      ? useRequestHeaders(['cookie'])
      : {}
    const base = import.meta.server ? useRuntimeConfig().apiBase : ''
    const method = options.method ?? 'GET'

    try {
      const response = await $fetch.raw<T>(base + url, {
        method,
        credentials: 'include',
        retry: 0,
        ignoreResponseError: true,
        params: options.params ?? options.query,
        body: options.body as BodyInit | Record<string, unknown> | undefined,
        signal: options.signal,
        timeout: options.timeout,
        headers: {
          ...cookieHeaders,
          ...Object.fromEntries(new Headers(options.headers)),
        },
      })

      if (response.status >= 200 && response.status < 300) {
        return response._data as T
      }

      const headers = new Headers(response.headers)
      headers.delete('content-length')
      const problemResponse = new Response(
        responseBody(response._data, response.status, method),
        {
          status: response.status,
          statusText: response.statusText,
          headers,
        },
      )
      throwFailure(await failureFromProblemResponse(problemResponse, ERROR_BODY_LIMIT))
    } catch (error) {
      if (getApiFailure(error)) throw error
      throwFailure(localFailure(error, options.signal))
    }
  }

  return { call }
}
