import { t } from "i18next"

export class ApiError extends Error {
  readonly status: number
  /** Tells what the panel may offer, e.g. to confirm the request. */
  readonly code?: string

  constructor(status: number, message: string, code?: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

export interface RequestInit {
  method?: string
  body?: unknown
}

/** Sends a request to the master's REST API, with JSON as body. The session cookie is sent automatically. */
export function request(path: string, init: RequestInit = {}): Promise<Response> {
  const hasBody = init.body !== undefined
  return fetch(`/api${path}`, {
    method: init.method ?? (hasBody ? "POST" : "GET"),
    headers: hasBody ? { "Content-Type": "application/json" } : undefined,
    body: hasBody ? JSON.stringify(init.body) : undefined,
  })
}

/** Reads the JSON of a response, or throws the error the master sent. */
export async function read<T>(res: Response): Promise<T> {
  if (res.status === 204) return undefined as T
  if (!res.ok) throw await responseError(res)
  return (await res.json().catch(() => ({}))) as T
}

/** Calls the master's REST API. */
export async function api<T = void>(path: string, init: RequestInit = {}): Promise<T> {
  return read<T>(await request(path, init))
}

/** Reads the message the master sent with a failed response. */
export async function responseError(
  res: Response,
  fallback = t("The request failed with status {{status}}.", { status: res.status }),
): Promise<ApiError> {
  const data = await res.json().catch(() => ({}))
  return new ApiError(res.status, data.error ?? fallback, data.code)
}
