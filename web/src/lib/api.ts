// ---------------------------------------------------------------------------
// API gateway – single entry-point for all backend calls
// ---------------------------------------------------------------------------

const BASE_URL: string = import.meta.env.VITE_API_URL ?? '';

// ---- snake_case <-> camelCase transforms -----------------------------------

function toCamelCase(str: string): string {
  return str.replace(/_([a-z])/g, (_, c: string) => c.toUpperCase());
}

function toSnakeCase(str: string): string {
  return str.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`);
}

/** Recursively convert all keys of an object from snake_case to camelCase. */
export function keysToCamel<T>(obj: unknown): T {
  if (Array.isArray(obj)) {
    return obj.map((v) => keysToCamel(v)) as unknown as T;
  }
  if (obj !== null && typeof obj === 'object' && Object.getPrototypeOf(obj) === Object.prototype) {
    return Object.fromEntries(
      Object.entries(obj as Record<string, unknown>).map(([k, v]) => [
        toCamelCase(k),
        keysToCamel(v),
      ]),
    ) as T;
  }
  return obj as T;
}

/** Recursively convert all keys of an object from camelCase to snake_case. */
export function keysToSnake<T>(obj: unknown): T {
  if (Array.isArray(obj)) {
    return obj.map((v) => keysToSnake(v)) as unknown as T;
  }
  if (obj !== null && typeof obj === 'object' && Object.getPrototypeOf(obj) === Object.prototype) {
    return Object.fromEntries(
      Object.entries(obj as Record<string, unknown>).map(([k, v]) => [
        toSnakeCase(k),
        keysToSnake(v),
      ]),
    ) as T;
  }
  return obj as T;
}

// ---- Auth token ------------------------------------------------------------

// TODO(story-1.2): Move to httpOnly cookie-based auth. localStorage is a placeholder.
function getToken(): string | null {
  return localStorage.getItem('token');
}

// ---- Error type ------------------------------------------------------------

export class ApiError extends Error {
  code: string;

  constructor(code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
  }
}

// ---- Fetch wrapper ---------------------------------------------------------

interface ApiFetchOptions extends Omit<RequestInit, 'body'> {
  body?: unknown;
}

/**
 * Typed fetch wrapper.
 *
 * - Prepends BASE_URL to the path.
 * - Attaches the JWT bearer token when available.
 * - Converts outgoing body keys to snake_case.
 * - Converts incoming response keys to camelCase.
 * - Throws `ApiError` on non-2xx responses.
 */
export async function apiFetch<T>(
  path: string,
  options: ApiFetchOptions = {},
): Promise<T> {
  const { body, headers: extraHeaders, ...rest } = options;

  const headers: Record<string, string> = {
    ...(extraHeaders as Record<string, string>),
  };

  const token = getToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const init: RequestInit = {
    ...rest,
    headers,
  };

  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    init.body = JSON.stringify(keysToSnake(body));
  }

  let res: Response;
  try {
    res = await fetch(`${BASE_URL}${path}`, init);
  } catch (err) {
    if (err instanceof TypeError) {
      throw new ApiError('NETWORK_ERROR', 'Network request failed');
    }
    throw err;
  }

  if (!res.ok) {
    let code = 'UNKNOWN';
    let message = res.statusText;
    try {
      const body = (await res.json()) as { error?: { code?: string; message?: string } };
      code = body.error?.code ?? code;
      message = body.error?.message ?? message;
    } catch {
      // response body is not JSON – keep defaults
    }
    throw new ApiError(code, message);
  }

  // 204 No Content – nothing to parse
  if (res.status === 204) {
    return undefined as unknown as T;
  }

  const json: unknown = await res.json();
  return keysToCamel<T>(json);
}
