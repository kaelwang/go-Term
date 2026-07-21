import type {
  ApiResponse,
  ConnectionSpec,
  FileEntry,
  TransferStatusPayload,
  User,
  ConnectionGroup,
  SavedConnection,
  CredentialMeta,
  CredentialSecret,
  UserSettings,
} from '../types'
import { getItemWithMigration, tokenKey } from '../lib/storage'

const BASE = '/api'

function token(): string {
  return getItemWithMigration(tokenKey) || ''
}

function authHeader(): Record<string, string> {
  const t = token()
  return t ? { Authorization: `Bearer ${t}` } : {}
}

async function req<T = any>(
  path: string,
  opts: RequestInit = {},
): Promise<ApiResponse<T>> {
  const res = await fetch(BASE + path, {
    ...opts,
    headers: {
      'Content-Type': 'application/json',
      ...(opts.headers || {}),
      ...authHeader(),
    },
  })
  return (await res.json()) as ApiResponse<T>
}

export const rest = {
  // ---- Auth / identity ----

  // login authenticates and, on success, persists the JWT to localStorage so
  // every subsequent REST call and the WS handshake can present it. The data
  // shape also carries lockout fields (locked / banned / retry_after / remaining
  // / warn) returned by the server's brute-force guard on failure.
  login: (user: string, password: string) =>
    req<{
      token?: string
      user?: string
      role?: string
      locked?: boolean
      banned?: boolean
      retry_after?: number
      remaining?: number
      warn?: string
      fail_count?: number
    }>('/login', {
      method: 'POST',
      body: JSON.stringify({ user, password }),
    }).then((r) => {
      if (r.code === 0 && r.data?.token) {
        localStorage.setItem(tokenKey, r.data.token)
      }
      return r
    }),

  // publicConfig is a no-auth endpoint reporting whether login is required.
  publicConfig: () =>
    req<{ auth_enabled: boolean; version: string }>('/public/config'),

  me: () => req<{ user: string; role: string }>('/me'),

  // updateMe lets the logged-in user edit their own username and/or password.
  // When the username changes the server returns a fresh token.
  updateMe: (username: string, password: string) =>
    req<{ user: string; role: string; token?: string }>('/me', {
      method: 'PUT',
      body: JSON.stringify({ username, password }),
    }),

  // ---- User management (admin only) ----

  listUsers: () => req<User[]>('/users'),
  createUser: (username: string, password: string, role = 'user') =>
    req<{ id: number }>('/users', {
      method: 'POST',
      body: JSON.stringify({ username, password, role }),
    }),
  // updateUser edits any user's username, role and optionally password (admin).
  updateUser: (id: number, username: string, role: string, password: string) =>
    req<{ token?: string }>(`/users/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ username, role, password }),
    }),
  deleteUser: (id: number) => req(`/users/${id}`, { method: 'DELETE' }),
  resetUserPassword: (id: number, password: string) =>
    req(`/users/${id}/reset-password`, {
      method: 'POST',
      body: JSON.stringify({ password }),
    }),

  // ---- Login lockouts (admin only, keyed by client IP) ----

  // listLockouts returns every client IP currently tracked by the brute-force
  // guard, with failure count / lock / ban state for admin review.
  listLockouts: () =>
    req<
      {
        ip: string
        fail_count: number
        locked: boolean
        banned: boolean
        retry_after: number
        last_username: string
        updated_at: string
      }[]
    >('/users/lockouts'),

  // unlockLockout clears the brute-force lockout for a specific IP. The IP is
  // URL-encoded because IPv6 addresses contain colons.
  unlockLockout: (ip: string) =>
    req(`/users/lockouts/${encodeURIComponent(ip)}/unlock`, { method: 'POST' }),

  // ---- Credential vault (per-user, encrypted server-side) ----

  listCredentials: () => req<CredentialMeta[]>('/credentials'),
  createCredential: (cred: Record<string, unknown>) =>
    req<{ id: number }>('/credentials', {
      method: 'POST',
      body: JSON.stringify(cred),
    }),
  updateCredential: (id: number, cred: Record<string, unknown>) =>
    req(`/credentials/${id}`, {
      method: 'PUT',
      body: JSON.stringify(cred),
    }),
  deleteCredential: (id: number) =>
    req(`/credentials/${id}`, { method: 'DELETE' }),
  // getCredentialSecret decrypts a single credential's plaintext on demand.
  getCredentialSecret: (id: number) =>
    req<CredentialSecret>(`/credentials/${id}/secret`),

  // ---- Saved connections & groups ----

  listConnections: () => req<SavedConnection[]>('/connections'),
  createConnection: (conn: Record<string, unknown>) =>
    req<{ id: number }>('/connections', {
      method: 'POST',
      body: JSON.stringify(conn),
    }),
  updateConnection: (id: number, conn: Record<string, unknown>) =>
    req(`/connections/${id}`, {
      method: 'PUT',
      body: JSON.stringify(conn),
    }),
  deleteConnection: (id: number) =>
    req(`/connections/${id}`, { method: 'DELETE' }),

  listGroups: () => req<ConnectionGroup[]>('/connection-groups'),
  createGroup: (name: string, sortOrder = 0) =>
    req<{ id: number }>('/connection-groups', {
      method: 'POST',
      body: JSON.stringify({ name, sort_order: sortOrder }),
    }),
  updateGroup: (id: number, patch: { name?: string; sort_order?: number }) =>
    req(`/connection-groups/${id}`, {
      method: 'PUT',
      body: JSON.stringify(patch),
    }),
  deleteGroup: (id: number) =>
    req(`/connection-groups/${id}`, { method: 'DELETE' }),

  // ---- Per-user settings ----

  getSettings: () => req<UserSettings>('/settings'),
  putSettings: (settings: Partial<UserSettings>) =>
    req('/settings', { method: 'PUT', body: JSON.stringify(settings) }),

  // ---- Legacy / protocol endpoints (unchanged from prior frontend) ----

  testTerminal: (conn: ConnectionSpec) =>
    req('/test-terminal', { method: 'POST', body: JSON.stringify(conn) }),

  localShellEnabled: () => req<{ enabled: boolean }>('/local-shell-enabled'),

  list: (conn: ConnectionSpec, path: string, transfer = 'sftp') =>
    req<FileEntry[]>('/list', {
      method: 'POST',
      body: JSON.stringify({ connection: conn, path, transfer }),
    }),

  mkdir: (conn: ConnectionSpec, path: string, transfer = 'sftp') =>
    req('/mkdir', {
      method: 'POST',
      body: JSON.stringify({ connection: conn, path, transfer }),
    }),

  remove: (conn: ConnectionSpec, path: string, transfer = 'sftp') =>
    req('/remove', {
      method: 'POST',
      body: JSON.stringify({ connection: conn, path, transfer }),
    }),

  rename: (
    conn: ConnectionSpec,
    oldPath: string,
    newPath: string,
    transfer = 'sftp',
  ) =>
    req('/rename', {
      method: 'POST',
      body: JSON.stringify({ connection: conn, old: oldPath, new: newPath, transfer }),
    }),

  hostKeys: () => req<{ host: string; fingerprint: string }[]>('/hostkey'),

  addHostKey: (host: string, key: string) =>
    req('/hostkey', { method: 'POST', body: JSON.stringify({ host, key }) }),

  sessions: () => req<{ count: number }>('/sessions'),

  // ssh-config-hosts: Host aliases declared in the server's ~/.ssh/config.
  sshConfigHosts: () => req<{ hosts: string[] }>('/ssh-config-hosts'),

  // transfer-bins: availability of the external trz/tsz tools.
  transferBins: () =>
    req<{ trz: boolean; tsz: boolean }>('/transfer-bins'),

  // transferUpload: upload a local file to the server's UploadDir; the
  // returned path is the server temp path handed to the WS transfer layer.
  transferUpload: async (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    const res = await fetch(`${BASE}/transfer-upload`, {
      method: 'POST',
      body: fd,
      headers: { ...authHeader() },
    })
    return (await res.json()) as ApiResponse<{ path: string }>
  },

  transferFileUrl: (path: string) => {
    const t = token()
    const qs = t ? `&token=${encodeURIComponent(t)}` : ''
    return `${BASE}/transfer-file?path=${encodeURIComponent(path)}${qs}`
  },

  downloadUrl: (conn: ConnectionSpec, path: string, transfer = 'sftp') => {
    const t = token()
    const qs = t ? `&token=${encodeURIComponent(t)}` : ''
    return `${BASE}/file?connection=${encodeURIComponent(
      JSON.stringify(conn),
    )}&path=${encodeURIComponent(path)}&transfer=${encodeURIComponent(transfer)}${qs}`
  },

  upload: async (conn: ConnectionSpec, path: string, file: File, transfer = 'sftp') => {
    const fd = new FormData()
    fd.append('file', file)
    fd.append('meta', JSON.stringify({ connection: conn, path, transfer }))
    const res = await fetch(`${BASE}/file`, {
      method: 'POST',
      body: fd,
      headers: { ...authHeader() },
    })
    return (await res.json()) as ApiResponse
  },
}

// transferStatusOf narrows an opaque WsMessage payload into a typed
// TransferStatusPayload when it carries a transfer status update.
export function transferStatusOf(payload: unknown): TransferStatusPayload | null {
  if (!payload || typeof payload !== 'object') return null
  const p = payload as Partial<TransferStatusPayload>
  if (
    p.protocol &&
    p.direction &&
    (p.status === 'running' || p.status === 'done' || p.status === 'error')
  ) {
    return p as TransferStatusPayload
  }
  return null
}
