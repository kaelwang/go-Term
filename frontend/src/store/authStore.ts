import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'
import { rest } from '../api/rest'
import { authPersistKey, getItemWithMigration, migratedStorage, tokenKey } from '../lib/storage'

// Token storage key must stay in sync with rest.authHeader() and ws.url,
// which both read the JWT from localStorage under the goterm_token key.
const TOKEN_KEY = tokenKey

interface AuthState {
  token: string
  user: string
  role: 'admin' | 'user'
  // authEnabled / version come from GET /api/public/config (C7). They gate the
  // login screen and display the build version in the UI.
  authEnabled: boolean
  version: string
  // ready becomes true once /api/public/config has been fetched (or failed),
  // so the app can decide whether to render the login gate.
  ready: boolean

  loadPublicConfig: () => Promise<void>
  loadMe: () => Promise<void>
  login: (username: string, password: string) => Promise<LoginResult>
  logout: () => void
  // setIdentity updates the current user's display name and (optionally) token
  // after a self-edit that may have re-issued the JWT.
  setIdentity: (user: string, token?: string) => void
}

// LoginResult reports the outcome of a login attempt, including the server's
// brute-force lockout details so the UI can show remaining retries / harm.
export interface LoginResult {
  ok: boolean
  message?: string
  warn?: string
  locked?: boolean
  banned?: boolean
  retryAfter?: number
  remaining?: number
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: '',
      user: '',
      role: 'user',
      authEnabled: false,
      version: '',
      ready: false,

      loadPublicConfig: async () => {
        try {
          const r = await rest.publicConfig()
          if (r.code === 0 && r.data) {
            set({ authEnabled: r.data.auth_enabled, version: r.data.version, ready: true })
            return
          }
        } catch {
          /* server unreachable; fall through to ready=true */
        }
        set({ ready: true })
      },

      loadMe: async () => {
        const t = getItemWithMigration(TOKEN_KEY) || get().token
        if (!t) return
        try {
          const r = await rest.me()
          if (r.code === 0 && r.data) {
            set({
              token: t,
              user: r.data.user,
              role: (r.data.role as 'admin' | 'user'),
              authEnabled: true,
            })
            localStorage.setItem(TOKEN_KEY, t)
            return
          }
        } catch {
          /* invalid/expired token; clear it */
        }
        localStorage.removeItem(TOKEN_KEY)
        set({ token: '', user: '', role: 'user' })
      },

      login: async (username, password) => {
        const r = await rest.login(username, password)
        if (r.code === 0 && r.data?.token) {
          set({
            token: r.data.token,
            user: r.data.user,
            role: (r.data.role as 'admin' | 'user'),
            authEnabled: true,
          })
          localStorage.setItem(TOKEN_KEY, r.data.token)
          return { ok: true }
        }
        const d = r.data || {}
        return {
          ok: false,
          message: r.message || '用户名或密码错误',
          warn: d.warn,
          locked: d.locked,
          banned: d.banned,
          retryAfter: d.retry_after,
          remaining: d.remaining,
        }
      },

      logout: () => {
        localStorage.removeItem(TOKEN_KEY)
        set({ token: '', user: '', role: 'user' })
      },

      setIdentity: (user, token) => {
        set((s) => ({ user, token: token ?? s.token }))
        if (token) localStorage.setItem(TOKEN_KEY, token)
      },
    }),
    {
      name: authPersistKey,
      storage: createJSONStorage(() => migratedStorage),
      // Persist only durable identity/version data; recompute `ready` on load.
      partialize: (s) => ({
        token: s.token,
        user: s.user,
        role: s.role,
        authEnabled: s.authEnabled,
        version: s.version,
      }),
    },
  ),
)
