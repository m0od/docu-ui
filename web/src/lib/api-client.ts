// Shared fetch wrapper for the Docu-UI API. URLs are relative so they resolve
// against <base href>, which keeps them working under a gateway sub-path.

// ApiError keeps the HTTP status and the JSON body, because some error bodies
// carry more than a message (sign-in answers {"totpRequired": true}).
export class ApiError extends Error {
  readonly status: number
  readonly responseBody: Record<string, unknown>

  constructor(status: number, responseBody: Record<string, unknown>) {
    const serverMessage = responseBody.error
    super(
      typeof serverMessage === 'string'
        ? serverMessage
        : `Request failed (HTTP ${status})`
    )
    this.status = status
    this.responseBody = responseBody
  }
}

export async function requestJson<ResponseBody>(
  url: string,
  init?: RequestInit
): Promise<ResponseBody> {
  const response = await fetch(url, init)
  // 204 No Content and non-JSON error pages have no body to parse.
  const responseBody = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new ApiError(response.status, responseBody)
  }
  return responseBody as ResponseBody
}

// postJson and sendJson send a JSON body; the server rejects writes without this
// content type, which blocks cross-site form posts (CSRF).
export function postJson<ResponseBody>(
  url: string,
  requestBody?: unknown
): Promise<ResponseBody> {
  return sendJson<ResponseBody>('POST', url, requestBody)
}

export function sendJson<ResponseBody>(
  method: string,
  url: string,
  requestBody: unknown
): Promise<ResponseBody> {
  return requestJson<ResponseBody>(url, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(requestBody ?? {}),
  })
}
