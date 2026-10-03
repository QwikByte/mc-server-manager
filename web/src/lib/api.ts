import { t } from "i18next"

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

/** Calls the master's REST API. The session cookie is sent automatically. */
export async function api<T = void>(path: string, init: { method?: string; body?: unknown } = {}): Promise<T> {
  const hasBody = init.body !== undefined
  const res = await fetch(`/api${path}`, {
    method: init.method ?? (hasBody ? "POST" : "GET"),
    headers: hasBody ? { "Content-Type": "application/json" } : undefined,
    body: hasBody ? JSON.stringify(init.body) : undefined,
  })
  if (res.status === 204) return undefined as T
  if (!res.ok) throw await responseError(res)
  return (await res.json().catch(() => ({}))) as T
}

/** Reads the message the master sent with a failed response. */
export async function responseError(
  res: Response,
  fallback = t("The request failed with status {{status}}.", { status: res.status }),
): Promise<ApiError> {
  const data = await res.json().catch(() => ({}))
  return new ApiError(res.status, data.error ?? fallback)
}
